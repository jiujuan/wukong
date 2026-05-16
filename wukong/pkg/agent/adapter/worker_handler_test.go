package adapter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/queue"
)

func TestWorkerHandlerWritesResultOnRuntimeSuccess(t *testing.T) {
	subTask := &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "search",
		params:    map[string]any{"query": "wukong"},
	}
	runtime := &fakeRuntime{
		result: &agent.RunResult{
			RunID:     "queue-1",
			AgentID:   agent.AgentID("agent-1"),
			TaskID:    "task-1",
			SubTaskID: "sub-1",
			Status:    "completed",
			Strategy:  "direct",
			Output:    "done",
		},
	}
	handler := NewWorkerHandler(runtime, nil)

	err := handler.Handle(context.Background(), &queue.Task{TaskID: "queue-1", Data: subTask})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if runtime.req.SubTaskID != "sub-1" || runtime.req.Action != "search" {
		t.Fatalf("runtime request = %#v, want subtask request", runtime.req)
	}
	if subTask.err != "" {
		t.Fatalf("subtask error = %q, want empty", subTask.err)
	}
	if subTask.result["agent_id"] != "agent-1" || subTask.result["strategy"] != "direct" || subTask.result["output"] != "done" {
		t.Fatalf("subtask result = %#v, want agent strategy output", subTask.result)
	}
	if subTask.updatedAt.IsZero() {
		t.Fatal("subtask updatedAt was not set")
	}
}

func TestWorkerHandlerWritesErrorOnRuntimeError(t *testing.T) {
	subTask := &fakeSubTask{subTaskID: "sub-1", taskID: "task-1", action: "search"}
	wantErr := errors.New("runtime failed")
	handler := NewWorkerHandler(&fakeRuntime{err: wantErr}, nil)

	err := handler.Handle(context.Background(), &queue.Task{TaskID: "queue-1", Data: subTask})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Handle() error = %v, want %v", err, wantErr)
	}
	if subTask.err != wantErr.Error() {
		t.Fatalf("subtask error = %q, want runtime failed", subTask.err)
	}
	if subTask.updatedAt.IsZero() {
		t.Fatal("subtask updatedAt was not set")
	}
}

func TestWorkerHandlerInvalidTaskDataReturnsClearError(t *testing.T) {
	handler := NewWorkerHandler(&fakeRuntime{}, nil)

	err := handler.Handle(context.Background(), &queue.Task{TaskID: "queue-1", Data: "bad"})
	if err == nil {
		t.Fatal("Handle() error = nil, want invalid payload error")
	}
	if !strings.Contains(err.Error(), "invalid subtask payload") {
		t.Fatalf("Handle() error = %v, want invalid subtask payload", err)
	}
}

type fakeRuntime struct {
	req    agent.RunRequest
	result *agent.RunResult
	err    error
}

func (r *fakeRuntime) Run(_ context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	r.req = req
	return r.result, r.err
}
