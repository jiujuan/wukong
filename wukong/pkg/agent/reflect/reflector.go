package reflect

import (
	"context"

	"github.com/jiujuan/wukong/pkg/agent"
)

// Evaluation is the agent reflection result.
type Evaluation = agent.Evaluation

// Reflector evaluates plan execution results.
type Reflector interface {
	Reflect(ctx context.Context, agentCtx agent.AgentContext, plan *agent.AgentPlan, result *agent.ActionResult, execErr error) (*Evaluation, error)
}

// RetryDecision describes whether and how a failed result should be retried.
type RetryDecision struct {
	ShouldRetry bool           `json:"should_retry"`
	Strategy    string         `json:"strategy,omitempty"`
	ParamsPatch map[string]any `json:"params_patch,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

// NoopReflector accepts results without additional checks.
type NoopReflector struct{}

// Reflect implements Reflector.
func (NoopReflector) Reflect(ctx context.Context, _ agent.AgentContext, _ *agent.AgentPlan, result *agent.ActionResult, execErr error) (*Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if execErr != nil {
		return failedEvaluation(execErr.Error(), true), nil
	}
	if result != nil && result.Error != "" {
		return failedEvaluation(result.Error, true), nil
	}
	return &agent.Evaluation{
		Success: true,
		Score:   1,
		Reason:  "noop reflector accepted result",
	}, nil
}

func failedEvaluation(reason string, retry bool) *agent.Evaluation {
	decision := RetryDecision{
		ShouldRetry: retry,
		Strategy:    "revise",
		Reason:      reason,
	}
	return &agent.Evaluation{
		Success: false,
		Score:   0,
		Reason:  reason,
		Retry:   retry,
		Metadata: map[string]any{
			"retry_decision": decision,
		},
	}
}
