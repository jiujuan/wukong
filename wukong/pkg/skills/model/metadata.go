package model

// SkillSource describes where a skill comes from and how it is packaged.
type SkillSource struct {
	Type         string `json:"type,omitempty"`
	Name         string `json:"name,omitempty"`
	RootDir      string `json:"root_dir,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
	SourcePath   string `json:"source_path,omitempty"`
	PackageName  string `json:"package_name,omitempty"`
	Version      string `json:"version,omitempty"`
}

// SkillResource represents a skill-associated resource such as a reference or asset.
type SkillResource struct {
	Kind     string         `json:"kind,omitempty"`
	Name     string         `json:"name,omitempty"`
	Path     string         `json:"path,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Text     string         `json:"text,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SkillRuntimeSpec describes how a skill is executed.
type SkillRuntimeSpec struct {
	Runtime    string   `json:"runtime,omitempty"`
	Entry      string   `json:"entry,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	WorkDir    string   `json:"work_dir,omitempty"`
	AllowShell bool     `json:"allow_shell,omitempty"`
}
