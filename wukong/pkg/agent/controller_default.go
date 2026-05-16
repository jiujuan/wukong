package agent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const loopStartedAtMetadataKey = "started_at"

// DefaultLoopController applies basic loop budgets and error handling.
type DefaultLoopController struct {
	cfg DefaultLoopControllerConfig
}

// DefaultLoopControllerConfig configures the default controller.
type DefaultLoopControllerConfig struct {
	MaxIterations int
	MaxDuration   time.Duration
	MaxRetries    int
}

// DefaultLoopControllerOption configures a DefaultLoopController.
type DefaultLoopControllerOption func(*DefaultLoopControllerConfig)

// WithMaxIterations configures the maximum loop iterations.
func WithMaxIterations(max int) DefaultLoopControllerOption {
	return func(cfg *DefaultLoopControllerConfig) {
		cfg.MaxIterations = max
	}
}

// WithMaxDuration configures the maximum loop duration.
func WithMaxDuration(max time.Duration) DefaultLoopControllerOption {
	return func(cfg *DefaultLoopControllerConfig) {
		cfg.MaxDuration = max
	}
}

// WithMaxRetries configures the maximum retries for retryable errors.
func WithMaxRetries(max int) DefaultLoopControllerOption {
	return func(cfg *DefaultLoopControllerConfig) {
		cfg.MaxRetries = max
	}
}

// NewDefaultLoopController creates a basic loop controller.
func NewDefaultLoopController(options ...DefaultLoopControllerOption) *DefaultLoopController {
	cfg := DefaultLoopControllerConfig{
		MaxRetries: 1,
	}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	return &DefaultLoopController{cfg: cfg}
}

// BeforeRun initializes runtime bookkeeping and allows the loop to start.
func (c *DefaultLoopController) BeforeRun(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state != nil {
		state.Status = LoopStatusRunning
		ensureLoopMetadata(state)[loopStartedAtMetadataKey] = time.Now()
	}
	return continueDecision("loop started"), nil
}

// BeforeIteration checks iteration and duration budgets before work starts.
func (c *DefaultLoopController) BeforeIteration(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state == nil {
		return stopDecision("loop state is nil"), nil
	}
	if c.cfg.MaxIterations > 0 && state.Iteration >= c.cfg.MaxIterations {
		state.Status = LoopStatusStopped
		return stopDecision("max iterations exceeded"), nil
	}
	if c.durationExceeded(state) {
		state.Status = LoopStatusStopped
		return stopDecision("max duration exceeded"), nil
	}
	state.Status = LoopStatusRunning
	return continueDecision("iteration allowed"), nil
}

// AfterIteration advances the iteration counter and keeps the loop moving.
func (c *DefaultLoopController) AfterIteration(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state != nil {
		state.Iteration++
	}
	return continueDecision("iteration completed"), nil
}

// OnError decides whether an error should retry, pause, or stop the loop.
func (c *DefaultLoopController) OnError(ctx context.Context, state *LoopState, err error) (*LoopDecision, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err == nil {
		return continueDecision("no error"), nil
	}
	if state != nil {
		state.LastError = err.Error()
	}
	if errors.Is(err, ErrLoopPaused) {
		if state != nil {
			state.Status = LoopStatusPaused
			state.Phase = LoopPhasePaused
		}
		return pauseDecision("loop paused"), nil
	}
	if !isRetryableLoopError(err) {
		if state != nil {
			state.Status = LoopStatusFailed
		}
		return stopDecision(fmt.Sprintf("non-retryable error: %v", err)), nil
	}

	retries := retryCount(state)
	if retries < c.cfg.MaxRetries {
		setRetryCount(state, retries+1)
		return &LoopDecision{
			Retry:  true,
			Reason: "retryable error",
			Metadata: map[string]any{
				"retry_count": retries + 1,
			},
		}, nil
	}
	if state != nil {
		state.Status = LoopStatusFailed
	}
	return stopDecision("retry limit exceeded"), nil
}

// OnHumanResponse resumes a paused loop and carries any human patch forward.
func (c *DefaultLoopController) OnHumanResponse(ctx context.Context, state *LoopState, input HumanInput) (*LoopDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state != nil {
		state.Status = LoopStatusRunning
	}
	return &LoopDecision{
		Resume: true,
		Reason: "human response received",
		Patch:  cloneMap(input.Patch),
	}, nil
}

// BeforeStop marks a non-terminal loop as stopped.
func (c *DefaultLoopController) BeforeStop(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if state != nil && !state.Done() {
		state.Status = LoopStatusStopped
	}
	return stopDecision("loop stopping"), nil
}

func (c *DefaultLoopController) durationExceeded(state *LoopState) bool {
	if c.cfg.MaxDuration <= 0 || state == nil || state.Metadata == nil {
		return false
	}
	startedAt, ok := state.Metadata[loopStartedAtMetadataKey].(time.Time)
	if !ok || startedAt.IsZero() {
		return false
	}
	return time.Since(startedAt) >= c.cfg.MaxDuration
}

func isRetryableLoopError(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func retryCount(state *LoopState) int {
	if state == nil || state.Metadata == nil {
		return 0
	}
	value, ok := state.Metadata["retry_count"].(int)
	if !ok {
		return 0
	}
	return value
}

func setRetryCount(state *LoopState, count int) {
	if state == nil {
		return
	}
	ensureLoopMetadata(state)["retry_count"] = count
}

func ensureLoopMetadata(state *LoopState) map[string]any {
	if state.Metadata == nil {
		state.Metadata = make(map[string]any)
	}
	return state.Metadata
}

func continueDecision(reason string) *LoopDecision {
	return &LoopDecision{Continue: true, Reason: reason}
}

func stopDecision(reason string) *LoopDecision {
	return &LoopDecision{Stop: true, Reason: reason}
}

func pauseDecision(reason string) *LoopDecision {
	return &LoopDecision{
		Pause:  true,
		Reason: reason,
	}
}
