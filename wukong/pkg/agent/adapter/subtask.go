package adapter

import (
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/queue"
)

// ExecutableSubTask is the minimal protocol consumed by Agent Runtime adapters.
type ExecutableSubTask interface {
	GetSubTaskID() string
	GetTaskID() string
	GetAction() string
	GetParams() map[string]any
	SetResult(map[string]any)
	SetError(string)
	SetUpdatedAt(time.Time)
}

// SubTaskMapper converts between WorkerPool subtasks and Agent Runtime values.
type SubTaskMapper interface {
	FromQueueTask(task *queue.Task) (agent.RunRequest, ExecutableSubTask, error)
	ToSubTaskResult(result *agent.RunResult) map[string]any
}

// DefaultSubTaskMapper maps existing queue subtasks to Agent Runtime requests.
type DefaultSubTaskMapper struct{}

// NewDefaultSubTaskMapper creates a default subtask mapper.
func NewDefaultSubTaskMapper() DefaultSubTaskMapper {
	return DefaultSubTaskMapper{}
}

// FromQueueTask extracts an ExecutableSubTask from queue.Task.Data and builds a RunRequest.
func (DefaultSubTaskMapper) FromQueueTask(task *queue.Task) (agent.RunRequest, ExecutableSubTask, error) {
	if task == nil {
		return agent.RunRequest{}, nil, fmt.Errorf("queue task is nil")
	}
	subTask, ok := task.Data.(ExecutableSubTask)
	if !ok || subTask == nil {
		return agent.RunRequest{}, nil, fmt.Errorf("invalid subtask payload for task_id=%s", task.TaskID)
	}
	params := cloneMap(subTask.GetParams())
	req := agent.RunRequest{
		RunID:     task.TaskID,
		TaskID:    subTask.GetTaskID(),
		SubTaskID: subTask.GetSubTaskID(),
		Action:    subTask.GetAction(),
		Params:    params,
		Context: map[string]any{
			"queue_task_id": task.TaskID,
		},
	}
	if req.RunID == "" {
		req.RunID = subTask.GetSubTaskID()
	}
	if req.SubTaskID == "" {
		req.SubTaskID = task.TaskID
	}
	return req, subTask, nil
}

// ToSubTaskResult converts an Agent run result into the map stored on a subtask.
func (DefaultSubTaskMapper) ToSubTaskResult(result *agent.RunResult) map[string]any {
	return ToSubTaskResult(result)
}

// ToSubTaskResult converts an Agent run result into the map stored on a subtask.
func ToSubTaskResult(result *agent.RunResult) map[string]any {
	out := map[string]any{}
	if result == nil {
		return out
	}
	out["run_id"] = result.RunID
	out["agent_id"] = string(result.AgentID)
	out["task_id"] = result.TaskID
	out["sub_task_id"] = result.SubTaskID
	out["status"] = result.Status
	out["output"] = result.Output
	out["result"] = cloneMap(result.Result)
	out["error"] = result.Error
	out["strategy"] = result.Strategy
	if result.Evaluation != nil {
		out["evaluation"] = result.Evaluation
	}
	if out["strategy"] == "" {
		strategy, ok := strategyFromResult(result)
		if !ok {
			return out
		}
		out["strategy"] = strategy
	}
	return out
}

func strategyFromResult(result *agent.RunResult) (string, bool) {
	if result == nil || result.Evaluation == nil || result.Evaluation.Metadata == nil {
		return "", false
	}
	strategy, ok := result.Evaluation.Metadata["strategy"].(string)
	return strategy, ok && strategy != ""
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
