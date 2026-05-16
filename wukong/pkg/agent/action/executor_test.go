package action

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestActionContractsAreImplementable(t *testing.T) {
	var _ ActionExecutor = fakeExecutor{}
	var _ ActionRouter = fakeRouter{}
	var _ ActionRunner = fakeRunner{}
}

func TestStepResultJSONRoundTrip(t *testing.T) {
	result := agent.StepResult{
		StepID:    "step-1",
		Index:     0,
		Type:      agent.StepTypeTool,
		Action:    "search",
		Target:    "web",
		Status:    "completed",
		Output:    "done",
		Result:    map[string]any{"count": float64(2)},
		Metadata:  map[string]any{"executor": "fake"},
		SkillName: "lookup",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded agent.StepResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.StepID != result.StepID || decoded.Type != agent.StepTypeTool || decoded.Status != "completed" {
		t.Fatalf("decoded StepResult = %#v, want stable identity/type/status", decoded)
	}
	if decoded.Result["count"] != float64(2) {
		t.Fatalf("decoded Result = %#v, want count=2", decoded.Result)
	}
}

func TestActionResultJSONRoundTrip(t *testing.T) {
	result := agent.ActionResult{
		Status: "completed",
		Output: "all done",
		Result: map[string]any{"ok": true},
		StepResults: []agent.StepResult{
			{
				StepID: "step-1",
				Type:   agent.StepTypeSkill,
				Status: "completed",
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded agent.ActionResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Status != "completed" || decoded.Output != "all done" {
		t.Fatalf("decoded ActionResult = %#v, want completed output", decoded)
	}
	if len(decoded.StepResults) != 1 || decoded.StepResults[0].Type != agent.StepTypeSkill {
		t.Fatalf("decoded StepResults = %#v, want one skill result", decoded.StepResults)
	}
}

type fakeExecutor struct{}

func (fakeExecutor) Name() string {
	return "fake"
}

func (fakeExecutor) CanExecute(context.Context, agent.AgentContext, agent.PlanStep) bool {
	return true
}

func (fakeExecutor) Execute(context.Context, agent.AgentContext, agent.PlanStep) (*agent.StepResult, error) {
	return &agent.StepResult{Status: "completed"}, nil
}

type fakeRouter struct{}

func (fakeRouter) Route(context.Context, agent.AgentContext, agent.PlanStep, []ActionExecutor) (ActionExecutor, error) {
	return fakeExecutor{}, nil
}

type fakeRunner struct{}

func (fakeRunner) RunPlan(context.Context, agent.AgentContext, *agent.AgentPlan) (*agent.ActionResult, error) {
	return &agent.ActionResult{Status: "completed"}, nil
}
