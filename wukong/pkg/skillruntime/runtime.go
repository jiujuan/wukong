package skillruntime

import "context"

const (
	// CallerTypeAgent identifies an agent runtime caller.
	CallerTypeAgent = "agent"
	// CallerTypeTask identifies a task executor caller.
	CallerTypeTask = "task"
	// CallerTypeChat identifies a chat service caller.
	CallerTypeChat = "chat"
	// CallerTypeCLI identifies a command-line caller.
	CallerTypeCLI = "cli"
)

// SkillRuntime discovers, prepares, and executes skills from one skill source.
type SkillRuntime interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	Discover(ctx context.Context) ([]SkillManifest, error)
	Get(ctx context.Context, name string) (*SkillSpec, error)
	Match(ctx context.Context, req SkillMatchRequest) ([]SkillCandidate, error)

	Prepare(ctx context.Context, activation SkillActivation, runtimeCtx RuntimeContext) (*PreparedSkill, error)
	Execute(ctx context.Context, prepared *PreparedSkill, input SkillInput) (*SkillOutput, error)
}

// Caller describes the system actor requesting a skill operation.
type Caller struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Role string `json:"role,omitempty"`
}

// RuntimeContext is the agent-independent context passed into skill runtimes.
type RuntimeContext struct {
	Caller    Caller         `json:"caller"`
	TaskID    string         `json:"task_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Goal      string         `json:"goal,omitempty"`
	Action    string         `json:"action,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	Memory    map[string]any `json:"memory,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// SkillMatchRequest describes the current caller need for skill matching.
type SkillMatchRequest struct {
	Caller    Caller         `json:"caller"`
	TaskID    string         `json:"task_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	SkillName string         `json:"skill_name,omitempty"`
	Action    string         `json:"action,omitempty"`
	Goal      string         `json:"goal,omitempty"`
	Query     string         `json:"query,omitempty"`
	Tools     []string       `json:"tools,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	Memory    map[string]any `json:"memory,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// SkillCandidate is one matched skill with its score and reason.
type SkillCandidate struct {
	Manifest SkillManifest  `json:"manifest"`
	Score    float64        `json:"score,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SkillActivation records why one skill is activated for a caller.
type SkillActivation struct {
	SkillName   string         `json:"skill_name"`
	RuntimeName string         `json:"runtime_name"`
	Reason      string         `json:"reason,omitempty"`
	Score       float64        `json:"score,omitempty"`
	RequestedBy string         `json:"requested_by,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
