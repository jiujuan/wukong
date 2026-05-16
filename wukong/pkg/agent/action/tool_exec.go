package action

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

// ToolManager is the minimal tool dependency used by ToolExecutor.
type ToolManager interface {
	Execute(ctx context.Context, name string, params map[string]any) (map[string]any, error)
}

// ToolExecutor executes tool plan steps through an injected manager.
type ToolExecutor struct {
	manager ToolManager
}

// NewToolExecutor creates a tool action executor skeleton.
func NewToolExecutor(manager ToolManager) *ToolExecutor {
	return &ToolExecutor{manager: manager}
}

// Name returns the executor name.
func (e *ToolExecutor) Name() string {
	return "tool"
}

// CanExecute reports whether this executor can handle the step.
func (e *ToolExecutor) CanExecute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) bool {
	return ctx.Err() == nil && step.Type == agent.StepTypeTool
}

// Execute calls the injected tool manager and records the map result.
func (e *ToolExecutor) Execute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	startedAt := time.Now()
	result := baseStepResult(step, "running", startedAt)
	if err := ctx.Err(); err != nil {
		return failStepResult(result, err), err
	}
	if e.manager == nil {
		err := fmt.Errorf("tool manager is not configured")
		return failStepResult(result, err), err
	}
	toolName := step.Action
	if toolName == "" {
		toolName = step.Target
	}
	if toolName == "" {
		err := fmt.Errorf("tool step %q missing action or target", step.StepID)
		return failStepResult(result, err), err
	}

	output, err := e.manager.Execute(ctx, toolName, cloneMap(step.Params))
	if err != nil {
		return failStepResult(result, err), err
	}
	result.Status = "completed"
	result.Result = output
	result.CompletedAt = time.Now()
	return result, nil
}

func baseStepResult(step agent.PlanStep, status string, startedAt time.Time) *agent.StepResult {
	return &agent.StepResult{
		StepID:    step.StepID,
		Type:      step.Type,
		Action:    step.Action,
		Target:    step.Target,
		SkillName: step.SkillName,
		AgentID:   step.AgentID,
		Status:    status,
		StartedAt: startedAt,
		Metadata: map[string]any{
			"expected": step.Expected,
		},
	}
}

func failStepResult(result *agent.StepResult, err error) *agent.StepResult {
	result.Status = "failed"
	if err != nil {
		result.Error = err.Error()
	}
	result.CompletedAt = time.Now()
	return result
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
