package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAgentLoopSingleActionRunSucceeds(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopCheckpointStore(store),
		WithAgentLoopActionRunner(NewSequentialActionRunner(&fakePlanExecutor{
			result: &StepResult{
				Status: "completed",
				Output: "done",
				Result: map[string]any{"ok": true},
			},
		})),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
		Params: map[string]any{"query": "wukong"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(LoopStatusCompleted) || result.Output != "done" {
		t.Fatalf("RunResult = %#v, want completed output", result)
	}
	if result.Result["ok"] != true {
		t.Fatalf("RunResult Result = %#v, want ok=true", result.Result)
	}
	if result.Strategy != "direct" {
		t.Fatalf("RunResult Strategy = %q, want direct", result.Strategy)
	}
}

func TestAgentLoopActionErrorGoesThroughController(t *testing.T) {
	wantErr := errors.New("action failed")
	controller := &recordingErrorController{DefaultLoopController: *NewDefaultLoopController(WithMaxRetries(0))}
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopController(controller),
		WithAgentLoopActionRunner(NewSequentialActionRunner(&fakePlanExecutor{err: wantErr})),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if controller.err == nil || controller.err.Error() != wantErr.Error() {
		t.Fatalf("controller err = %v, want action failed", controller.err)
	}
	if result.Status != string(LoopStatusFailed) || result.Error == "" {
		t.Fatalf("RunResult = %#v, want failed result", result)
	}
}

func TestAgentLoopMaxIterationStop(t *testing.T) {
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopController(&stopBeforeIterationController{}),
	)
	result, err := loop.Run(context.Background(), RunRequest{RunID: "run-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(LoopStatusStopped) {
		t.Fatalf("RunResult Status = %q, want stopped", result.Status)
	}
}

func TestAgentLoopSavesCheckpointAtLeastOnce(t *testing.T) {
	store := NewInMemoryCheckpointStore()
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopCheckpointStore(store),
		WithAgentLoopActionRunner(NewSequentialActionRunner(&fakePlanExecutor{
			result: &StepResult{Status: "completed", Output: "done"},
		})),
	)

	_, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	checkpoint, err := store.Load(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if checkpoint.RunID != "run-1" || checkpoint.Status != LoopStatusCompleted {
		t.Fatalf("checkpoint = %#v, want completed run-1", checkpoint)
	}
}

func TestAgentLoopCallsReflector(t *testing.T) {
	reflector := &recordingReflector{
		evaluation: &Evaluation{Success: true, Score: 0.75, Reason: "checked"},
	}
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopReflector(reflector),
		WithAgentLoopActionRunner(NewSequentialActionRunner(&fakePlanExecutor{
			result: &StepResult{Status: "completed", Output: "done"},
		})),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflector.called {
		t.Fatal("reflector was not called")
	}
	if result.Evaluation == nil || result.Evaluation.Score != 0.75 || result.Evaluation.Reason != "checked" {
		t.Fatalf("Evaluation = %#v, want reflector evaluation", result.Evaluation)
	}
}

func TestAgentLoopReflectRetryRevisesPlanThenSucceeds(t *testing.T) {
	strategy := &recordingReviseStrategy{}
	runner := &sequencePlanRunner{
		results: []*ActionResult{
			{Status: "completed"},
			{Status: "completed", Output: "done"},
		},
	}
	reflector := &outputRetryReflector{}
	loop := NewAgentLoop(AgentProfile{
		ID:         "agent-1",
		Reflection: ReflectConfig{MaxRetries: 1},
	},
		WithAgentLoopStrategy(strategy),
		WithAgentLoopActionRunner(runner),
		WithAgentLoopReflector(reflector),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(LoopStatusCompleted) || result.Output != "done" {
		t.Fatalf("RunResult = %#v, want completed done", result)
	}
	if strategy.reviseCalls != 1 {
		t.Fatalf("Revise calls = %d, want 1", strategy.reviseCalls)
	}
	if runner.calls != 2 || reflector.calls != 2 {
		t.Fatalf("runner calls = %d reflector calls = %d, want 2 each", runner.calls, reflector.calls)
	}
}

func TestAgentLoopReflectRetryLimitReturnsFailedResult(t *testing.T) {
	strategy := &recordingReviseStrategy{}
	runner := &sequencePlanRunner{
		results: []*ActionResult{
			{Status: "completed"},
			{Status: "completed"},
		},
	}
	loop := NewAgentLoop(AgentProfile{
		ID:         "agent-1",
		Reflection: ReflectConfig{MaxRetries: 1},
	},
		WithAgentLoopStrategy(strategy),
		WithAgentLoopActionRunner(runner),
		WithAgentLoopReflector(&outputRetryReflector{}),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(LoopStatusFailed) {
		t.Fatalf("RunResult Status = %q, want failed", result.Status)
	}
	if result.Error != "reflect retry limit exceeded" {
		t.Fatalf("RunResult Error = %q, want retry limit exceeded", result.Error)
	}
	if strategy.reviseCalls != 1 {
		t.Fatalf("Revise calls = %d, want 1", strategy.reviseCalls)
	}
}

func TestAgentLoopLoadsMemoryBeforePlanAndWritesRun(t *testing.T) {
	memory := &recordingLoopMemoryProvider{
		snapshot: &MemorySnapshot{
			Working: []MemoryItem{{ID: "working-1", Content: "working memory"}},
			Long:    []MemoryItem{{ID: "long-1", Content: "long memory"}},
			Shared:  map[string]any{"shared": "memory"},
		},
	}
	strategy := &memoryRecordingStrategy{}
	executor := &memoryRecordingExecutor{
		result: &StepResult{Status: "completed", Output: "done"},
	}
	loop := NewAgentLoop(AgentProfile{ID: "agent-1"},
		WithAgentLoopMemoryProvider(memory),
		WithAgentLoopStrategy(strategy),
		WithAgentLoopActionRunner(NewSequentialActionRunner(executor)),
	)

	result, err := loop.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(LoopStatusCompleted) {
		t.Fatalf("RunResult Status = %q, want completed", result.Status)
	}
	if !memory.loadCalled {
		t.Fatal("memory Load was not called")
	}
	if len(strategy.agentCtx.WorkingMemory) != 1 || strategy.agentCtx.WorkingMemory[0].Content != "working memory" {
		t.Fatalf("strategy WorkingMemory = %#v, want loaded memory", strategy.agentCtx.WorkingMemory)
	}
	if len(strategy.agentCtx.LongMemory) != 1 || strategy.agentCtx.LongMemory[0].Content != "long memory" {
		t.Fatalf("strategy LongMemory = %#v, want loaded memory", strategy.agentCtx.LongMemory)
	}
	if strategy.agentCtx.SharedMemory["shared"] != "memory" {
		t.Fatalf("strategy SharedMemory = %#v, want loaded shared memory", strategy.agentCtx.SharedMemory)
	}
	if len(executor.agentCtx.WorkingMemory) != 1 || executor.agentCtx.SharedMemory["shared"] != "memory" {
		t.Fatalf("executor AgentContext = %#v, want loaded memory", executor.agentCtx)
	}
	if !memory.writeCalled || memory.writtenResult == nil || memory.writtenResult.Output != "done" {
		t.Fatalf("memory WriteRun result = %#v called=%v, want done write", memory.writtenResult, memory.writeCalled)
	}
	if memory.writtenEval == nil || !memory.writtenEval.Success {
		t.Fatalf("memory WriteRun eval = %#v, want successful evaluation", memory.writtenEval)
	}
}

type fakePlanExecutor struct {
	result *StepResult
	err    error
	step   PlanStep
}

type recordingReflector struct {
	called     bool
	evaluation *Evaluation
}

func (r *recordingReflector) Reflect(context.Context, AgentContext, *AgentPlan, *ActionResult, error) (*Evaluation, error) {
	r.called = true
	return r.evaluation, nil
}

type outputRetryReflector struct {
	calls int
}

func (r *outputRetryReflector) Reflect(_ context.Context, _ AgentContext, _ *AgentPlan, result *ActionResult, _ error) (*Evaluation, error) {
	r.calls++
	if result == nil || result.Output == "" {
		return &Evaluation{Success: false, Score: 0, Reason: "empty action output", Retry: true}, nil
	}
	return &Evaluation{Success: true, Score: 1, Reason: "output accepted"}, nil
}

type recordingReviseStrategy struct {
	reviseCalls int
}

func (s *recordingReviseStrategy) Name() string {
	return "recording"
}

func (s *recordingReviseStrategy) Plan(context.Context, AgentContext) (*AgentPlan, error) {
	return &AgentPlan{
		PlanID:   "plan-1",
		Strategy: s.Name(),
		Goal:     "test",
		Steps: []PlanStep{
			{StepID: "step-1", Type: StepTypeTool, Action: "search", Target: "search"},
		},
		CreatedAt: time.Now(),
	}, nil
}

func (s *recordingReviseStrategy) Revise(ctx context.Context, agentCtx AgentContext, previous *AgentPlan, _ *Evaluation) (*AgentPlan, error) {
	s.reviseCalls++
	revised := previous.Clone()
	revised.PlanID = previous.PlanID + "-revised"
	revised.CreatedAt = time.Now()
	return &revised, nil
}

type sequencePlanRunner struct {
	results []*ActionResult
	calls   int
}

func (r *sequencePlanRunner) RunPlan(context.Context, AgentContext, *AgentPlan) (*ActionResult, error) {
	index := r.calls
	r.calls++
	if index >= len(r.results) {
		return &ActionResult{Status: "completed"}, nil
	}
	result := *r.results[index]
	if result.Output != "" && len(result.StepResults) == 0 {
		result.StepResults = []StepResult{{Status: "completed", Output: result.Output}}
	}
	return &result, nil
}

func (e *fakePlanExecutor) Name() string {
	return "fake"
}

func (e *fakePlanExecutor) CanExecute(context.Context, AgentContext, PlanStep) bool {
	return true
}

func (e *fakePlanExecutor) Execute(_ context.Context, _ AgentContext, step PlanStep) (*StepResult, error) {
	e.step = step
	if e.result != nil {
		result := *e.result
		result.Type = step.Type
		result.Action = step.Action
		result.StartedAt = time.Now()
		result.CompletedAt = time.Now()
		return &result, e.err
	}
	return nil, e.err
}

type recordingLoopMemoryProvider struct {
	snapshot *MemorySnapshot

	loadCalled  bool
	writeCalled bool
	events      []AgentEvent

	writtenCtx    AgentContext
	writtenResult *ActionResult
	writtenEval   *Evaluation
}

func (p *recordingLoopMemoryProvider) Load(context.Context, RunRequest, AgentProfile) (*MemorySnapshot, error) {
	p.loadCalled = true
	return p.snapshot, nil
}

func (p *recordingLoopMemoryProvider) AppendEvent(_ context.Context, event AgentEvent) error {
	p.events = append(p.events, event)
	return nil
}

func (p *recordingLoopMemoryProvider) WriteRun(_ context.Context, agentCtx AgentContext, result *ActionResult, eval *Evaluation) error {
	p.writeCalled = true
	p.writtenCtx = agentCtx
	p.writtenResult = result
	p.writtenEval = eval
	return nil
}

type memoryRecordingStrategy struct {
	agentCtx AgentContext
}

func (s *memoryRecordingStrategy) Name() string {
	return "memory-recording"
}

func (s *memoryRecordingStrategy) Plan(_ context.Context, agentCtx AgentContext) (*AgentPlan, error) {
	s.agentCtx = agentCtx
	return &AgentPlan{
		PlanID:   "plan-1",
		Strategy: s.Name(),
		Goal:     "test",
		Steps: []PlanStep{
			{StepID: "step-1", Type: StepTypeTool, Action: "search", Target: "search"},
		},
		CreatedAt: time.Now(),
	}, nil
}

func (s *memoryRecordingStrategy) Revise(context.Context, AgentContext, *AgentPlan, *Evaluation) (*AgentPlan, error) {
	return nil, errors.New("unexpected revise")
}

type memoryRecordingExecutor struct {
	agentCtx AgentContext
	result   *StepResult
}

func (e *memoryRecordingExecutor) Name() string {
	return "memory-recording"
}

func (e *memoryRecordingExecutor) CanExecute(context.Context, AgentContext, PlanStep) bool {
	return true
}

func (e *memoryRecordingExecutor) Execute(_ context.Context, agentCtx AgentContext, step PlanStep) (*StepResult, error) {
	e.agentCtx = agentCtx
	result := *e.result
	result.Type = step.Type
	result.Action = step.Action
	result.StartedAt = time.Now()
	result.CompletedAt = time.Now()
	return &result, nil
}

type recordingErrorController struct {
	DefaultLoopController
	err error
}

func (c *recordingErrorController) OnError(ctx context.Context, state *LoopState, err error) (*LoopDecision, error) {
	c.err = err
	return c.DefaultLoopController.OnError(ctx, state, err)
}

type stopBeforeIterationController struct {
	DefaultLoopController
}

func (c stopBeforeIterationController) BeforeRun(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	return NewDefaultLoopController().BeforeRun(ctx, state)
}

func (c stopBeforeIterationController) BeforeIteration(ctx context.Context, state *LoopState) (*LoopDecision, error) {
	state.Status = LoopStatusStopped
	return &LoopDecision{Stop: true, Reason: "max iterations exceeded"}, nil
}
