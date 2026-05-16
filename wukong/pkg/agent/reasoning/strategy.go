package reasoning

import (
	"context"

	"github.com/jiujuan/wukong/pkg/agent"
)

// ReasoningStrategy produces and revises Agent execution plans.
type ReasoningStrategy interface {
	Name() string
	Plan(ctx context.Context, agentCtx agent.AgentContext) (*agent.AgentPlan, error)
	Revise(ctx context.Context, agentCtx agent.AgentContext, previous *agent.AgentPlan, eval *agent.Evaluation) (*agent.AgentPlan, error)
}
