package agent

import (
	"context"
	"errors"
	"testing"
)

func TestNewRuntimeDoesNotPanic(t *testing.T) {
	runtime := NewRuntime()
	if runtime == nil {
		t.Fatal("NewRuntime() returned nil")
	}
	if runtime.Registry() == nil {
		t.Fatal("NewRuntime() should provide a default registry")
	}
}

func TestRegisterAgentCanBeFoundInRegistry(t *testing.T) {
	runtime := NewRuntime()
	profile := AgentProfile{
		ID:   AgentID("general"),
		Name: "General Agent",
		Role: AgentRoleGeneral,
	}

	if err := runtime.RegisterAgent(profile); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}

	got, ok := runtime.Registry().Get(profile.ID)
	if !ok {
		t.Fatalf("Registry().Get(%q) returned false", profile.ID)
	}
	if got.ID != profile.ID || got.Name != profile.Name {
		t.Fatalf("registered profile = %#v, want %#v", got, profile)
	}
}

func TestRunWithoutAvailableAgentReturnsClearError(t *testing.T) {
	runtime := NewRuntime()
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err := runtime.Run(context.Background(), RunRequest{RunID: "run-1", TaskID: "task-1"})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("Run() error = %v, want ErrAgentNotFound", err)
	}
}

func TestRunBeforeStartReturnsRuntimeNotStarted(t *testing.T) {
	runtime := NewRuntime()

	_, err := runtime.Run(context.Background(), RunRequest{RunID: "run-1", TaskID: "task-1"})
	if !errors.Is(err, ErrRuntimeNotStarted) {
		t.Fatalf("Run() error = %v, want ErrRuntimeNotStarted", err)
	}
}

func TestRunRoutesToRegisteredAgent(t *testing.T) {
	runtime := NewRuntime()
	profile := AgentProfile{
		ID:   AgentID("tool-agent"),
		Name: "Tool Agent",
		Role: AgentRoleTool,
		Capabilities: []Capability{
			{
				Name:    "tooling",
				Actions: []string{"search"},
			},
		},
	}

	if err := runtime.RegisterAgent(profile); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := runtime.Run(context.Background(), RunRequest{
		RunID:  "run-1",
		TaskID: "task-1",
		Action: "search",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.AgentID != profile.ID {
		t.Fatalf("Run() AgentID = %q, want %q", result.AgentID, profile.ID)
	}
}
