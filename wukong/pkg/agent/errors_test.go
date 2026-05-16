package agent_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/agent/internaltest"
)

func TestAgentSentinelErrorsSupportErrorsIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "agent not found", err: agent.ErrAgentNotFound},
		{name: "runtime not started", err: agent.ErrRuntimeNotStarted},
		{name: "loop paused", err: agent.ErrLoopPaused},
		{name: "checkpoint not found", err: agent.ErrCheckpointNotFound},
		{name: "action executor not found", err: agent.ErrActionExecutorNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("wrapped: %w", tt.err)
			if !errors.Is(wrapped, tt.err) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", wrapped, tt.err)
			}
		})
	}
}

func TestInternalTestFakesCanDriveRuntime(t *testing.T) {
	profile := agent.AgentProfile{
		ID:   agent.AgentID("fake-agent"),
		Name: "Fake Agent",
		Role: agent.AgentRoleGeneral,
	}
	loop := &internaltest.Loop{
		Result: &agent.RunResult{
			RunID:   "run-1",
			AgentID: profile.ID,
			TaskID:  "task-1",
			Status:  "completed",
			Output:  "ok",
		},
	}
	router := &internaltest.Router{Profile: profile}
	factory := &internaltest.LoopFactory{Loop: loop}
	runtime := agent.NewRuntime(
		agent.WithAgentRouter(router),
		agent.WithLoopFactory(factory),
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	req := agent.RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Params: map[string]any{
			"topic": "fake",
		},
	}
	result, err := runtime.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Output != "ok" {
		t.Fatalf("Run() Output = %q, want ok", result.Output)
	}
	if len(router.Requests) != 1 || router.Requests[0].RunID != req.RunID {
		t.Fatalf("router Requests = %#v, want one cloned request", router.Requests)
	}
	if len(factory.Profiles) != 1 || factory.Profiles[0].ID != profile.ID {
		t.Fatalf("factory Profiles = %#v, want profile %q", factory.Profiles, profile.ID)
	}
	if len(loop.Requests) != 1 || loop.Requests[0].Params["topic"] != "fake" {
		t.Fatalf("loop Requests = %#v, want one cloned request with params", loop.Requests)
	}
}
