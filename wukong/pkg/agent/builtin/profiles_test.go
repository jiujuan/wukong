package builtin

import (
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestDefaultProfilesHaveUniqueIDs(t *testing.T) {
	seen := make(map[agent.AgentID]bool)
	for _, profile := range DefaultProfiles() {
		if profile.ID == "" {
			t.Fatal("DefaultProfiles contains empty profile id")
		}
		if seen[profile.ID] {
			t.Fatalf("duplicate profile id %q", profile.ID)
		}
		seen[profile.ID] = true
	}
}

func TestGeneralAgentHasFallbackCapability(t *testing.T) {
	if GeneralAgent.ID == "" {
		t.Fatal("GeneralAgent ID is empty")
	}
	for _, capability := range GeneralAgent.Capabilities {
		if capability.Name == "fallback" {
			return
		}
	}
	t.Fatalf("GeneralAgent capabilities = %#v, want fallback capability", GeneralAgent.Capabilities)
}

func TestToolAgentHasToolCapability(t *testing.T) {
	if ToolAgent.ID == "" {
		t.Fatal("ToolAgent ID is empty")
	}
	if !contains(ToolAgent.Tools, "file_read") || !contains(ToolAgent.Tools, "code_exec") {
		t.Fatalf("ToolAgent tools = %#v, want file_read and code_exec", ToolAgent.Tools)
	}
	for _, capability := range ToolAgent.Capabilities {
		if capability.Name == "tool_execution" && contains(capability.Tools, "file_write") {
			return
		}
	}
	t.Fatalf("ToolAgent capabilities = %#v, want tool_execution capability", ToolAgent.Capabilities)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
