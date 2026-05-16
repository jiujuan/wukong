package skillruntime

import (
	"context"
	"reflect"
	"testing"
)

func TestDefaultToolPolicyMapperMapsCommonAllowedTools(t *testing.T) {
	mapper := NewDefaultToolPolicyMapper()

	policy, err := mapper.MapAllowedTools(context.Background(), []string{
		"Read",
		"Write",
		"WebSearch",
		"llm_chat",
		"mcp__server_tool",
	})
	if err != nil {
		t.Fatalf("MapAllowedTools() error = %v", err)
	}

	want := []string{ToolFileRead, ToolFileWrite, ToolWebSearch, "llm_chat", "mcp__server_tool"}
	if !reflect.DeepEqual(policy.AllowedTools, want) {
		t.Fatalf("AllowedTools = %#v, want %#v", policy.AllowedTools, want)
	}
	if !reflect.DeepEqual(policy.Raw, []string{"Read", "Write", "WebSearch", "llm_chat", "mcp__server_tool"}) {
		t.Fatalf("Raw = %#v, want original declarations", policy.Raw)
	}
}

func TestDefaultToolPolicyMapperPreservesBashAllowlist(t *testing.T) {
	mapper := NewDefaultToolPolicyMapper()

	policy, err := mapper.MapAllowedTools(context.Background(), []string{
		"Bash(git:*)",
		"Bash(npm run test)",
		"Bash()",
	})
	if err != nil {
		t.Fatalf("MapAllowedTools() error = %v", err)
	}

	if !reflect.DeepEqual(policy.AllowedTools, []string{ToolCodeExec}) {
		t.Fatalf("AllowedTools = %#v, want only code_exec once", policy.AllowedTools)
	}
	got, ok := policy.Metadata["bash_allowlist"].([]string)
	if !ok {
		t.Fatalf("bash_allowlist metadata = %#v, want []string", policy.Metadata["bash_allowlist"])
	}
	want := []string{"git:*", "npm run test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bash_allowlist = %#v, want %#v", got, want)
	}
}

func TestDefaultToolPolicyMapperKeepsUnknownRawButNotAllowed(t *testing.T) {
	mapper := NewDefaultToolPolicyMapper()

	policy, err := mapper.MapAllowedTools(context.Background(), []string{
		"UnknownTool",
		"Read",
	})
	if err != nil {
		t.Fatalf("MapAllowedTools() error = %v", err)
	}

	if !reflect.DeepEqual(policy.AllowedTools, []string{ToolFileRead}) {
		t.Fatalf("AllowedTools = %#v, want only file_read", policy.AllowedTools)
	}
	if !reflect.DeepEqual(policy.Raw, []string{"UnknownTool", "Read"}) {
		t.Fatalf("Raw = %#v, want original non-empty declarations", policy.Raw)
	}
	got, ok := policy.Metadata["unknown_tools"].([]string)
	if !ok {
		t.Fatalf("unknown_tools metadata = %#v, want []string", policy.Metadata["unknown_tools"])
	}
	if !reflect.DeepEqual(got, []string{"UnknownTool"}) {
		t.Fatalf("unknown_tools = %#v, want UnknownTool", got)
	}
}

func TestDefaultToolPolicyMapperCanDisableUnknownNativePassthrough(t *testing.T) {
	mapper := DefaultToolPolicyMapper{PassUnknownNative: false}

	policy, err := mapper.MapAllowedTools(context.Background(), []string{"llm_chat"})
	if err != nil {
		t.Fatalf("MapAllowedTools() error = %v", err)
	}

	if len(policy.AllowedTools) != 0 {
		t.Fatalf("AllowedTools = %#v, want empty when passthrough disabled", policy.AllowedTools)
	}
	if !reflect.DeepEqual(policy.Metadata["unknown_tools"], []string{"llm_chat"}) {
		t.Fatalf("unknown_tools = %#v, want llm_chat", policy.Metadata["unknown_tools"])
	}
}

func TestDefaultToolPolicyMapperRespectsContextCancellation(t *testing.T) {
	mapper := NewDefaultToolPolicyMapper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := mapper.MapAllowedTools(ctx, []string{"Read"})
	if err == nil {
		t.Fatal("MapAllowedTools() error = nil, want context cancellation")
	}
}
