package agent

import (
	"context"
	"fmt"
	"time"
)

// LoopPhaseHandler executes one Agent Loop phase.
type LoopPhaseHandler func(ctx context.Context, state *LoopState) error

// LoopMiddleware wraps a phase handler with cross-cutting loop behavior.
type LoopMiddleware interface {
	Wrap(next LoopPhaseHandler) LoopPhaseHandler
}

// LoopMiddlewareFunc adapts a function into a LoopMiddleware.
type LoopMiddlewareFunc func(next LoopPhaseHandler) LoopPhaseHandler

// Wrap implements LoopMiddleware.
func (f LoopMiddlewareFunc) Wrap(next LoopPhaseHandler) LoopPhaseHandler {
	if f == nil {
		return next
	}
	return f(next)
}

// BuildLoopMiddlewareChain wraps handler with middleware in declaration order.
func BuildLoopMiddlewareChain(handler LoopPhaseHandler, middleware ...LoopMiddleware) LoopPhaseHandler {
	if handler == nil {
		handler = func(context.Context, *LoopState) error { return nil }
	}
	wrapped := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		if middleware[i] == nil {
			continue
		}
		wrapped = middleware[i].Wrap(wrapped)
	}
	return wrapped
}

// RecoveryMiddleware converts phase panics into errors.
type RecoveryMiddleware struct{}

// NewRecoveryMiddleware creates a panic recovery middleware.
func NewRecoveryMiddleware() RecoveryMiddleware {
	return RecoveryMiddleware{}
}

// Wrap implements LoopMiddleware.
func (m RecoveryMiddleware) Wrap(next LoopPhaseHandler) LoopPhaseHandler {
	return func(ctx context.Context, state *LoopState) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("agent loop phase panic: %v", recovered)
				if state != nil {
					state.LastError = err.Error()
					state.Status = LoopStatusFailed
				}
			}
		}()
		if next == nil {
			return nil
		}
		return next(ctx, state)
	}
}

// CheckpointMiddleware saves a checkpoint after a phase completes successfully.
type CheckpointMiddleware struct {
	store CheckpointStore
}

// NewCheckpointMiddleware creates checkpoint persistence middleware.
func NewCheckpointMiddleware(store CheckpointStore) CheckpointMiddleware {
	return CheckpointMiddleware{store: store}
}

// Wrap implements LoopMiddleware.
func (m CheckpointMiddleware) Wrap(next LoopPhaseHandler) LoopPhaseHandler {
	return func(ctx context.Context, state *LoopState) error {
		if next != nil {
			if err := next(ctx, state); err != nil {
				return err
			}
		}
		if m.store == nil || state == nil {
			return nil
		}
		checkpoint := NewLoopCheckpoint(state)
		return m.store.Save(ctx, checkpoint)
	}
}

// TraceMiddleware records minimal phase events on LoopState.AgentContext.Trace.
type TraceMiddleware struct {
	now func() time.Time
}

// NewTraceMiddleware creates minimal trace middleware.
func NewTraceMiddleware() TraceMiddleware {
	return TraceMiddleware{now: time.Now}
}

// Wrap implements LoopMiddleware.
func (m TraceMiddleware) Wrap(next LoopPhaseHandler) LoopPhaseHandler {
	return func(ctx context.Context, state *LoopState) error {
		phase := LoopPhase("")
		if state != nil {
			phase = state.Phase
			appendLoopTraceEvent(state, "loop.phase.start", phase, "")
		}
		err := error(nil)
		if next != nil {
			err = next(ctx, state)
		}
		if state != nil {
			if err != nil {
				appendLoopTraceEvent(state, "loop.phase.error", phase, err.Error())
			} else {
				appendLoopTraceEvent(state, "loop.phase.end", phase, "")
			}
		}
		return err
	}
}

func appendLoopTraceEvent(state *LoopState, eventType string, phase LoopPhase, message string) {
	event := AgentEvent{
		RunID:     state.RunID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"phase": string(phase),
		},
	}
	state.AgentContext.Trace = append(state.AgentContext.Trace, event)
}
