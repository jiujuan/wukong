package collaboration

import (
	"context"
	"fmt"
	"sync"

	"github.com/jiujuan/wukong/pkg/agent"
)

// AgentRegistry stores available agent profiles for collaboration routing.
type AgentRegistry interface {
	Register(profile agent.AgentProfile) error
	Get(id agent.AgentID) (agent.AgentProfile, bool)
	List() []agent.AgentProfile
	Find(ctx context.Context, query CapabilityQuery) ([]agent.AgentProfile, error)
}

// CapabilityQuery describes the capabilities needed by a run or handoff.
type CapabilityQuery struct {
	Action    string
	SkillName string
	Tools     []string
	Goal      string
	Tags      []string
}

// InMemoryAgentRegistry is a concurrency-safe profile registry.
type InMemoryAgentRegistry struct {
	mu       sync.RWMutex
	profiles map[agent.AgentID]agent.AgentProfile
	order    []agent.AgentID
}

// NewInMemoryAgentRegistry creates an empty registry.
func NewInMemoryAgentRegistry() *InMemoryAgentRegistry {
	return &InMemoryAgentRegistry{
		profiles: make(map[agent.AgentID]agent.AgentProfile),
		order:    make([]agent.AgentID, 0),
	}
}

// Register adds or replaces a profile.
func (r *InMemoryAgentRegistry) Register(profile agent.AgentProfile) error {
	if profile.ID == "" {
		return fmt.Errorf("register agent: empty id")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[profile.ID]; !exists {
		r.order = append(r.order, profile.ID)
	}
	r.profiles[profile.ID] = profile
	return nil
}

// Get returns a profile by id.
func (r *InMemoryAgentRegistry) Get(id agent.AgentID) (agent.AgentProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, ok := r.profiles[id]
	return profile, ok
}

// List returns profiles in registration order.
func (r *InMemoryAgentRegistry) List() []agent.AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]agent.AgentProfile, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.profiles[id])
	}
	return out
}

// Find returns profiles that match the query in registration order.
func (r *InMemoryAgentRegistry) Find(ctx context.Context, query CapabilityQuery) ([]agent.AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	profiles := r.List()
	out := make([]agent.AgentProfile, 0, len(profiles))
	for _, profile := range profiles {
		if ProfileMatchesCapabilityQuery(profile, query) {
			out = append(out, profile)
		}
	}
	return out, nil
}

// ProfileMatchesCapabilityQuery reports whether a profile satisfies the query.
func ProfileMatchesCapabilityQuery(profile agent.AgentProfile, query CapabilityQuery) bool {
	if query.SkillName != "" && (containsString(profile.Skills, query.SkillName) || capabilityHasSkill(profile.Capabilities, query.SkillName)) {
		return true
	}
	if query.Action != "" && capabilityHasAction(profile.Capabilities, query.Action) {
		return true
	}
	for _, tool := range query.Tools {
		if containsString(profile.Tools, tool) {
			return true
		}
	}
	for _, tag := range query.Tags {
		if containsString(profile.Tools, tag) || containsString(profile.Skills, tag) || capabilityHasName(profile.Capabilities, tag) {
			return true
		}
	}
	return false
}

func capabilityHasAction(capabilities []agent.Capability, action string) bool {
	for _, capability := range capabilities {
		if containsString(capability.Actions, action) {
			return true
		}
	}
	return false
}

func capabilityHasSkill(capabilities []agent.Capability, skill string) bool {
	for _, capability := range capabilities {
		if containsString(capability.Skills, skill) {
			return true
		}
	}
	return false
}

func capabilityHasName(capabilities []agent.Capability, name string) bool {
	for _, capability := range capabilities {
		if capability.Name == name {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
