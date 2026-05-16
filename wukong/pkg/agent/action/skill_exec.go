package action

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/skillruntime"
)

// SkillRuntimeRegistry is the minimal registry dependency used by SkillRuntimeExecutor.
type SkillRuntimeRegistry interface {
	Resolve(skillName string) (skillruntime.SkillRuntime, bool)
}

// SkillRuntimeContextMapper maps Agent context into agent-independent skill runtime context.
type SkillRuntimeContextMapper interface {
	Map(agentCtx agent.AgentContext, step agent.PlanStep) skillruntime.RuntimeContext
}

// DefaultSkillRuntimeContextMapper is the default AgentContext to RuntimeContext mapper.
type DefaultSkillRuntimeContextMapper struct{}

// Map converts AgentContext into skillruntime.RuntimeContext without leaking agent types.
func (DefaultSkillRuntimeContextMapper) Map(agentCtx agent.AgentContext, step agent.PlanStep) skillruntime.RuntimeContext {
	req := agentCtx.Request
	params := cloneMap(req.Params)
	for key, value := range step.Params {
		if params == nil {
			params = make(map[string]any)
		}
		params[key] = value
	}

	memory := cloneMap(agentCtx.SharedMemory)
	if len(agentCtx.WorkingMemory) > 0 || len(agentCtx.LongMemory) > 0 {
		if memory == nil {
			memory = make(map[string]any)
		}
		memory["working_count"] = len(agentCtx.WorkingMemory)
		memory["long_count"] = len(agentCtx.LongMemory)
	}

	return skillruntime.RuntimeContext{
		Caller: skillruntime.Caller{
			Type: skillruntime.CallerTypeAgent,
			ID:   string(agentCtx.Agent.ID),
			Role: string(agentCtx.Agent.Role),
		},
		TaskID:    req.TaskID,
		SessionID: req.SessionID,
		Goal:      req.Goal,
		Action:    step.Action,
		Params:    params,
		Memory:    memory,
		Metadata: map[string]any{
			"run_id":     req.RunID,
			"step_id":    step.StepID,
			"skill_name": step.SkillName,
		},
	}
}

// SkillRuntimeExecutor executes skill plan steps through pkg/skillruntime.
type SkillRuntimeExecutor struct {
	registry SkillRuntimeRegistry
	mapper   SkillRuntimeContextMapper
}

// NewSkillRuntimeExecutor creates a skill runtime action executor.
func NewSkillRuntimeExecutor(registry SkillRuntimeRegistry, mapper SkillRuntimeContextMapper) *SkillRuntimeExecutor {
	if mapper == nil {
		mapper = DefaultSkillRuntimeContextMapper{}
	}
	return &SkillRuntimeExecutor{registry: registry, mapper: mapper}
}

// Name returns the executor name.
func (e *SkillRuntimeExecutor) Name() string {
	return "skillruntime"
}

// CanExecute reports whether this executor can handle the step.
func (e *SkillRuntimeExecutor) CanExecute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) bool {
	return ctx.Err() == nil && step.Type == agent.StepTypeSkill
}

// Execute resolves, prepares, and executes the requested skill runtime.
func (e *SkillRuntimeExecutor) Execute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	startedAt := time.Now()
	result := baseStepResult(step, "running", startedAt)
	if err := ctx.Err(); err != nil {
		return failStepResult(result, err), err
	}
	if e.registry == nil {
		err := fmt.Errorf("skill runtime registry is not configured")
		return failStepResult(result, err), err
	}
	skillName := step.SkillName
	if skillName == "" {
		skillName = step.Target
	}
	if skillName == "" {
		err := fmt.Errorf("skill step %q missing skill name or target", step.StepID)
		return failStepResult(result, err), err
	}

	runtime, ok := e.registry.Resolve(skillName)
	if !ok || runtime == nil {
		err := fmt.Errorf("skill runtime not found: %s", skillName)
		return failStepResult(result, err), err
	}
	runtimeCtx := e.mapper.Map(agentCtx, step)
	activation := skillruntime.SkillActivation{
		SkillName:   skillName,
		RuntimeName: runtime.Name(),
		Reason:      step.Thought,
		RequestedBy: string(agentCtx.Agent.ID),
		Params:      cloneMap(step.Params),
		Metadata: map[string]any{
			"step_id": step.StepID,
		},
	}
	prepared, err := runtime.Prepare(ctx, activation, runtimeCtx)
	if err != nil {
		return failStepResult(result, err), err
	}
	output, err := runtime.Execute(ctx, prepared, skillruntime.SkillInput{
		Params:  cloneMap(step.Params),
		Context: runtimeCtx,
		Text:    step.Expected,
		Metadata: map[string]any{
			"step_id": step.StepID,
		},
	})
	if err != nil {
		return failStepResult(result, err), err
	}
	applySkillOutput(result, output)
	result.CompletedAt = time.Now()
	return result, nil
}

func applySkillOutput(result *agent.StepResult, output *skillruntime.SkillOutput) {
	if output == nil {
		result.Status = "completed"
		return
	}
	result.Status = output.Status
	if result.Status == "" {
		result.Status = "completed"
	}
	result.Output = output.Output
	result.Result = cloneMap(output.Result)
	result.Error = output.Error
	result.Metadata = cloneMap(output.Metadata)
	if len(output.Artifacts) > 0 {
		if result.Metadata == nil {
			result.Metadata = make(map[string]any)
		}
		result.Metadata["artifacts_count"] = len(output.Artifacts)
	}
}
