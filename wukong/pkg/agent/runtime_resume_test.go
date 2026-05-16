package agent

import (
	"context"
	"errors"
	"testing"
)

func TestResumeMissingCheckpointReturnsNotFound(t *testing.T) {
	runtime := NewRuntime(WithCheckpointStore(NewInMemoryCheckpointStore()))
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err := runtime.Resume(context.Background(), "missing-run", HumanInput{RunID: "missing-run"})
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("Resume() error = %v, want ErrCheckpointNotFound", err)
	}
}

func TestResumeAppliesHumanInputPatchToLoopStateMetadata(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	state := &LoopState{
		RunID:      "run-1",
		Phase:      LoopPhasePaused,
		Iteration:  2,
		StepCursor: 3,
		Request: RunRequest{
			RunID:  "run-1",
			TaskID: "task-1",
		},
		Agent: AgentProfile{
			ID:   "agent-1",
			Name: "Agent One",
		},
		Status:   LoopStatusPaused,
		Metadata: map[string]any{"existing": "kept"},
	}
	if err := store.Save(context.Background(), NewLoopCheckpoint(state)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	controller := &recordingResumeController{}
	runtime := NewRuntime(
		WithCheckpointStore(store),
		WithLoopController(controller),
	)
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := runtime.Resume(context.Background(), "run-1", HumanInput{
		RunID: "run-1",
		Patch: map[string]any{
			"approved": true,
		},
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	if result.Status != string(LoopStatusRunning) {
		t.Fatalf("RunResult Status = %q, want running", result.Status)
	}
	if controller.state == nil {
		t.Fatal("controller did not receive restored state")
	}
	if controller.state.Metadata["approved"] != true {
		t.Fatalf("restored metadata = %#v, want approved=true", controller.state.Metadata)
	}
	if controller.state.Metadata["existing"] != "kept" {
		t.Fatalf("restored metadata = %#v, want existing metadata kept", controller.state.Metadata)
	}
}

type recordingResumeController struct {
	DefaultLoopController
	state *LoopState
}

func (c *recordingResumeController) OnHumanResponse(ctx context.Context, state *LoopState, input HumanInput) (*LoopDecision, error) {
	decision, err := NewDefaultLoopController().OnHumanResponse(ctx, state, input)
	c.state = state
	return decision, err
}
