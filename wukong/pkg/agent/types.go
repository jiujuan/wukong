package agent

import "time"

// AgentID identifies an agent profile and its runtime state.
type AgentID string

// AgentRole describes the primary responsibility of an agent.
type AgentRole string

const (
	AgentRoleGeneral  AgentRole = "general"
	AgentRoleResearch AgentRole = "research"
	AgentRoleTool     AgentRole = "tool"
	AgentRoleCritic   AgentRole = "critic"
)

// AgentProfile describes an agent identity, capability boundary, and default behavior preferences.
type AgentProfile struct {
	ID            AgentID             `json:"id"`
	Name          string              `json:"name"`
	Role          AgentRole           `json:"role"`
	Description   string              `json:"description,omitempty"`
	Goal          string              `json:"goal,omitempty"`
	Capabilities  []Capability        `json:"capabilities,omitempty"`
	Tools         []string            `json:"tools,omitempty"`
	Skills        []string            `json:"skills,omitempty"`
	Reasoning     ReasoningConfig     `json:"reasoning,omitempty"`
	Memory        MemoryConfig        `json:"memory,omitempty"`
	Reflection    ReflectConfig       `json:"reflection,omitempty"`
	Collaboration CollaborationConfig `json:"collaboration,omitempty"`
	Metadata      map[string]any      `json:"metadata,omitempty"`
}

// Capability is a structured matching source for routing actions, skills, and tools to agents.
type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Priority    int      `json:"priority,omitempty"`
}

