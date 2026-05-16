package reflect

import (
	"context"
	"strings"

	"github.com/jiujuan/wukong/pkg/agent"
)

// RuleReflector evaluates results with deterministic low-cost checks.
type RuleReflector struct {
	MinScore float64
}

// NewRuleReflector creates a rule-based reflector.
func NewRuleReflector() RuleReflector {
	return RuleReflector{MinScore: 0.5}
}

// Reflect implements Reflector.
func (r RuleReflector) Reflect(ctx context.Context, _ agent.AgentContext, _ *agent.AgentPlan, result *agent.ActionResult, execErr error) (*Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if execErr != nil {
		return failedEvaluation(execErr.Error(), true), nil
	}
	if result == nil {
		return failedEvaluation("empty action result", true), nil
	}
	if result.Error != "" {
		return failedEvaluation(result.Error, true), nil
	}
	if strings.TrimSpace(result.Output) == "" && len(result.Result) == 0 {
		return failedEvaluation("empty action output", true), nil
	}

	score := 1.0
	if raw, ok := result.Metadata["confidence"].(float64); ok {
		score = raw
	}
	if raw, ok := result.Metadata["score"].(float64); ok {
		score = raw
	}
	if score < r.threshold() {
		return failedEvaluation("low confidence action result", true), nil
	}
	return &agent.Evaluation{
		Success: true,
		Score:   score,
		Reason:  "rule reflector accepted result",
	}, nil
}

func (r RuleReflector) threshold() float64 {
	if r.MinScore <= 0 {
		return 0.5
	}
	return r.MinScore
}
