package skillruntime

// SkillManifest is the lightweight metadata used for discovery and matching.
type SkillManifest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Runtime       string            `json:"runtime"`
	Version       string            `json:"version,omitempty"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// SkillSpec is the complete skill definition loaded only when needed.
type SkillSpec struct {
	Manifest     SkillManifest   `json:"manifest"`
	Instructions string          `json:"instructions"`
	AllowedTools []string        `json:"allowed_tools,omitempty"`
	RootDir      string          `json:"root_dir,omitempty"`
	Scripts      []SkillResource `json:"scripts,omitempty"`
	References   []SkillResource `json:"references,omitempty"`
	Assets       []SkillResource `json:"assets,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

// SkillResource represents a script, reference, asset, or generated artifact.
type SkillResource struct {
	Kind        string         `json:"kind,omitempty"`
	Name        string         `json:"name,omitempty"`
	Path        string         `json:"path,omitempty"`
	MIMEType    string         `json:"mime_type,omitempty"`
	Text        string         `json:"text,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
