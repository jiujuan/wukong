package agent

import "time"

// StepType identifies the execution kind of one plan step.
type StepType string

const (
	StepTypeLLM   StepType = "llm"
	StepTypeTool  StepType = "tool"
	StepTypeSkill StepType = "skill"
	StepTypeAgent StepType = "agent"
	StepTypeFinal StepType = "final"
)

// StopPolicy describes when a plan should stop executing.
type StopPolicy struct {
	MaxSteps        int           `json:"max_steps,omitempty"`
	MaxDuration     time.Duration `json:"max_duration,omitempty"`
	StopOnError     bool          `json:"stop_on_error,omitempty"`
	StopOnFinalStep bool          `json:"stop_on_final_step,omitempty"`
	RequireSuccess  bool          `json:"require_success,omitempty"`
}

// AgentPlan is the strategy-produced execution plan for an Agent Loop run.
type AgentPlan struct {
	PlanID     string         `json:"plan_id"`
	Strategy   string         `json:"strategy"`
	Goal       string         `json:"goal"`
	Steps      []PlanStep     `json:"steps"`
	MaxSteps   int            `json:"max_steps,omitempty"`
	StopPolicy StopPolicy     `json:"stop_policy"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
}

// PlanStep describes one executable unit inside an AgentPlan.
type PlanStep struct {
	StepID    string         `json:"step_id"`
	Type      StepType       `json:"type"`
	Thought   string         `json:"thought,omitempty"`
	Action    string         `json:"action,omitempty"`
	Target    string         `json:"target,omitempty"`
	SkillName string         `json:"skill_name,omitempty"`
	AgentID   AgentID        `json:"agent_id,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	Context   map[string]any `json:"context,omitempty"`
	Expected  string         `json:"expected,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Clone returns an independent plan copy.
func (p AgentPlan) Clone() AgentPlan {
	out := p
	out.Steps = clonePlanSteps(p.Steps)
	out.Metadata = cloneMap(p.Metadata)
	return out
}

func clonePlanSteps(in []PlanStep) []PlanStep {
	if in == nil {
		return nil
	}
	out := make([]PlanStep, len(in))
	for i, step := range in {
		out[i] = step
		out[i].Params = cloneMap(step.Params)
		out[i].Context = cloneMap(step.Context)
		out[i].Metadata = cloneMap(step.Metadata)
	}
	return out
}