// ReasoningConfig keeps the profile-level reasoning defaults without binding to a concrete strategy.
type ReasoningConfig struct {
	Strategy   string         `json:"strategy,omitempty"`
	Depth      string         `json:"depth,omitempty"`
	MaxRetries int            `json:"max_retries,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// MemoryConfig describes which memory layers the agent should consume or write.
type MemoryConfig struct {
	Enabled        bool           `json:"enabled,omitempty"`
	WorkingEnabled bool           `json:"working_enabled,omitempty"`
	LongEnabled    bool           `json:"long_enabled,omitempty"`
	SharedEnabled  bool           `json:"shared_enabled,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ReflectConfig describes profile-level reflection defaults.
type ReflectConfig struct {
	Enabled    bool           `json:"enabled,omitempty"`
	Strategy   string         `json:"strategy,omitempty"`
	MaxRetries int            `json:"max_retries,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// CollaborationConfig describes whether an agent can delegate or receive handoffs.
type CollaborationConfig struct {
	CanDelegate  bool           `json:"can_delegate,omitempty"`
	CanBeTarget  bool           `json:"can_be_target,omitempty"`
	MaxDepth     int            `json:"max_depth,omitempty"`
	AllowedRoles []AgentRole    `json:"allowed_roles,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// AgentStatus describes the lifecycle state of an agent run.
type AgentStatus string

const (
	AgentStatusIdle       AgentStatus = "idle"
	AgentStatusRunning    AgentStatus = "running"
	AgentStatusWaiting    AgentStatus = "waiting"
	AgentStatusReflecting AgentStatus = "reflecting"
	AgentStatusCompleted  AgentStatus = "completed"
	AgentStatusFailed     AgentStatus = "failed"
)

// AgentState captures the mutable runtime state of one agent.
type AgentState struct {
	AgentID       AgentID        `json:"agent_id"`
	Status        AgentStatus    `json:"status"`
	CurrentTaskID string         `json:"current_task_id,omitempty"`
	CurrentStep   int            `json:"current_step"`
	Goal          string         `json:"goal,omitempty"`
	Scratchpad    map[string]any `json:"scratchpad,omitempty"`
	LastResult    *ActionResult  `json:"last_result,omitempty"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// RunRequest is the independent protocol used to invoke the Agent Runtime.
type RunRequest struct {
	RunID       string         `json:"run_id"`
	TaskID      string         `json:"task_id"`
	SubTaskID   string         `json:"sub_task_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	SkillName   string         `json:"skill_name,omitempty"`
	Action      string         `json:"action,omitempty"`
	Goal        string         `json:"goal,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	AgentID     AgentID        `json:"agent_id,omitempty"`
	ParentRunID string         `json:"parent_run_id,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
	Constraints RunConstraints `json:"constraints,omitempty"`
}

// Clone returns a copy of the request with independent mutable maps.
func (r RunRequest) Clone() RunRequest {
	clone := r
	clone.Params = cloneMap(r.Params)
	clone.Context = cloneMap(r.Context)
	return clone
}

// RunConstraints contains runtime budgets and permissions for one run.
type RunConstraints struct {
	MaxSteps        int           `json:"max_steps,omitempty"`
	MaxIterations   int           `json:"max_iterations,omitempty"`
	Timeout         time.Duration `json:"timeout,omitempty"`
	TokenBudget     int           `json:"token_budget,omitempty"`
	AllowDelegate   bool          `json:"allow_delegate,omitempty"`
	AllowReflection bool          `json:"allow_reflection,omitempty"`
}

// RunResult is the Agent Runtime response returned to callers and adapters.
type RunResult struct {
	RunID       string         `json:"run_id"`
	AgentID     AgentID        `json:"agent_id"`
	TaskID      string         `json:"task_id"`
	SubTaskID   string         `json:"sub_task_id,omitempty"`
	Status      string         `json:"status"`
	Strategy    string         `json:"strategy,omitempty"`
	Output      string         `json:"output,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	Steps       []LoopStep     `json:"steps,omitempty"`
	Evaluation  *Evaluation    `json:"evaluation,omitempty"`
	Usage       *Usage         `json:"usage,omitempty"`
	Error       string         `json:"error,omitempty"`
	CompletedAt time.Time      `json:"completed_at"`
}

// AgentContext is the full context consumed by planning, reasoning, and action execution.
type AgentContext struct {
	Request         RunRequest       `json:"request"`
	Agent           AgentProfile     `json:"agent"`
	State           AgentState       `json:"state"`
	WorkingMemory   []MemoryItem     `json:"working_memory,omitempty"`
	LongMemory      []MemoryItem     `json:"long_memory,omitempty"`
	SharedMemory    map[string]any   `json:"shared_memory,omitempty"`
	SkillSpec       map[string]any   `json:"skill_spec,omitempty"`
	ActivatedSkills []ActivatedSkill `json:"activated_skills,omitempty"`
	ToolCatalog     []ToolDescriptor `json:"tool_catalog,omitempty"`
	Trace           []AgentEvent     `json:"trace,omitempty"`
}

// MemorySnapshot groups memory loaded before planning.
type MemorySnapshot struct {
	Working []MemoryItem   `json:"working,omitempty"`
	Long    []MemoryItem   `json:"long,omitempty"`
	Shared  map[string]any `json:"shared,omitempty"`
}

// Usage records resource consumption for one run.
type Usage struct {
	PromptTokens     int            `json:"prompt_tokens,omitempty"`
	CompletionTokens int            `json:"completion_tokens,omitempty"`
	TotalTokens      int            `json:"total_tokens,omitempty"`
	ToolCalls        int            `json:"tool_calls,omitempty"`
	Duration         time.Duration  `json:"duration,omitempty"`
	Cost             float64        `json:"cost,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// LoopStep records a visible step completed by the Agent Loop.
type LoopStep struct {
	Index       int            `json:"index"`
	Phase       string         `json:"phase,omitempty"`
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Action      string         `json:"action,omitempty"`
	SkillName   string         `json:"skill_name,omitempty"`
	Status      string         `json:"status"`
	Input       map[string]any `json:"input,omitempty"`
	Output      string         `json:"output,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// StepResult records the execution result of one AgentPlan step.
type StepResult struct {
	StepID      string         `json:"step_id"`
	Index       int            `json:"index"`
	Type        StepType       `json:"type"`
	Action      string         `json:"action,omitempty"`
	Target      string         `json:"target,omitempty"`
	SkillName   string         `json:"skill_name,omitempty"`
	AgentID     AgentID        `json:"agent_id,omitempty"`
	Status      string         `json:"status"`
	Output      string         `json:"output,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ActionResult is the aggregate result produced by action execution.
type ActionResult struct {
	Status      string         `json:"status"`
	Output      string         `json:"output,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	StepResults []StepResult   `json:"step_results,omitempty"`
	Steps       []LoopStep     `json:"steps,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Evaluation is the reflection result for a run or plan execution.
type Evaluation struct {
	Success  bool           `json:"success"`
	Score    float64        `json:"score,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Retry    bool           `json:"retry,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemoryItem is a normalized memory record available to the Agent Loop.
type MemoryItem struct {
	ID        string         `json:"id,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Key       string         `json:"key,omitempty"`
	Type      string         `json:"type,omitempty"`
	Content   string         `json:"content"`
	Score     float64        `json:"score,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

// ActivatedSkill describes a skill selected for the current run.
type ActivatedSkill struct {
	Name         string         `json:"name"`
	Runtime      string         `json:"runtime,omitempty"`
	Instructions string         `json:"instructions,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ToolDescriptor describes a tool visible to an agent.
type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// AgentEvent records a trace event emitted during a run.
type AgentEvent struct {
	RunID     string         `json:"run_id,omitempty"`
	Type      string         `json:"type"`
	Message   string         `json:"message,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
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
