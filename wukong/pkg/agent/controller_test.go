package agent

import (
	"encoding/json"
	"testing"
)

func TestLoopStateDone(t *testing.T) {
	tests := []struct {
		name  string
		state *LoopState
		want  bool
	}{
		{name: "nil", state: nil, want: true},
		{name: "pending", state: &LoopState{Status: LoopStatusPending}, want: false},
		{name: "running", state: &LoopState{Status: LoopStatusRunning}, want: false},
		{name: "paused", state: &LoopState{Status: LoopStatusPaused}, want: false},
		{name: "completed", state: &LoopState{Status: LoopStatusCompleted}, want: true},
		{name: "failed", state: &LoopState{Status: LoopStatusFailed}, want: true},
		{name: "stopped", state: &LoopState{Status: LoopStatusStopped}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.Done(); got != tt.want {
				t.Fatalf("Done() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestLoopDecisionJSONRoundTrip(t *testing.T) {
	original := LoopDecision{
		Continue: true,
		Retry:    true,
		Revise:   true,
		Reason:   "revise after tool error",
		Patch: map[string]any{
			"query": "updated",
		},
		WaitFor: &HumanRequest{
			RequestID: "human-1",
			RunID:     "run-1",
			Type:      "approve_action",
			Prompt:    "Approve action?",
			Options: []HumanOption{
				{ID: "approve", Label: "Approve", Value: "yes"},
			},
		},
		Metadata: map[string]any{
			"phase": string(LoopPhaseAct),
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded LoopDecision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !decoded.Continue || !decoded.Retry || !decoded.Revise {
		t.Fatalf("decoded decision flags = %#v", decoded)
	}
	if decoded.WaitFor == nil || decoded.WaitFor.RequestID != "human-1" || decoded.WaitFor.RunID != "run-1" {
		t.Fatalf("WaitFor = %#v, want human request", decoded.WaitFor)
	}
	if decoded.Patch["query"] != "updated" {
		t.Fatalf("Patch = %#v, want query patch", decoded.Patch)
	}
}

func TestHumanRequestContainsRunAndRequestID(t *testing.T) {
	request := HumanRequest{
		RequestID: "human-1",
		RunID:     "run-1",
		Type:      "provide_info",
		Prompt:    "Need more context",
	}

	if request.RequestID == "" {
		t.Fatal("RequestID is empty")
	}
	if request.RunID == "" {
		t.Fatal("RunID is empty")
	}
}

func TestNewLoopStateInitializesRunScene(t *testing.T) {
	req := RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Params: map[string]any{
			"topic": "agents",
		},
	}
	profile := AgentProfile{ID: AgentID("general"), Role: AgentRoleGeneral}
	agentState := AgentState{AgentID: profile.ID, CurrentTaskID: "task-1"}
	agentCtx := AgentContext{Request: req, Agent: profile, State: agentState}

	state := NewLoopState(req, profile, agentState, agentCtx)
	state.Request.Params["topic"] = "changed"

	if state.RunID != req.RunID {
		t.Fatalf("RunID = %q, want %q", state.RunID, req.RunID)
	}
	if state.Phase != LoopPhasePerceive {
		t.Fatalf("Phase = %q, want %q", state.Phase, LoopPhasePerceive)
	}
	if state.Status != LoopStatusPending {
		t.Fatalf("Status = %q, want %q", state.Status, LoopStatusPending)
	}
	if req.Params["topic"] != "agents" {
		t.Fatalf("NewLoopState should clone request maps, original Params = %#v", req.Params)
	}
}
