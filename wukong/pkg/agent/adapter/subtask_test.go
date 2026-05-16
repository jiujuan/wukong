package adapter

import (
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/queue"
)

func TestDefaultSubTaskMapperFromQueueTask(t *testing.T) {
	mapper := NewDefaultSubTaskMapper()
	subTask := &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "search",
		params:    map[string]any{"query": "wukong"},
	}

	req, holder, err := mapper.FromQueueTask(&queue.Task{
		TaskID: "queue-1",
		Data:   subTask,
	})
	if err != nil {
		t.Fatalf("FromQueueTask() error = %v", err)
	}
	if holder != subTask {
		t.Fatal("holder is not original subtask")
	}
	if req.RunID != "queue-1" || req.TaskID != "task-1" || req.SubTaskID != "sub-1" || req.Action != "search" {
		t.Fatalf("RunRequest = %#v, want mapped ids and action", req)
	}
	if req.Params["query"] != "wukong" {
		t.Fatalf("RunRequest params = %#v, want query", req.Params)
	}
	if req.Context["queue_task_id"] != "queue-1" {
		t.Fatalf("RunRequest context = %#v, want queue_task_id", req.Context)
	}
}

func TestToSubTaskResultContainsAgentStrategyOutput(t *testing.T) {
	mapper := NewDefaultSubTaskMapper()
	result := mapper.ToSubTaskResult(&agent.RunResult{
		RunID:     "run-1",
		AgentID:   agent.AgentID("agent-1"),
		TaskID:    "task-1",
		SubTaskID: "sub-1",
		Status:    "completed",
		Strategy:  "direct",
		Output:    "done",
		Result:    map[string]any{"ok": true},
	})

	if result["agent_id"] != "agent-1" {
		t.Fatalf("agent_id = %#v, want agent-1", result["agent_id"])
	}
	if result["strategy"] != "direct" {
		t.Fatalf("strategy = %#v, want direct", result["strategy"])
	}
	if result["output"] != "done" {
		t.Fatalf("output = %#v, want done", result["output"])
	}
	nested, ok := result["result"].(map[string]any)
	if !ok || nested["ok"] != true {
		t.Fatalf("result = %#v, want nested ok=true", result["result"])
	}
}

type fakeSubTask struct {
	subTaskID string
	taskID    string
	action    string
	params    map[string]any
	result    map[string]any
	err       string
	updatedAt time.Time
}

func (s *fakeSubTask) GetSubTaskID() string {
	return s.subTaskID
}

func (s *fakeSubTask) GetTaskID() string {
	return s.taskID
}

func (s *fakeSubTask) GetAction() string {
	return s.action
}

func (s *fakeSubTask) GetParams() map[string]any {
	return s.params
}

func (s *fakeSubTask) SetResult(result map[string]any) {
	s.result = result
}

func (s *fakeSubTask) SetError(err string) {
	s.err = err
}

func (s *fakeSubTask) SetUpdatedAt(updatedAt time.Time) {
	s.updatedAt = updatedAt
}
