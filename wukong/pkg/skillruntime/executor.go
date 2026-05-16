package skillruntime

// SkillExecutionMode describes how a prepared skill should be executed.
type SkillExecutionMode string

const (
	// SkillExecutionModeContextOnly means the skill only contributes instructions or context.
	SkillExecutionModeContextOnly SkillExecutionMode = "context_only"
	// SkillExecutionModeTool means execution should call Wukong tools.
	SkillExecutionModeTool SkillExecutionMode = "tool"
	// SkillExecutionModeScript means execution should run a controlled script.
	SkillExecutionModeScript SkillExecutionMode = "script"
	// SkillExecutionModeRemote means execution should call a remote runtime.
	SkillExecutionModeRemote SkillExecutionMode = "remote"
)

// ContextBlock is a portable context fragment prepared by a skill runtime.
type ContextBlock struct {
	Name      string         `json:"name"`
	Type      string         `json:"type,omitempty"`
	Source    string         `json:"source,omitempty"`
	Content   string         `json:"content,omitempty"`
	Priority  int            `json:"priority,omitempty"`
	Tokens    int            `json:"tokens,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// PreparedSkill is the executable context produced by activating a skill.
type PreparedSkill struct {
	Spec          *SkillSpec         `json:"spec"`
	Activation    SkillActivation    `json:"activation"`
	ContextBlocks []ContextBlock     `json:"context_blocks,omitempty"`
	ToolPolicy    ToolPolicy         `json:"tool_policy,omitempty"`
	ExecutionMode SkillExecutionMode `json:"execution_mode,omitempty"`
	WorkDir       string             `json:"work_dir,omitempty"`
	Resources     []SkillResource    `json:"resources,omitempty"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
}

// SkillInput is the normalized input passed to a prepared skill execution.
type SkillInput struct {
	Params   map[string]any `json:"params,omitempty"`
	Context  RuntimeContext `json:"context"`
	Text     string         `json:"text,omitempty"`
	Files    []string       `json:"files,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SkillOutput is the normalized result returned by a skill execution.
type SkillOutput struct {
	Status    string          `json:"status"`
	Output    string          `json:"output,omitempty"`
	Result    map[string]any  `json:"result,omitempty"`
	Artifacts []SkillResource `json:"artifacts,omitempty"`
	Error     string          `json:"error,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}
