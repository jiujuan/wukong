package reasoning

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestDirectStrategyRequestActionToActionStep(t *testing.T) {
	strategy := NewDirectStrategy()
	plan, err := strategy.Plan(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID:  "run-1",
			TaskID: "task-1",
			Action: "search",
			Params: map[string]any{
				"query": "wukong",
			},
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.Strategy != "direct" || plan.MaxSteps != 1 {
		t.Fatalf("plan = %#v, want direct single-step plan", plan)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("Steps length = %d, want 1", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.Type != agent.StepTypeTool || step.Action != "search" || step.Target != "search" {
		t.Fatalf("step = %#v, want tool action search", step)
	}
	if step.Params["query"] != "wukong" {
		t.Fatalf("Params = %#v, want query=wukong", step.Params)
	}
}

func TestDirectStrategyRequestSkillNameToSkillStep(t *testing.T) {
	strategy := NewDirectStrategy()
	plan, err := strategy.Plan(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID:     "run-1",
			SkillName: "report",
			Params: map[string]any{
				"format": "md",
			},
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("Steps length = %d, want 1", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.Type != agent.StepTypeSkill || step.SkillName != "report" || step.Target != "report" {
		t.Fatalf("step = %#v, want skill report", step)
	}
	if step.Params["format"] != "md" {
		t.Fatalf("Params = %#v, want format=md", step.Params)
	}
}

func TestDirectStrategyReviseAppliesParamsPatch(t *testing.T) {
	strategy := NewDirectStrategy()
	previous := &agent.AgentPlan{
		PlanID:   "plan-1",
		Strategy: "direct",
		Steps: []agent.PlanStep{
			{
				StepID: "step-1",
				Type:   agent.StepTypeTool,
				Action: "search",
				Params: map[string]any{
					"query": "old",
					"limit": 3,
				},
			},
		},
	}
	eval := &agent.Evaluation{
		Retry: true,
		Metadata: map[string]any{
			"params_patch": map[string]any{
				"query": "new",
				"limit": 5,
			},
		},
	}

	revised, err := strategy.Revise(context.Background(), agent.AgentContext{}, previous, eval)
	if err != nil {
		t.Fatalf("Revise() error = %v", err)
	}

	if revised.Steps[0].Params["query"] != "new" || revised.Steps[0].Params["limit"] != 5 {
		t.Fatalf("revised Params = %#v, want patched query and limit", revised.Steps[0].Params)
	}
	if previous.Steps[0].Params["query"] != "old" || previous.Steps[0].Params["limit"] != 3 {
		t.Fatalf("previous Params mutated = %#v", previous.Steps[0].Params)
	}
}

func TestDirectStrategyDefaultsToLLMStep(t *testing.T) {
	strategy := NewDirectStrategy()
	plan, err := strategy.Plan(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID: "run-1",
			Goal:  "summarize context",
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Steps[0].Type != agent.StepTypeLLM {
		t.Fatalf("step Type = %q, want llm", plan.Steps[0].Type)
	}
}
