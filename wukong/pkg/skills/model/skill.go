package model

// CanonicalSkill is the unified internal representation for all skill sources.
type CanonicalSkill struct {
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	Instructions string           `json:"instructions,omitempty"`
	AllowedTools []string         `json:"allowed_tools,omitempty"`
	Runtime      SkillRuntimeSpec `json:"runtime,omitempty"`
	Source       SkillSource      `json:"source,omitempty"`
	References   []SkillResource  `json:"references,omitempty"`
	Assets       []SkillResource  `json:"assets,omitempty"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
	Enabled      bool             `json:"enabled,omitempty"`
	Version      string           `json:"version,omitempty"`
}
