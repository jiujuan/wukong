package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentCoreTypesJSONRoundTrip(t *testing.T) {
	completedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	original := RunResult{
		RunID:     "run-1",
		AgentID:   AgentID("agent-general"),
		TaskID:    "task-1",
		SubTaskID: "subtask-1",
		Status:    "completed",
		Output:    "done",
		Result: map[string]any{
			"ok": true,
		},
		Steps: []LoopStep{
			{
				Index:     1,
				Phase:     "act",
				Type:      "tool",
				Action:    "web_search",
				Status:    "completed",
				Output:    "result",
				StartedAt: completedAt.Add(-time.Second),
			},
		},
		Evaluation: &Evaluation{
			Success: true,
			Score:   0.98,
			Reason:  "accepted",
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Duration:         2 * time.Second,
		},
		CompletedAt: completedAt,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded RunResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.RunID != original.RunID {
		t.Fatalf("RunID = %q, want %q", decoded.RunID, original.RunID)
	}
	if decoded.AgentID != original.AgentID {
		t.Fatalf("AgentID = %q, want %q", decoded.AgentID, original.AgentID)
	}
	if decoded.Usage == nil || decoded.Usage.TotalTokens != original.Usage.TotalTokens {
		t.Fatalf("Usage = %#v, want total tokens %d", decoded.Usage, original.Usage.TotalTokens)
	}
	if len(decoded.Steps) != 1 || decoded.Steps[0].Action != original.Steps[0].Action {
		t.Fatalf("Steps = %#v, want one %q action step", decoded.Steps, original.Steps[0].Action)
	}
}

func TestAgentContextZeroValueUsable(t *testing.T) {
	var ctx AgentContext

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("Marshal(zero AgentContext) error = %v", err)
	}

	var decoded AgentContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal(zero AgentContext) error = %v", err)
	}

	cloned := decoded.Request.Clone()
	if cloned.Params != nil {
		t.Fatalf("zero RunRequest clone Params = %#v, want nil", cloned.Params)
	}
	if cloned.Context != nil {
		t.Fatalf("zero RunRequest clone Context = %#v, want nil", cloned.Context)
	}
}

func TestRunRequestCloneDoesNotShareParamsMap(t *testing.T) {
	original := RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Params: map[string]any{
			"topic": "agents",
		},
		Context: map[string]any{
			"source": "worker",
		},
	}

	cloned := original.Clone()
	cloned.Params["topic"] = "runtime"
	cloned.Params["new"] = true
	cloned.Context["source"] = "agent"

	if original.Params["topic"] != "agents" {
		t.Fatalf("original Params was mutated: %#v", original.Params)
	}
	if _, ok := original.Params["new"]; ok {
		t.Fatalf("original Params shares cloned map: %#v", original.Params)
	}
	if original.Context["source"] != "worker" {
		t.Fatalf("original Context was mutated: %#v", original.Context)
	}
}

func TestAgentProfileJSONRoundTrip(t *testing.T) {
	profile := AgentProfile{
		ID:          AgentID("researcher"),
		Name:        "Research Agent",
		Role:        AgentRoleResearch,
		Description: "Handles research tasks",
		Goal:        "Produce grounded answers",
		Capabilities: []Capability{
			{
				Name:     "research",
				Actions:  []string{"search", "summarize"},
				Skills:   []string{"paper-reader"},
				Tools:    []string{"web_search"},
				Priority: 10,
			},
		},
		Tools:  []string{"web_search"},
		Skills: []string{"paper-reader"},
		Reasoning: ReasoningConfig{
			Strategy: "direct",
			Depth:    "medium",
		},
		Memory: MemoryConfig{
			Enabled:        true,
			WorkingEnabled: true,
		},
		Reflection: ReflectConfig{
			Enabled: true,
		},
		Collaboration: CollaborationConfig{
			CanDelegate: true,
			MaxDepth:    2,
		},
		Metadata: map[string]any{
			"owner": "test",
		},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded AgentProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.ID != profile.ID || decoded.Role != profile.Role {
		t.Fatalf("decoded profile = %#v, want id %q role %q", decoded, profile.ID, profile.Role)
	}
	if len(decoded.Capabilities) != 1 || decoded.Capabilities[0].Name != "research" {
		t.Fatalf("Capabilities = %#v, want research capability", decoded.Capabilities)
	}
}
