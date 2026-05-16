package skillruntime

import (
	"context"
	"strings"
)

const (
	ToolFileRead  = "file_read"
	ToolFileWrite = "file_write"
	ToolCodeExec  = "code_exec"
	ToolWebSearch = "web_search"
)

// ToolPolicy describes which tools a prepared skill is allowed to use.
type ToolPolicy struct {
	AllowedTools []string       `json:"allowed_tools,omitempty"`
	DeniedTools  []string       `json:"denied_tools,omitempty"`
	Raw          []string       `json:"raw,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ToolPolicyMapper converts external allowed-tools declarations into Wukong tool policy.
type ToolPolicyMapper interface {
	MapAllowedTools(ctx context.Context, specTools []string) (ToolPolicy, error)
}

// DefaultToolPolicyMapper maps Agent Skills Spec allowed-tools to Wukong tool names.
type DefaultToolPolicyMapper struct {
	// PassUnknownNative allows unknown tool names that look like native Wukong or MCP tools.
	PassUnknownNative bool
}

// NewDefaultToolPolicyMapper creates a conservative mapper.
func NewDefaultToolPolicyMapper() DefaultToolPolicyMapper {
	return DefaultToolPolicyMapper{PassUnknownNative: true}
}

// MapAllowedTools maps common Agent Skills Spec tool declarations to Wukong tools.
func (m DefaultToolPolicyMapper) MapAllowedTools(ctx context.Context, specTools []string) (ToolPolicy, error) {
	if err := ctx.Err(); err != nil {
		return ToolPolicy{}, err
	}

	policy := ToolPolicy{
		Raw:      compactStrings(specTools),
		Metadata: make(map[string]any),
	}
	allowed := make([]string, 0, len(specTools))
	unknown := make([]string, 0)
	bashAllowlist := make([]string, 0)

	for _, raw := range specTools {
		tool := strings.TrimSpace(raw)
		if tool == "" {
			continue
		}

		mapped, bashRule, ok := m.mapOne(tool)
		if !ok {
			unknown = append(unknown, tool)
			continue
		}
		allowed = appendUnique(allowed, mapped)
		if bashRule != "" {
			bashAllowlist = appendUnique(bashAllowlist, bashRule)
		}
	}

	policy.AllowedTools = allowed
	if len(bashAllowlist) > 0 {
		policy.Metadata["bash_allowlist"] = bashAllowlist
	}
	if len(unknown) > 0 {
		policy.Metadata["unknown_tools"] = unknown
	}
	if len(policy.Metadata) == 0 {
		policy.Metadata = nil
	}
	return policy, nil
}

func (m DefaultToolPolicyMapper) mapOne(tool string) (mapped string, bashRule string, ok bool) {
	switch tool {
	case "Read":
		return ToolFileRead, "", true
	case "Write":
		return ToolFileWrite, "", true
	case "WebSearch":
		return ToolWebSearch, "", true
	}

	if rule, ok := parseBashRule(tool); ok {
		return ToolCodeExec, rule, true
	}
	if m.PassUnknownNative && looksLikeNativeTool(tool) {
		return tool, "", true
	}
	return "", "", false
}

func parseBashRule(tool string) (string, bool) {
	if !strings.HasPrefix(tool, "Bash(") || !strings.HasSuffix(tool, ")") {
		return "", false
	}
	rule := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tool, "Bash("), ")"))
	return rule, true
}

func looksLikeNativeTool(tool string) bool {
	if strings.HasPrefix(tool, "mcp__") {
		return true
	}
	if strings.Contains(tool, "__") {
		return false
	}
	for _, r := range tool {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return false
	}
	return strings.Contains(tool, "_")
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
