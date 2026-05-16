package adapter_test

import (
	"context"
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	agentadapter "github.com/jiujuan/wukong/pkg/agent/adapter"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/skillruntime"
	"github.com/jiujuan/wukong/pkg/worker"
)

func TestWorkerPoolAgentRuntimeAdapterExample(t *testing.T) {
	ctx := context.Background()

	skillRegistry := skillruntime.NewRegistry()
	_ = skillRegistry

	runtime := agent.NewRuntime()
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

	handler := agentadapter.NewWorkerHandler(runtime, agentadapter.NewDefaultSubTaskMapper())
	pool := worker.New(worker.WithWorkerCount(1))
	pool.SetTaskHandler(handler.Handle)

	subTask := &exampleSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "respond",
		params:    map[string]any{"prompt": "hello"},
	}
	if err := handler.Handle(ctx, &queue.Task{TaskID: "queue-1", Data: subTask}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if subTask.err != "" {
		t.Fatalf("subtask error = %q, want empty", subTask.err)
	}
	if subTask.result["agent_id"] != "general" || subTask.result["status"] != "completed" {
		t.Fatalf("subtask result = %#v, want completed general agent result", subTask.result)
	}
}

type exampleSubTask struct {
	subTaskID string
	taskID    string
	action    string
	params    map[string]any
	result    map[string]any
	err       string
	updatedAt time.Time
}

func (s *exampleSubTask) GetSubTaskID() string {
	return s.subTaskID
}

func (s *exampleSubTask) GetTaskID() string {
	return s.taskID
}

func (s *exampleSubTask) GetAction() string {
	return s.action
}

func (s *exampleSubTask) GetParams() map[string]any {
	return s.params
}

func (s *exampleSubTask) SetResult(result map[string]any) {
	s.result = result
}

func (s *exampleSubTask) SetError(err string) {
	s.err = err
}

func (s *exampleSubTask) SetUpdatedAt(updatedAt time.Time) {
	s.updatedAt = updatedAt
}
