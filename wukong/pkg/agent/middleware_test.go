package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildLoopMiddlewareChainExecutesInOrder(t *testing.T) {
	var calls []string
	handler := func(context.Context, *LoopState) error {
		calls = append(calls, "handler")
		return nil
	}
	middleware := func(name string) LoopMiddleware {
		return LoopMiddlewareFunc(func(next LoopPhaseHandler) LoopPhaseHandler {
			return func(ctx context.Context, state *LoopState) error {
				calls = append(calls, name+":before")
				err := next(ctx, state)
				calls = append(calls, name+":after")
				return err
			}
		})
	}

	chain := BuildLoopMiddlewareChain(handler, middleware("a"), middleware("b"))
	if err := chain(context.Background(), &LoopState{}); err != nil {
		t.Fatalf("chain() error = %v", err)
	}

	want := []string{"a:before", "b:before", "handler", "b:after", "a:after"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestRecoveryMiddlewareConvertsPanicToError(t *testing.T) {
	state := &LoopState{Status: LoopStatusRunning}
	handler := NewRecoveryMiddleware().Wrap(func(context.Context, *LoopState) error {
		panic("boom")
	})

	err := handler(context.Background(), state)
	if err == nil {
		t.Fatal("handler() error = nil, want panic error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("handler() error = %v, want boom", err)
	}
	if state.Status != LoopStatusFailed || state.LastError == "" {
		t.Fatalf("state = %#v, want failed status with last error", state)
	}
}

func TestCheckpointMiddlewareSavesAfterSuccessfulPhase(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	state := &LoopState{
		RunID:      "run-1",
		Phase:      LoopPhaseAct,
		Iteration:  1,
		Status:     LoopStatusRunning,
		StepCursor: 2,
		Plan:       map[string]any{"next": "observe"},
	}
	handler := NewCheckpointMiddleware(store).Wrap(func(context.Context, *LoopState) error {
		state.StepResults = append(state.StepResults, LoopStep{Index: 1, Status: "completed"})
		return nil
	})

	if err := handler(context.Background(), state); err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	checkpoint, err := store.Load(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if checkpoint.Phase != LoopPhaseAct || checkpoint.StepCursor != 2 {
		t.Fatalf("checkpoint = %#v, want act phase and cursor 2", checkpoint)
	}
	if len(checkpoint.StepResults) != 1 {
		t.Fatalf("StepResults length = %d, want 1", len(checkpoint.StepResults))
	}
}

func TestCheckpointMiddlewareSkipsSaveWhenPhaseFails(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	wantErr := errors.New("phase failed")
	handler := NewCheckpointMiddleware(store).Wrap(func(context.Context, *LoopState) error {
		return wantErr
	})

	err := handler(context.Background(), &LoopState{RunID: "run-1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("handler() error = %v, want %v", err, wantErr)
	}
	_, err = store.Load(context.Background(), "run-1")
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("Load() error = %v, want ErrCheckpointNotFound", err)
	}
}

func TestTraceMiddlewareRecordsPhaseEvents(t *testing.T) {
	state := &LoopState{
		RunID: "run-1",
		Phase: LoopPhasePlan,
	}
	handler := NewTraceMiddleware().Wrap(func(context.Context, *LoopState) error {
		return nil
	})

	if err := handler(context.Background(), state); err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if len(state.AgentContext.Trace) != 2 {
		t.Fatalf("Trace length = %d, want 2", len(state.AgentContext.Trace))
	}
	if state.AgentContext.Trace[0].Type != "loop.phase.start" || state.AgentContext.Trace[1].Type != "loop.phase.end" {
		t.Fatalf("Trace = %#v, want start/end events", state.AgentContext.Trace)
	}
	if state.AgentContext.Trace[0].Metadata["phase"] != string(LoopPhasePlan) {
		t.Fatalf("Trace metadata = %#v, want plan phase", state.AgentContext.Trace[0].Metadata)
	}
}

func TestTraceMiddlewareRecordsErrorEvent(t *testing.T) {
	state := &LoopState{
		RunID: "run-1",
		Phase: LoopPhaseAct,
	}
	wantErr := errors.New("action failed")
	handler := NewTraceMiddleware().Wrap(func(context.Context, *LoopState) error {
		return wantErr
	})

	err := handler(context.Background(), state)
	if !errors.Is(err, wantErr) {
		t.Fatalf("handler() error = %v, want %v", err, wantErr)
	}
	if len(state.AgentContext.Trace) != 2 {
		t.Fatalf("Trace length = %d, want 2", len(state.AgentContext.Trace))
	}
	if state.AgentContext.Trace[1].Type != "loop.phase.error" || state.AgentContext.Trace[1].Message != "action failed" {
		t.Fatalf("Trace = %#v, want error event", state.AgentContext.Trace)
	}
}
