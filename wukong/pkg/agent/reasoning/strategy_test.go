package reasoning

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestAgentPlanJSONRoundTrip(t *testing.T) {
	plan := agent.AgentPlan{
		PlanID:   "plan-1",
		Strategy: "direct",
		Goal:     "search docs",
		MaxSteps: 2,
		StopPolicy: agent.StopPolicy{
			MaxSteps:        2,
			StopOnError:     true,
			StopOnFinalStep: true,
		},
		Steps: []agent.PlanStep{
			{
				StepID:   "step-1",
				Type:     agent.StepTypeTool,
				Thought:  "Need fresh information",
				Action:   "search",
				Target:   "web",
				Params:   map[string]any{"query": "wukong"},
				Expected: "search results",
			},
			{
				StepID:   "step-2",
				Type:     agent.StepTypeFinal,
				Expected: "final answer",
			},
		},
		Metadata: map[string]any{"source": "test"},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded agent.AgentPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.PlanID != plan.PlanID || decoded.Strategy != plan.Strategy || decoded.Goal != plan.Goal {
		t.Fatalf("decoded plan = %#v, want %#v", decoded, plan)
	}
	if len(decoded.Steps) != 2 {
		t.Fatalf("decoded Steps length = %d, want 2", len(decoded.Steps))
	}
	if decoded.Steps[0].Type != agent.StepTypeTool || decoded.Steps[1].Type != agent.StepTypeFinal {
		t.Fatalf("decoded step types = %#v, want tool/final", decoded.Steps)
	}
	if decoded.StopPolicy.MaxSteps != 2 || !decoded.StopPolicy.StopOnError {
		t.Fatalf("decoded StopPolicy = %#v, want max steps and stop on error", decoded.StopPolicy)
	}
}

func TestStepTypeConstantsStable(t *testing.T) {
	tests := map[agent.StepType]string{
		agent.StepTypeLLM:   "llm",
		agent.StepTypeTool:  "tool",
		agent.StepTypeSkill: "skill",
		agent.StepTypeAgent: "agent",
		agent.StepTypeFinal: "final",
	}
	for stepType, want := range tests {
		if string(stepType) != want {
			t.Fatalf("StepType %v = %q, want %q", stepType, string(stepType), want)
		}
	}
}

func TestReasoningStrategyInterface(t *testing.T) {
	var _ ReasoningStrategy = fakeStrategy{}
}

type fakeStrategy struct{}

func (fakeStrategy) Name() string {
	return "fake"
}

func (fakeStrategy) Plan(context.Context, agent.AgentContext) (*agent.AgentPlan, error) {
	return &agent.AgentPlan{Strategy: "fake"}, nil
}

func (fakeStrategy) Revise(context.Context, agent.AgentContext, *agent.AgentPlan, *agent.Evaluation) (*agent.AgentPlan, error) {
	return &agent.AgentPlan{Strategy: "fake-revised"}, nil
}
