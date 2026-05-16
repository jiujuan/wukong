package action

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/agent/collaboration"
)

const handoffDepthContextKey = "handoff_depth"

// AgentRuntime is the minimal runtime dependency used for same-process handoff.
type AgentRuntime interface {
	Run(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error)
}

// AgentExecutor delegates agent plan steps to a child AgentRuntime run.
type AgentExecutor struct {
	runtime AgentRuntime
	router  collaboration.AgentRouter
}

// NewAgentExecutor creates an executor for delegated agent steps.
func NewAgentExecutor(runtime AgentRuntime, router collaboration.AgentRouter) *AgentExecutor {
	return &AgentExecutor{runtime: runtime, router: router}
}

// Name returns the executor name.
func (e *AgentExecutor) Name() string {
	return "agent"
}

// CanExecute reports whether this executor can handle delegated agent steps.
func (e *AgentExecutor) CanExecute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) bool {
	return ctx.Err() == nil && step.Type == agent.StepTypeAgent
}

// Execute routes a delegated step, creates a child run, and converts the child result.
func (e *AgentExecutor) Execute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	startedAt := time.Now()
	result := baseStepResult(step, "running", startedAt)
	if err := ctx.Err(); err != nil {
		return failStepResult(result, err), err
	}
	if e.runtime == nil {
		err := fmt.Errorf("agent runtime is not configured")
		return failStepResult(result, err), err
	}
	if e.router == nil {
		err := fmt.Errorf("agent router is not configured")
		return failStepResult(result, err), err
	}
	if err := checkHandoffDepth(agentCtx); err != nil {
		return failStepResult(result, err), err
	}

	toProfile, err := e.router.RouteHandoff(ctx, agentCtx.Agent, step)
	if err != nil {
		return failStepResult(result, err), err
	}

	handoff := buildHandoffRequest(agentCtx, step, toProfile)
	childReq := handoffRunRequest(agentCtx, handoff)
	childResult, err := e.runtime.Run(ctx, childReq)
	if err != nil {
		return failStepResult(result, err), err
	}

	return stepResultFromRunResult(step, handoff, childResult, startedAt), nil
}

func checkHandoffDepth(agentCtx agent.AgentContext) error {
	maxDepth := agentCtx.Agent.Collaboration.MaxDepth
	if maxDepth <= 0 {
		return nil
	}
	depth := handoffDepth(agentCtx.Request.Context)
	if depth >= maxDepth {
		return fmt.Errorf("handoff depth limit exceeded")
	}
	return nil
}

func buildHandoffRequest(agentCtx agent.AgentContext, step agent.PlanStep, toProfile agent.AgentProfile) collaboration.HandoffRequest {
	return collaboration.HandoffRequest{
		HandoffID:   handoffID(agentCtx.Request, step),
		FromAgentID: agentCtx.Agent.ID,
		ToAgentID:   toProfile.ID,
		ParentRunID: agentCtx.Request.RunID,
		Goal:        handoffGoal(agentCtx.Request, step),
		Action:      step.Action,
		SkillName:   step.SkillName,
		Params:      cloneMap(step.Params),
		Context:     handoffContext(agentCtx.Request.Context, step.Context),
		Contract: collaboration.HandoffContract{
			ExpectedOutput:       step.Expected,
			AllowFurtherDelegate: agentCtx.Agent.Collaboration.MaxDepth == 0 || handoffDepth(agentCtx.Request.Context)+1 < agentCtx.Agent.Collaboration.MaxDepth,
		},
	}
}

func handoffRunRequest(agentCtx agent.AgentContext, handoff collaboration.HandoffRequest) agent.RunRequest {
	child := agentCtx.Request.Clone()
	child.RunID = handoff.HandoffID
	child.SubTaskID = handoff.HandoffID
	child.AgentID = handoff.ToAgentID
	child.ParentRunID = handoff.ParentRunID
	child.Goal = handoff.Goal
	child.Action = handoff.Action
	child.SkillName = handoff.SkillName
	child.Params = cloneMap(handoff.Params)
	child.Context = cloneMap(handoff.Context)
	child.Constraints.AllowDelegate = handoff.Contract.AllowFurtherDelegate
	return child
}

func stepResultFromRunResult(step agent.PlanStep, handoff collaboration.HandoffRequest, runResult *agent.RunResult, startedAt time.Time) *agent.StepResult {
	result := baseStepResult(step, "completed", startedAt)
	result.AgentID = handoff.ToAgentID
	result.CompletedAt = time.Now()
	result.Metadata["handoff_id"] = handoff.HandoffID
	result.Metadata["parent_run_id"] = handoff.ParentRunID
	result.Metadata["to_agent_id"] = string(handoff.ToAgentID)
	if runResult == nil {
		result.Status = "failed"
		result.Error = "child agent returned nil result"
		return result
	}
	result.Status = runResult.Status
	result.Output = runResult.Output
	result.Result = cloneMap(runResult.Result)
	result.Error = runResult.Error
	result.Metadata["child_run_id"] = runResult.RunID
	return result
}

func handoffID(req agent.RunRequest, step agent.PlanStep) string {
	base := req.RunID
	if base == "" {
		base = req.TaskID
	}
	if base == "" {
		base = "run"
	}
	stepID := step.StepID
	if stepID == "" {
		stepID = "agent"
	}
	return base + ":" + stepID
}

func handoffGoal(req agent.RunRequest, step agent.PlanStep) string {
	if step.Expected != "" {
		return step.Expected
	}
	if step.Action != "" {
		return step.Action
	}
	if step.SkillName != "" {
		return step.SkillName
	}
	return req.Goal
}

func handoffContext(parent map[string]any, step map[string]any) map[string]any {
	out := cloneMap(parent)
	if out == nil {
		out = make(map[string]any)
	}
	for key, value := range step {
		out[key] = value
	}
	out[handoffDepthContextKey] = handoffDepth(parent) + 1
	return out
}

func handoffDepth(context map[string]any) int {
	if context == nil {
		return 0
	}
	switch depth := context[handoffDepthContextKey].(type) {
	case int:
		return depth
	case int64:
		return int(depth)
	case float64:
		return int(depth)
	default:
		return 0
	}
}
