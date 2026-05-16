package collaboration

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestRouterExplicitAgentIDWins(t *testing.T) {
	registry := NewInMemoryAgentRegistry()
	mustRegister(t, registry, agent.AgentProfile{
		ID: agent.AgentID("action-agent"),
		Capabilities: []agent.Capability{
			{Name: "search", Actions: []string{"search"}},
		},
	})
	mustRegister(t, registry, agent.AgentProfile{
		ID: agent.AgentID("explicit-agent"),
	})

	router := NewDefaultAgentRouter(registry)
	profile, err := router.Route(context.Background(), agent.RunRequest{
		AgentID: agent.AgentID("explicit-agent"),
		Action:  "search",
	})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if profile.ID != agent.AgentID("explicit-agent") {
		t.Fatalf("Route() ID = %q, want explicit-agent", profile.ID)
	}
}

func TestRouterMatchesActionCapability(t *testing.T) {
	registry := NewInMemoryAgentRegistry()
	mustRegister(t, registry, agent.AgentProfile{ID: agent.AgentID("general")})
	mustRegister(t, registry, agent.AgentProfile{
		ID: agent.AgentID("tool-agent"),
		Capabilities: []agent.Capability{
			{Name: "web", Actions: []string{"search"}},
		},
	})

	router := NewDefaultAgentRouter(registry)
	profile, err := router.Route(context.Background(), agent.RunRequest{Action: "search"})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if profile.ID != agent.AgentID("tool-agent") {
		t.Fatalf("Route() ID = %q, want tool-agent", profile.ID)
	}
}

func TestRouterMatchesProfileSkill(t *testing.T) {
	registry := NewInMemoryAgentRegistry()
	mustRegister(t, registry, agent.AgentProfile{ID: agent.AgentID("general")})
	mustRegister(t, registry, agent.AgentProfile{
		ID:     agent.AgentID("skill-agent"),
		Skills: []string{"code-review"},
	})

	router := NewDefaultAgentRouter(registry)
	profile, err := router.Route(context.Background(), agent.RunRequest{SkillName: "code-review"})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if profile.ID != agent.AgentID("skill-agent") {
		t.Fatalf("Route() ID = %q, want skill-agent", profile.ID)
	}
}

func TestRegistryFindMatchesCapabilitySkill(t *testing.T) {
	registry := NewInMemoryAgentRegistry()
	mustRegister(t, registry, agent.AgentProfile{
		ID: agent.AgentID("capability-skill-agent"),
		Capabilities: []agent.Capability{
			{Name: "review", Skills: []string{"code-review"}},
		},
	})

	matches, err := registry.Find(context.Background(), CapabilityQuery{SkillName: "code-review"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if len(matches) != 1 || matches[0].ID != agent.AgentID("capability-skill-agent") {
		t.Fatalf("Find() = %#v, want capability-skill-agent", matches)
	}
}

func mustRegister(t *testing.T, registry AgentRegistry, profile agent.AgentProfile) {
	t.Helper()
	if err := registry.Register(profile); err != nil {
		t.Fatalf("Register(%q) error = %v", profile.ID, err)
	}
}
