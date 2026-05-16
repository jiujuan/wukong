package reasoning

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

const directStrategyName = "direct"

// DirectStrategy turns one request into a single executable plan step.
type DirectStrategy struct{}

// NewDirectStrategy creates the simplest built-in reasoning strategy.
func NewDirectStrategy() DirectStrategy {
	return DirectStrategy{}
}

// Name returns the strategy name.
func (DirectStrategy) Name() string {
	return directStrategyName
}

// Plan builds a single-step plan from the request action, skill, or goal.
func (s DirectStrategy) Plan(ctx context.Context, agentCtx agent.AgentContext) (*agent.AgentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req := agentCtx.Request.Clone()
	step := directStep(req)

	return &agent.AgentPlan{
		PlanID:   directPlanID(req),
		Strategy: s.Name(),
		Goal:     directGoal(req),
		Steps:    []agent.PlanStep{step},
		MaxSteps: 1,
		StopPolicy: agent.StopPolicy{
			MaxSteps:        1,
			StopOnError:     true,
			StopOnFinalStep: true,
			RequireSuccess:  true,
		},
		CreatedAt: time.Now(),
	}, nil
}

// Revise returns a cloned plan with retry evaluation param patches applied.
func (s DirectStrategy) Revise(ctx context.Context, agentCtx agent.AgentContext, previous *agent.AgentPlan, eval *agent.Evaluation) (*agent.AgentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if previous == nil {
		return s.Plan(ctx, agentCtx)
	}
	plan := previous.Clone()
	plan.Strategy = s.Name()
	if patch := paramsPatch(eval); len(patch) > 0 && len(plan.Steps) > 0 {
		if plan.Steps[0].Params == nil {
			plan.Steps[0].Params = make(map[string]any, len(patch))
		}
		for key, value := range patch {
			plan.Steps[0].Params[key] = value
		}
	}
	return &plan, nil
}

func directStep(req agent.RunRequest) agent.PlanStep {
	step := agent.PlanStep{
		StepID:   "step-1",
		Params:   cloneMap(req.Params),
		Context:  cloneMap(req.Context),
		Expected: "complete request",
	}
	switch {
	case req.SkillName != "":
		step.Type = agent.StepTypeSkill
		step.SkillName = req.SkillName
		step.Target = req.SkillName
		step.Thought = "Execute the requested skill directly."
	case req.Action != "":
		step.Type = agent.StepTypeTool
		step.Action = req.Action
		step.Target = req.Action
		step.Thought = "Execute the requested action directly."
	default:
		step.Type = agent.StepTypeLLM
		step.Action = "respond"
		step.Target = "llm"
		step.Thought = "Answer the request directly."
	}
	return step
}

func directPlanID(req agent.RunRequest) string {
	if req.RunID != "" {
		return fmt.Sprintf("%s:direct", req.RunID)
	}
	if req.TaskID != "" {
		return fmt.Sprintf("%s:direct", req.TaskID)
	}
	return "direct"
}

func directGoal(req agent.RunRequest) string {
	switch {
	case req.Goal != "":
		return req.Goal
	case req.Action != "":
		return req.Action
	case req.SkillName != "":
		return req.SkillName
	default:
		return "complete request"
	}
}

func paramsPatch(eval *agent.Evaluation) map[string]any {
	if eval == nil || eval.Metadata == nil {
		return nil
	}
	if patch, ok := asMap(eval.Metadata["params_patch"]); ok {
		return cloneMap(patch)
	}
	if patch, ok := asMap(eval.Metadata["params"]); ok {
		return cloneMap(patch)
	}
	if patch, ok := asMap(eval.Metadata["patch"]); ok {
		if params, ok := asMap(patch["params"]); ok {
			return cloneMap(params)
		}
		return cloneMap(patch)
	}
	return nil
}

func asMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
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
