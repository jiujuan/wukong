package agent

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryCheckpointStoreSaveThenLoad(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	checkpoint := LoopCheckpoint{
		RunID:      "run-1",
		AgentID:    "agent-1",
		Iteration:  2,
		Phase:      LoopPhaseAct,
		StepCursor: 1,
		Request: RunRequest{
			RunID:   "run-1",
			TaskID:  "task-1",
			AgentID: "agent-1",
			Context: map[string]any{"trace": "enabled"},
		},
		Plan: map[string]any{"next": "observe"},
		StepResults: []LoopStep{
			{Index: 0, Status: "completed", Result: map[string]any{"ok": true}},
		},
		Context: map[string]any{"mode": "resume"},
		Status:  LoopStatusPaused,
	}

	if err := store.Save(context.Background(), checkpoint); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := store.Load(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.RunID != checkpoint.RunID || loaded.Phase != checkpoint.Phase || loaded.Iteration != checkpoint.Iteration {
		t.Fatalf("Load() = %#v, want saved checkpoint", loaded)
	}
	if loaded.Plan["next"] != "observe" {
		t.Fatalf("Plan = %#v, want next=observe", loaded.Plan)
	}
	if loaded.StepResults[0].Result["ok"] != true {
		t.Fatalf("StepResults = %#v, want ok=true", loaded.StepResults)
	}
}

func TestInMemoryCheckpointStoreDeleteThenLoadNotFound(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	if err := store.Save(context.Background(), LoopCheckpoint{RunID: "run-1"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Delete(context.Background(), "run-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Load(context.Background(), "run-1")
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("Load() error = %v, want ErrCheckpointNotFound", err)
	}
}

func TestInMemoryCheckpointStoreCheckpointCloneDoesNotShareMapsOrSlices(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	checkpoint := LoopCheckpoint{
		RunID: "run-1",
		Request: RunRequest{
			RunID:   "run-1",
			Params:  map[string]any{"input": "original"},
			Context: map[string]any{"request": "original"},
		},
		Agent: AgentProfile{
			ID:       "agent-1",
			Tools:    []string{"tool-a"},
			Metadata: map[string]any{"agent": "original"},
		},
		AgentState: AgentState{
			Scratchpad: map[string]any{"state": "original"},
		},
		AgentContext: AgentContext{
			SharedMemory: map[string]any{"shared": "original"},
			ActivatedSkills: []ActivatedSkill{
				{Name: "skill-a", Metadata: map[string]any{"skill": "original"}},
			},
			ToolCatalog: []ToolDescriptor{
				{Name: "tool-a", Metadata: map[string]any{"tool": "original"}},
			},
			Trace: []AgentEvent{
				{Type: "phase", Metadata: map[string]any{"event": "original"}},
			},
		},
		Plan: map[string]any{"plan": "original"},
		StepResults: []LoopStep{
			{Index: 0, Input: map[string]any{"input": "original"}, Result: map[string]any{"result": "original"}},
		},
		Context:  map[string]any{"context": "original"},
		Metadata: map[string]any{"metadata": "original"},
	}

	if err := store.Save(context.Background(), checkpoint); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	checkpoint.Plan["plan"] = "mutated"
	checkpoint.StepResults[0].Result["result"] = "mutated"
	checkpoint.Agent.Tools[0] = "mutated"

	loaded, err := store.Load(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	loaded.Plan["plan"] = "loaded-mutated"
	loaded.StepResults[0].Input["input"] = "loaded-mutated"
	loaded.AgentContext.ActivatedSkills[0].Metadata["skill"] = "loaded-mutated"

	reloaded, err := store.Load(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}

	if reloaded.Plan["plan"] != "original" {
		t.Fatalf("Plan shared mutation, got %#v", reloaded.Plan)
	}
	if reloaded.StepResults[0].Result["result"] != "original" || reloaded.StepResults[0].Input["input"] != "original" {
		t.Fatalf("StepResults shared mutation, got %#v", reloaded.StepResults)
	}
	if reloaded.Agent.Tools[0] != "tool-a" {
		t.Fatalf("Agent Tools shared mutation, got %#v", reloaded.Agent.Tools)
	}
	if reloaded.AgentContext.ActivatedSkills[0].Metadata["skill"] != "original" {
		t.Fatalf("ActivatedSkills shared mutation, got %#v", reloaded.AgentContext.ActivatedSkills)
	}
}

func TestLoopCheckpointToLoopStateClonesMutableFields(t *testing.T) {
	checkpoint := LoopCheckpoint{
		RunID:       "run-1",
		Iteration:   3,
		Phase:       LoopPhaseReflect,
		Plan:        map[string]any{"step": "reflect"},
		StepResults: []LoopStep{{Index: 1, Metadata: map[string]any{"phase": "act"}}},
		Status:      LoopStatusRunning,
		Metadata:    map[string]any{"retry_count": 1},
	}

	state := checkpoint.ToLoopState()
	state.Plan["step"] = "mutated"
	state.StepResults[0].Metadata["phase"] = "mutated"
	state.Metadata["retry_count"] = 2

	if checkpoint.Plan["step"] != "reflect" {
		t.Fatalf("checkpoint Plan shared state mutation, got %#v", checkpoint.Plan)
	}
	if checkpoint.StepResults[0].Metadata["phase"] != "act" {
		t.Fatalf("checkpoint StepResults shared state mutation, got %#v", checkpoint.StepResults)
	}
	if checkpoint.Metadata["retry_count"] != 1 {
		t.Fatalf("checkpoint Metadata shared state mutation, got %#v", checkpoint.Metadata)
	}
}
