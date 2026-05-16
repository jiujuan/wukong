package action

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

// ExecutableSubTask is the minimal subtask protocol consumed by legacy executors.
type ExecutableSubTask interface {
	GetSubTaskID() string
	GetTaskID() string
	GetAction() string
	GetParams() map[string]any
	SetResult(map[string]any)
	SetError(string)
	SetUpdatedAt(time.Time)
}

// LegacySubTaskExecutor is the minimal old executor shape used by the bridge.
type LegacySubTaskExecutor interface {
	Execute(ctx context.Context, subTask ExecutableSubTask) (map[string]any, error)
}

// ExistingSubTaskExecutorBridge adapts legacy subtask executors into Agent action executors.
type ExistingSubTaskExecutorBridge struct {
	executor LegacySubTaskExecutor
}

// NewExistingSubTaskExecutorBridge creates a bridge for an old executor.
func NewExistingSubTaskExecutorBridge(executor LegacySubTaskExecutor) *ExistingSubTaskExecutorBridge {
	return &ExistingSubTaskExecutorBridge{executor: executor}
}

// Name returns the executor name.
func (b *ExistingSubTaskExecutorBridge) Name() string {
	return "legacy_subtask"
}

// CanExecute reports whether this bridge can execute the step through legacy action semantics.
func (b *ExistingSubTaskExecutorBridge) CanExecute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) bool {
	return ctx.Err() == nil && b.executor != nil && step.Action != ""
}

// Execute converts a plan step into a virtual subtask and delegates to the legacy executor.
func (b *ExistingSubTaskExecutorBridge) Execute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	startedAt := time.Now()
	result := baseStepResult(step, "running", startedAt)
	if err := ctx.Err(); err != nil {
		return failStepResult(result, err), err
	}
	if b.executor == nil {
		err := fmt.Errorf("legacy subtask executor is not configured")
		return failStepResult(result, err), err
	}

	subTask := NewVirtualSubTask(agentCtx, step)
	output, err := b.executor.Execute(ctx, subTask)
	if err != nil {
		subTask.SetError(err.Error())
		return failStepResult(result, err), err
	}
	subTask.SetResult(output)
	subTask.SetUpdatedAt(time.Now())
	return stepResultFromSubTask(step, subTask, output, startedAt), nil
}

// VirtualSubTask adapts an Agent plan step to the legacy executable subtask protocol.
type VirtualSubTask struct {
	subTaskID string
	taskID    string
	action    string
	params    map[string]any
	result    map[string]any
	err       string
	updatedAt time.Time
}

// NewVirtualSubTask creates a virtual subtask from Agent context and one plan step.
func NewVirtualSubTask(agentCtx agent.AgentContext, step agent.PlanStep) *VirtualSubTask {
	subTaskID := step.StepID
	if subTaskID == "" {
		subTaskID = "virtual-step"
	}
	action := step.Action
	if action == "" {
		action = step.Target
	}
	return &VirtualSubTask{
		subTaskID: subTaskID,
		taskID:    agentCtx.Request.TaskID,
		action:    action,
		params:    cloneMap(step.Params),
	}
}

// GetSubTaskID returns the virtual subtask id.
func (s *VirtualSubTask) GetSubTaskID() string {
	return s.subTaskID
}

// GetTaskID returns the parent task id.
func (s *VirtualSubTask) GetTaskID() string {
	return s.taskID
}

// GetAction returns the legacy action name.
func (s *VirtualSubTask) GetAction() string {
	return s.action
}

// GetParams returns the virtual subtask params.
func (s *VirtualSubTask) GetParams() map[string]any {
	return cloneMap(s.params)
}

// SetResult records the legacy executor result.
func (s *VirtualSubTask) SetResult(result map[string]any) {
	s.result = cloneMap(result)
}

// SetError records the legacy executor error.
func (s *VirtualSubTask) SetError(err string) {
	s.err = err
}

// SetUpdatedAt records the virtual subtask update time.
func (s *VirtualSubTask) SetUpdatedAt(updatedAt time.Time) {
	s.updatedAt = updatedAt
}

// Result returns a copy of the recorded legacy result.
func (s *VirtualSubTask) Result() map[string]any {
	return cloneMap(s.result)
}

// Error returns the recorded legacy error.
func (s *VirtualSubTask) Error() string {
	return s.err
}

// UpdatedAt returns the last recorded update time.
func (s *VirtualSubTask) UpdatedAt() time.Time {
	return s.updatedAt
}

func stepResultFromSubTask(step agent.PlanStep, subTask *VirtualSubTask, output map[string]any, startedAt time.Time) *agent.StepResult {
	result := baseStepResult(step, "completed", startedAt)
	result.Result = cloneMap(output)
	if text, ok := output["output"].(string); ok {
		result.Output = text
	}
	result.CompletedAt = subTask.UpdatedAt()
	result.Metadata = map[string]any{
		"legacy_sub_task_id": subTask.GetSubTaskID(),
		"legacy_task_id":     subTask.GetTaskID(),
	}
	return result
}
