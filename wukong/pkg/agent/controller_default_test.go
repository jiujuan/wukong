package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultLoopControllerStopsAfterMaxIterations(t *testing.T) {
	controller := NewDefaultLoopController(WithMaxIterations(2))
	state := &LoopState{Iteration: 2, Status: LoopStatusRunning}

	decision, err := controller.BeforeIteration(context.Background(), state)
	if err != nil {
		t.Fatalf("BeforeIteration() error = %v", err)
	}
	if !decision.Stop {
		t.Fatalf("BeforeIteration() decision = %#v, want stop", decision)
	}
	if state.Status != LoopStatusStopped {
		t.Fatalf("state Status = %q, want stopped", state.Status)
	}
}

func TestDefaultLoopControllerStopsAfterMaxDuration(t *testing.T) {
	controller := NewDefaultLoopController(WithMaxDuration(time.Millisecond))
	state := &LoopState{
		Status: LoopStatusRunning,
		Metadata: map[string]any{
			loopStartedAtMetadataKey: time.Now().Add(-time.Second),
		},
	}

	decision, err := controller.BeforeIteration(context.Background(), state)
	if err != nil {
		t.Fatalf("BeforeIteration() error = %v", err)
	}
	if !decision.Stop || decision.Reason != "max duration exceeded" {
		t.Fatalf("BeforeIteration() decision = %#v, want max duration stop", decision)
	}
}

func TestDefaultLoopControllerRetriesRetryableError(t *testing.T) {
	controller := NewDefaultLoopController(WithMaxRetries(2))
	state := &LoopState{Status: LoopStatusRunning}

	decision, err := controller.OnError(context.Background(), state, errors.New("temporary failure"))
	if err != nil {
		t.Fatalf("OnError() error = %v", err)
	}
	if !decision.Retry {
		t.Fatalf("OnError() decision = %#v, want retry", decision)
	}
	if retryCount(state) != 1 {
		t.Fatalf("retry count = %d, want 1", retryCount(state))
	}
}

func TestDefaultLoopControllerStopsAfterRetryLimit(t *testing.T) {
	controller := NewDefaultLoopController(WithMaxRetries(1))
	state := &LoopState{Status: LoopStatusRunning}

	_, err := controller.OnError(context.Background(), state, errors.New("temporary failure"))
	if err != nil {
		t.Fatalf("first OnError() error = %v", err)
	}
	decision, err := controller.OnError(context.Background(), state, errors.New("still failing"))
	if err != nil {
		t.Fatalf("second OnError() error = %v", err)
	}
	if !decision.Stop {
		t.Fatalf("second OnError() decision = %#v, want stop", decision)
	}
	if state.Status != LoopStatusFailed {
		t.Fatalf("state Status = %q, want failed", state.Status)
	}
}

func TestDefaultLoopControllerStopsOnNonRetryableError(t *testing.T) {
	controller := NewDefaultLoopController(WithMaxRetries(3))
	state := &LoopState{Status: LoopStatusRunning}

	decision, err := controller.OnError(context.Background(), state, context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("OnError() error = %v", err)
	}
	if !decision.Stop || decision.Retry {
		t.Fatalf("OnError() decision = %#v, want stop without retry", decision)
	}
	if state.Status != LoopStatusFailed {
		t.Fatalf("state Status = %q, want failed", state.Status)
	}
}

func TestDefaultLoopControllerReturnsPauseDecision(t *testing.T) {
	controller := NewDefaultLoopController()
	state := &LoopState{Status: LoopStatusRunning}

	decision, err := controller.OnError(context.Background(), state, ErrLoopPaused)
	if err != nil {
		t.Fatalf("OnError() error = %v", err)
	}
	if !decision.Pause {
		t.Fatalf("OnError() decision = %#v, want pause", decision)
	}
	if state.Status != LoopStatusPaused || state.Phase != LoopPhasePaused {
		t.Fatalf("state = %#v, want paused phase/status", state)
	}
}

func TestDefaultLoopControllerHumanResponseResumesWithPatch(t *testing.T) {
	controller := NewDefaultLoopController()
	state := &LoopState{Status: LoopStatusPaused}
	input := HumanInput{
		RunID:     "run-1",
		RequestID: "human-1",
		Patch: map[string]any{
			"approved": true,
		},
	}

	decision, err := controller.OnHumanResponse(context.Background(), state, input)
	if err != nil {
		t.Fatalf("OnHumanResponse() error = %v", err)
	}
	if !decision.Resume {
		t.Fatalf("OnHumanResponse() decision = %#v, want resume", decision)
	}
	if decision.Patch["approved"] != true {
		t.Fatalf("Patch = %#v, want approved=true", decision.Patch)
	}
	if state.Status != LoopStatusRunning {
		t.Fatalf("state Status = %q, want running", state.Status)
	}
}

func TestDefaultLoopControllerBeforeRunInitializesState(t *testing.T) {
	controller := NewDefaultLoopController()
	state := &LoopState{}

	decision, err := controller.BeforeRun(context.Background(), state)
	if err != nil {
		t.Fatalf("BeforeRun() error = %v", err)
	}
	if !decision.Continue {
		t.Fatalf("BeforeRun() decision = %#v, want continue", decision)
	}
	if state.Status != LoopStatusRunning {
		t.Fatalf("state Status = %q, want running", state.Status)
	}
	if _, ok := state.Metadata[loopStartedAtMetadataKey].(time.Time); !ok {
		t.Fatalf("started_at metadata = %#v, want time.Time", state.Metadata[loopStartedAtMetadataKey])
	}
}
