package action

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestVirtualSubTaskReadsAndWritesActionParamsResult(t *testing.T) {
	subTask := NewVirtualSubTask(agent.AgentContext{
		Request: agent.RunRequest{TaskID: "task-1"},
	}, agent.PlanStep{
		StepID: "step-1",
		Action: "web_search",
		Params: map[string]any{"query": "wukong"},
	})

	if subTask.GetSubTaskID() != "step-1" || subTask.GetTaskID() != "task-1" {
		t.Fatalf("virtual subtask ids = %q/%q, want step-1/task-1", subTask.GetSubTaskID(), subTask.GetTaskID())
	}
	if subTask.GetAction() != "web_search" || subTask.GetParams()["query"] != "wukong" {
		t.Fatalf("virtual subtask action/params = %q %#v", subTask.GetAction(), subTask.GetParams())
	}

	subTask.SetResult(map[string]any{"output": "done"})
	subTask.SetError("failed once")
	if subTask.Result()["output"] != "done" || subTask.Error() != "failed once" {
		t.Fatalf("virtual subtask result/error = %#v/%q", subTask.Result(), subTask.Error())
	}
}

func TestExistingSubTaskExecutorBridgeConvertsLegacyResultToStepResult(t *testing.T) {
	legacy := &fakeLegacyExecutor{
		result: map[string]any{
			"output": "legacy done",
			"score":  0.9,
		},
	}
	bridge := NewExistingSubTaskExecutorBridge(legacy)
	step := agent.PlanStep{
		StepID: "step-1",
		Type:   agent.StepTypeTool,
		Action: "report_gen",
		Params: map[string]any{"topic": "wukong"},
	}

	result, err := bridge.Execute(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{TaskID: "task-1"},
	}, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if legacy.subTask == nil {
		t.Fatal("legacy executor was not called")
	}
	if legacy.subTask.GetAction() != "report_gen" || legacy.subTask.GetParams()["topic"] != "wukong" {
		t.Fatalf("legacy subtask = %q %#v", legacy.subTask.GetAction(), legacy.subTask.GetParams())
	}
	if result.Status != "completed" || result.Output != "legacy done" || result.Result["score"] != 0.9 {
		t.Fatalf("StepResult = %#v, want converted legacy output", result)
	}
	if result.Metadata["legacy_sub_task_id"] != "step-1" || result.Metadata["legacy_task_id"] != "task-1" {
		t.Fatalf("Metadata = %#v, want legacy ids", result.Metadata)
	}
}

type fakeLegacyExecutor struct {
	subTask ExecutableSubTask
	result  map[string]any
	err     error
}

func (e *fakeLegacyExecutor) Execute(ctx context.Context, subTask ExecutableSubTask) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.subTask = subTask
	return e.result, e.err
}
