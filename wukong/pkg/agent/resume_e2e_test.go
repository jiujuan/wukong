package agent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRunCheckpointTracksStepCursorAfterPartialExecution(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryCheckpointStore()
	plan := twoStepResumePlan()
	loop := NewAgentLoop(AgentProfile{ID: AgentID("agent-1")},
		WithAgentLoopCheckpointStore(store),
		WithAgentLoopStrategy(staticResumeStrategy{plan: plan}),
		WithAgentLoopActionRunner(partialResumeRunner{}),
	)

	_, err := loop.Run(ctx, RunRequest{RunID: "run-1", TaskID: "task-1", Action: "first"})
	if err == nil {
		t.Fatal("Run() error = nil, want partial execution error")
	}

	checkpoint, loadErr := store.Load(ctx, "run-1")
	if loadErr != nil {
		t.Fatalf("Load() error = %v", loadErr)
	}
	if checkpoint.StepCursor != 1 {
		t.Fatalf("checkpoint StepCursor = %d, want 1 after first completed step", checkpoint.StepCursor)
	}
	if checkpoint.AgentPlan == nil || len(checkpoint.AgentPlan.Steps) != 2 {
		t.Fatalf("checkpoint AgentPlan = %#v, want full two-step plan", checkpoint.AgentPlan)
	}
}

func TestResumeContinuesFromStepCursorWithoutRepeatingCompletedStep(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryCheckpointStore()
	executor := &recordingResumeExecutor{}
	runtime := NewRuntime(
		WithCheckpointStore(store),
		WithLoopFactory(resumeLoopFactory{executor: executor, store: store}),
	)
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	plan := twoStepResumePlan()
	profile := AgentProfile{ID: AgentID("agent-1")}
	req := RunRequest{RunID: "run-1", TaskID: "task-1", Action: "first"}
	state := &LoopState{
		RunID:        "run-1",
		Phase:        LoopPhasePaused,
		Request:      req,
		Agent:        profile,
		AgentContext: AgentContext{Request: req, Agent: profile},
		Plan:         planToMap(plan),
		AgentPlan:    plan,
		StepCursor:   1,
		StepResults: []LoopStep{
			{Index: 0, Type: string(StepTypeTool), Action: "first", Status: "completed", Output: "first done"},
		},
		Status: LoopStatusPaused,
	}
	if err := store.Save(ctx, NewLoopCheckpoint(state)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	result, err := runtime.Resume(ctx, "run-1", HumanInput{
		RunID: "run-1",
		Patch: map[string]any{
			"query":    "patched",
			"approved": true,
		},
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	if len(executor.steps) != 1 {
		t.Fatalf("executed steps = %#v, want only one remaining step", executor.steps)
	}
	if executor.steps[0].StepID != "step-2" || executor.steps[0].Action != "second" {
		t.Fatalf("executed step = %#v, want step-2 only", executor.steps[0])
	}
	if executor.steps[0].Params["query"] != "patched" || executor.steps[0].Params["approved"] != true {
		t.Fatalf("executed params = %#v, want human patch applied", executor.steps[0].Params)
	}
	if result.Status != string(LoopStatusCompleted) {
		t.Fatalf("RunResult Status = %q, want completed", result.Status)
	}
	if len(result.Steps) != 2 || result.Steps[0].Action != "first" || result.Steps[1].Action != "second" {
		t.Fatalf("RunResult Steps = %#v, want first preserved and second executed", result.Steps)
	}
	if result.Output != "second done" {
		t.Fatalf("RunResult Output = %q, want second done", result.Output)
	}
	loaded, err := store.Load(ctx, "run-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.StepCursor != 2 {
		t.Fatalf("checkpoint StepCursor = %d, want 2", loaded.StepCursor)
	}
}

type resumeLoopFactory struct {
	executor PlanActionExecutor
	store    CheckpointStore
}

func (f resumeLoopFactory) NewLoop(profile AgentProfile) Loop {
	return NewAgentLoop(profile,
		WithAgentLoopCheckpointStore(f.store),
		WithAgentLoopActionRunner(NewSequentialActionRunner(f.executor)),
	)
}

type staticResumeStrategy struct {
	plan *AgentPlan
}

func (s staticResumeStrategy) Name() string {
	return "static"
}

func (s staticResumeStrategy) Plan(context.Context, AgentContext) (*AgentPlan, error) {
	plan := s.plan.Clone()
	return &plan, nil
}

func (s staticResumeStrategy) Revise(context.Context, AgentContext, *AgentPlan, *Evaluation) (*AgentPlan, error) {
	plan := s.plan.Clone()
	return &plan, nil
}

type partialResumeRunner struct{}

func (partialResumeRunner) RunPlan(context.Context, AgentContext, *AgentPlan) (*ActionResult, error) {
	return &ActionResult{
		Status: "failed",
		StepResults: []StepResult{
			{StepID: "step-1", Type: StepTypeTool, Action: "first", Status: "completed", Output: "first done"},
		},
		Error: "interrupted",
	}, fmt.Errorf("interrupted")
}

type recordingResumeExecutor struct {
	steps []PlanStep
}

func twoStepResumePlan() *AgentPlan {
	return &AgentPlan{
		PlanID:   "plan-1",
		Strategy: "direct",
		Goal:     "resume task",
		Steps: []PlanStep{
			{StepID: "step-1", Type: StepTypeTool, Action: "first", Params: map[string]any{"query": "original"}},
			{StepID: "step-2", Type: StepTypeTool, Action: "second", Params: map[string]any{"query": "original"}},
		},
		CreatedAt: time.Now(),
	}
}

func (e *recordingResumeExecutor) Name() string {
	return "resume-recorder"
}

func (e *recordingResumeExecutor) CanExecute(context.Context, AgentContext, PlanStep) bool {
	return true
}

func (e *recordingResumeExecutor) Execute(_ context.Context, _ AgentContext, step PlanStep) (*StepResult, error) {
	e.steps = append(e.steps, step)
	return &StepResult{
		StepID:      step.StepID,
		Type:        step.Type,
		Action:      step.Action,
		Status:      "completed",
		Output:      step.Action + " done",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}
