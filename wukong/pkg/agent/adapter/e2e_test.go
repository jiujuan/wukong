package adapter_test

import (
	"context"
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	agentadapter "github.com/jiujuan/wukong/pkg/agent/adapter"
	"github.com/jiujuan/wukong/pkg/queue"
)

func TestSubTaskToAgentRuntimeToResult(t *testing.T) {
	ctx := context.Background()
	executor := &e2eActionExecutor{
		output: "agent handled subtask",
		result: map[string]any{"ok": true},
	}
	runtime := agent.NewRuntime(agent.WithLoopFactory(e2eLoopFactory{executor: executor}))
	if err := runtime.RegisterAgent(agent.AgentProfile{ID: agent.AgentID("general")}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Stop(ctx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	subTask := &e2eSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "search",
		params:    map[string]any{"query": "wukong"},
	}
	handler := agentadapter.NewWorkerHandler(runtime, agentadapter.NewDefaultSubTaskMapper())
	err := handler.Handle(ctx, &queue.Task{TaskID: "queue-1", Data: subTask})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if executor.step.Action != "search" || executor.step.Params["query"] != "wukong" {
		t.Fatalf("executor step = %#v, want mapped subtask action and params", executor.step)
	}
	if subTask.err != "" {
		t.Fatalf("subtask error = %q, want empty", subTask.err)
	}
	if subTask.result["agent_id"] != "general" || subTask.result["strategy"] != "direct" {
		t.Fatalf("subtask result = %#v, want general direct agent result", subTask.result)
	}
	if subTask.result["output"] != "agent handled subtask" {
		t.Fatalf("subtask output = %#v, want executor output", subTask.result["output"])
	}
	nested, ok := subTask.result["result"].(map[string]any)
	if !ok || nested["ok"] != true {
		t.Fatalf("subtask nested result = %#v, want ok=true", subTask.result["result"])
	}
	if subTask.updatedAt.IsZero() {
		t.Fatal("subtask updatedAt was not set")
	}
}

type e2eLoopFactory struct {
	executor agent.PlanActionExecutor
}

func (f e2eLoopFactory) NewLoop(profile agent.AgentProfile) agent.Loop {
	return agent.NewAgentLoop(profile,
		agent.WithAgentLoopActionRunner(agent.NewSequentialActionRunner(f.executor)),
	)
}

type e2eActionExecutor struct {
	step   agent.PlanStep
	output string
	result map[string]any
}

func (e *e2eActionExecutor) Name() string {
	return "fake"
}

func (e *e2eActionExecutor) CanExecute(context.Context, agent.AgentContext, agent.PlanStep) bool {
	return true
}

func (e *e2eActionExecutor) Execute(_ context.Context, _ agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	e.step = step
	return &agent.StepResult{
		StepID:      step.StepID,
		Type:        step.Type,
		Action:      step.Action,
		Target:      step.Target,
		Status:      "completed",
		Output:      e.output,
		Result:      cloneE2EMap(e.result),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

type e2eSubTask struct {
	subTaskID string
	taskID    string
	action    string
	params    map[string]any
	result    map[string]any
	err       string
	updatedAt time.Time
}

func (s *e2eSubTask) GetSubTaskID() string {
	return s.subTaskID
}

func (s *e2eSubTask) GetTaskID() string {
	return s.taskID
}

func (s *e2eSubTask) GetAction() string {
	return s.action
}

func (s *e2eSubTask) GetParams() map[string]any {
	return s.params
}

func (s *e2eSubTask) SetResult(result map[string]any) {
	s.result = result
}

func (s *e2eSubTask) SetError(err string) {
	s.err = err
}

func (s *e2eSubTask) SetUpdatedAt(updatedAt time.Time) {
	s.updatedAt = updatedAt
}

func cloneE2EMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
