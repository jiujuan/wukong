package skills

import "context"

type Param struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	DefaultVal string `json:"default_val,omitempty"`
}

type MemoryConfig struct {
	MemoryType     string `json:"memory_type"`
	WindowSize     int    `json:"window_size"`
	CompressSwitch bool   `json:"compress_switch"`
	RAGCollection  string `json:"rag_collection,omitempty"`
	ExpireTime     string `json:"expire_time,omitempty"`
}

type Skill struct {
	SkillName   string       `json:"skill_name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Enabled     bool         `json:"enabled"`
	Params      []Param      `json:"params"`
	Tools       []string     `json:"tools"`
	Execute     string       `json:"execute,omitempty"`
	Template    string       `json:"template,omitempty"`
	Memory      MemoryConfig `json:"memory"`
	SourcePath  string       `json:"source_path,omitempty"`
}

type MetaStore interface {
	BatchUpsertSkills(ctx context.Context, items []*Skill) error
}
