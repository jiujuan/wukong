package agent

import "context"

// LoopController centralizes Agent Loop control decisions.
type LoopController interface {
	BeforeRun(ctx context.Context, state *LoopState) (*LoopDecision, error)
	BeforeIteration(ctx context.Context, state *LoopState) (*LoopDecision, error)
	AfterIteration(ctx context.Context, state *LoopState) (*LoopDecision, error)
	OnError(ctx context.Context, state *LoopState, err error) (*LoopDecision, error)
	OnHumanResponse(ctx context.Context, state *LoopState, input HumanInput) (*LoopDecision, error)
	BeforeStop(ctx context.Context, state *LoopState) (*LoopDecision, error)
}

// LoopPhase identifies the current Agent Loop phase.
type LoopPhase string

const (
	LoopPhasePerceive      LoopPhase = "perceive"
	LoopPhaseSkillActivate LoopPhase = "skill_activate"
	LoopPhasePlan          LoopPhase = "plan"
	LoopPhaseAct           LoopPhase = "act"
	LoopPhaseObserve       LoopPhase = "observe"
	LoopPhaseReflect       LoopPhase = "reflect"
	LoopPhaseLearn         LoopPhase = "learn"
	LoopPhasePaused        LoopPhase = "paused"
)

// LoopStatus identifies the lifecycle status of a loop run.
type LoopStatus string

const (
	LoopStatusPending   LoopStatus = "pending"
	LoopStatusRunning   LoopStatus = "running"
	LoopStatusPaused    LoopStatus = "paused"
	LoopStatusCompleted LoopStatus = "completed"
	LoopStatusFailed    LoopStatus = "failed"
	LoopStatusStopped   LoopStatus = "stopped"
)

// LoopDecision describes whether the loop should continue, stop, retry, revise, pause, or resume.
type LoopDecision struct {
	Continue bool           `json:"continue"`
	Stop     bool           `json:"stop"`
	Retry    bool           `json:"retry"`
	Revise   bool           `json:"revise"`
	Pause    bool           `json:"pause"`
	Resume   bool           `json:"resume"`
	Reason   string         `json:"reason,omitempty"`
	Patch    map[string]any `json:"patch,omitempty"`
	WaitFor  *HumanRequest  `json:"wait_for,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// LoopState captures the complete mutable runtime scene for one Agent Loop run.
type LoopState struct {
	RunID        string         `json:"run_id"`
	Iteration    int            `json:"iteration"`
	Phase        LoopPhase      `json:"phase"`
	Request      RunRequest     `json:"request"`
	Agent        AgentProfile   `json:"agent"`
	AgentState   AgentState     `json:"agent_state"`
	AgentContext AgentContext   `json:"agent_context"`
	Plan         map[string]any `json:"plan,omitempty"`
	StepCursor   int            `json:"step_cursor"`
	StepResults  []LoopStep     `json:"step_results,omitempty"`
	ActionResult *ActionResult  `json:"action_result,omitempty"`
	Evaluation   *Evaluation    `json:"evaluation,omitempty"`
	LastError    string         `json:"last_error,omitempty"`
	Status       LoopStatus     `json:"status"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// NewLoopState creates the initial mutable state for a run.
func NewLoopState(req RunRequest, profile AgentProfile, agentState AgentState, agentCtx AgentContext) *LoopState {
	runID := req.RunID
	if runID == "" {
		runID = agentState.CurrentTaskID
	}
	return &LoopState{
		RunID:        runID,
		Phase:        LoopPhasePerceive,
		Request:      req.Clone(),
		Agent:        profile,
		AgentState:   agentState,
		AgentContext: agentCtx,
		Status:       LoopStatusPending,
	}
}

// Done reports whether the loop has reached a terminal state.
func (s *LoopState) Done() bool {
	if s == nil {
		return true
	}
	switch s.Status {
	case LoopStatusCompleted, LoopStatusFailed, LoopStatusStopped:
		return true
	default:
		return false
	}
}

// HumanRequest describes one human-in-loop pause request.
type HumanRequest struct {
	RequestID string         `json:"request_id"`
	RunID     string         `json:"run_id"`
	Type      string         `json:"type"`
	Prompt    string         `json:"prompt"`
	Options   []HumanOption  `json:"options,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// HumanOption describes one selectable human response option.
type HumanOption struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Value       string         `json:"value,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// HumanInput records the human response used to resume a paused loop.
type HumanInput struct {
	RequestID string         `json:"request_id"`
	RunID     string         `json:"run_id"`
	Approved  bool           `json:"approved"`
	Content   string         `json:"content,omitempty"`
	Choice    string         `json:"choice,omitempty"`
	Patch     map[string]any `json:"patch,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
