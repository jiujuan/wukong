package action

import (
	"context"

	"github.com/jiujuan/wukong/pkg/agent"
)

// ActionExecutor executes one plan step for an Agent Loop.
type ActionExecutor interface {
	Name() string
	CanExecute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) bool
	Execute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error)
}

// ActionRouter chooses an executor for one plan step.
type ActionRouter interface {
	Route(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep, executors []ActionExecutor) (ActionExecutor, error)
}

// ActionRunner executes a full plan and returns an aggregate action result.
type ActionRunner interface {
	RunPlan(ctx context.Context, agentCtx agent.AgentContext, plan *agent.AgentPlan) (*agent.ActionResult, error)
}
