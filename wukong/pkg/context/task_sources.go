package context

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	BlockTaskStateText = "task_state_text"
	BlockSkillSpecText = "skill_spec_text"
)

type SkillParam struct {
	Name       string
	Type       string
	Required   bool
	DefaultVal string
}

type SkillSpec struct {
	SkillName      string
	Description    string
	Version        string
	Enabled        bool
	SourceType     string
	RootDir        string
	Runtime        string
	Entry          string
	Tools          []string
	Params         []SkillParam
	MemoryType     string
	MemoryWindow   int
	MemoryCompress bool
	References     []string
	Assets         []string
	Metadata       map[string]any
}

type SkillSpecLoader interface {
	LoadSkillSpec(ctx stdctx.Context, skillName string) (*SkillSpec, error)
}

type TaskStateSource struct{}

func NewTaskStateSource() *TaskStateSource {
	return &TaskStateSource{}
}

func (s *TaskStateSource) Name() string { return "task_state" }

func (s *TaskStateSource) Load(_ stdctx.Context, req BuildRequest) ([]ContextBlock, error) {
	text := formatTaskState(req)
	if text == "" {
		return nil, nil
	}
	return []ContextBlock{{
		Name:     BlockTaskStateText,
		Type:     "task_state",
		Source:   s.Name(),
		Content:  text,
		Priority: 90,
	}}, nil
}

type SkillSpecSource struct {
	loader SkillSpecLoader
}

func NewSkillSpecSource(loader SkillSpecLoader) *SkillSpecSource {
	return &SkillSpecSource{loader: loader}
}

func (s *SkillSpecSource) Name() string { return "skill_spec" }

func (s *SkillSpecSource) Load(ctx stdctx.Context, req BuildRequest) ([]ContextBlock, error) {
	skillName := strings.TrimSpace(req.SkillName)
	if skillName == "" {
		skillName = strings.TrimSpace(requestVarString(req, "skill_name"))
	}
	if skillName == "" {
		return nil, nil
	}

	var text string
	if s != nil && s.loader != nil {
		spec, err := s.loader.LoadSkillSpec(ctx, skillName)
		if err == nil {
			text = formatSkillSpec(spec)
		}
	}
	if text == "" {
		text = fmt.Sprintf("skill_name: %s", skillName)
	}

	return []ContextBlock{{
		Name:     BlockSkillSpecText,
		Type:     "skill_spec",
		Source:   s.Name(),
		Content:  text,
		Priority: 80,
	}}, nil
}

func formatTaskState(req BuildRequest) string {
	lines := make([]string, 0, 12)
	appendLine := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, value))
	}

	appendLine("scene", req.Scene)
	appendLine("task_id", req.TaskID)
	appendLine("session_id", req.SessionID)
	appendLine("user_id", req.UserID)
	appendLine("skill_name", req.SkillName)
	appendLine("query", req.Query)
	appendLine("sub_task_id", requestVarString(req, "sub_task_id"))
	appendLine("action", requestVarString(req, "action"))
	appendLine("task_status", requestVarString(req, "task_status"))
	appendLine("subtask_status", requestVarString(req, "subtask_status"))
	appendLine("plan_thought", requestVarString(req, "plan_thought"))

	appendJSONLine := func(label, key string) {
		if text := requestJSONLike(req, key); text != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", label, text))
		}
	}
	appendJSONLine("params", "params_json")
	appendJSONLine("task_params", "task_params_json")
	appendJSONLine("depends_on", "depends_on_json")
	appendJSONLine("task_result", "task_result_json")
	appendJSONLine("subtask_result", "subtask_result_json")

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatSkillSpec(spec *SkillSpec) string {
	if spec == nil {
		return ""
	}
	lines := make([]string, 0, 12)
	appendLine := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, value))
	}

	appendLine("skill_name", spec.SkillName)
	appendLine("description", spec.Description)
	appendLine("version", spec.Version)
	appendLine("source_type", spec.SourceType)
	appendLine("root_dir", spec.RootDir)
	appendLine("runtime", spec.Runtime)
	appendLine("entry", spec.Entry)
	lines = append(lines, fmt.Sprintf("enabled: %t", spec.Enabled))
	if len(spec.Tools) > 0 {
		lines = append(lines, fmt.Sprintf("tools: %s", strings.Join(spec.Tools, ", ")))
	}
	if len(spec.References) > 0 {
		lines = append(lines, fmt.Sprintf("references: %s", strings.Join(spec.References, ", ")))
	}
	if len(spec.Assets) > 0 {
		lines = append(lines, fmt.Sprintf("assets: %s", strings.Join(spec.Assets, ", ")))
	}
	if spec.MemoryType != "" || spec.MemoryWindow > 0 {
		lines = append(lines, fmt.Sprintf("memory: type=%s window=%d compress=%t", spec.MemoryType, spec.MemoryWindow, spec.MemoryCompress))
	}
	if len(spec.Params) > 0 {
		paramLines := make([]string, 0, len(spec.Params))
		for _, item := range spec.Params {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			paramLine := fmt.Sprintf("%s(type=%s required=%t", name, strings.TrimSpace(item.Type), item.Required)
			if strings.TrimSpace(item.DefaultVal) != "" {
				paramLine += " default=" + strings.TrimSpace(item.DefaultVal)
			}
			paramLine += ")"
			paramLines = append(paramLines, paramLine)
		}
		if len(paramLines) > 0 {
			lines = append(lines, "params: "+strings.Join(paramLines, "; "))
		}
	}
	if len(spec.Metadata) > 0 {
		raw, err := json.Marshal(spec.Metadata)
		if err == nil && strings.TrimSpace(string(raw)) != "" {
			lines = append(lines, "metadata: "+string(raw))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func requestJSONLike(req BuildRequest, key string) string {
	if req.Variables == nil {
		return ""
	}
	raw, ok := req.Variables[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.RawMessage:
		return strings.TrimSpace(string(v))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return strings.TrimSpace(string(data))
	}
}

func requestVarString(req BuildRequest, key string) string {
	if req.Variables == nil {
		return ""
	}
	raw, ok := req.Variables[key]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}
