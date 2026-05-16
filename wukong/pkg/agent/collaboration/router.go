package collaboration

import (
	"context"
	"fmt"

	"github.com/jiujuan/wukong/pkg/agent"
)

// AgentRouter selects an agent profile for runs and handoffs.
type AgentRouter interface {
	Route(ctx context.Context, req agent.RunRequest) (agent.AgentProfile, error)
	RouteHandoff(ctx context.Context, from agent.AgentProfile, step agent.PlanStep) (agent.AgentProfile, error)
}

// DefaultAgentRouter routes explicit ids first, then capability matches.
type DefaultAgentRouter struct {
	registry AgentRegistry
}

// NewDefaultAgentRouter creates a router backed by a registry.
func NewDefaultAgentRouter(registry AgentRegistry) *DefaultAgentRouter {
	return &DefaultAgentRouter{registry: registry}
}

// Route chooses a profile for a run request.
func (r *DefaultAgentRouter) Route(ctx context.Context, req agent.RunRequest) (agent.AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return agent.AgentProfile{}, err
	}
	if r == nil || r.registry == nil {
		return agent.AgentProfile{}, agent.ErrAgentNotFound
	}

	if req.AgentID != "" {
		if profile, ok := r.registry.Get(req.AgentID); ok {
			return profile, nil
		}
		return agent.AgentProfile{}, fmt.Errorf("%w: %s", agent.ErrAgentNotFound, req.AgentID)
	}

	matches, err := r.registry.Find(ctx, CapabilityQuery{
		Action:    req.Action,
		SkillName: req.SkillName,
		Goal:      req.Goal,
	})
	if err != nil {
		return agent.AgentProfile{}, err
	}
	if len(matches) > 0 {
		return bestProfile(matches), nil
	}

	profiles := r.registry.List()
	if len(profiles) > 0 {
		return profiles[0], nil
	}
	return agent.AgentProfile{}, agent.ErrAgentNotFound
}

// RouteHandoff chooses a profile for a delegated plan step.
func (r *DefaultAgentRouter) RouteHandoff(ctx context.Context, from agent.AgentProfile, step agent.PlanStep) (agent.AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return agent.AgentProfile{}, err
	}
	if r == nil || r.registry == nil {
		return agent.AgentProfile{}, agent.ErrAgentNotFound
	}

	if step.AgentID != "" {
		if profile, ok := r.registry.Get(step.AgentID); ok {
			return profile, nil
		}
		return agent.AgentProfile{}, fmt.Errorf("%w: %s", agent.ErrAgentNotFound, step.AgentID)
	}

	matches, err := r.registry.Find(ctx, CapabilityQuery{
		Action:    step.Action,
		SkillName: step.SkillName,
		Tools:     []string{step.Target},
		Goal:      step.Expected,
	})
	if err != nil {
		return agent.AgentProfile{}, err
	}

	filtered := make([]agent.AgentProfile, 0, len(matches))
	for _, profile := range matches {
		if profile.ID != from.ID {
			filtered = append(filtered, profile)
		}
	}
	if len(filtered) > 0 {
		return bestProfile(filtered), nil
	}
	if len(matches) > 0 {
		return bestProfile(matches), nil
	}
	return agent.AgentProfile{}, agent.ErrAgentNotFound
}

func bestProfile(profiles []agent.AgentProfile) agent.AgentProfile {
	best := profiles[0]
	bestPriority := profilePriority(best)
	for _, profile := range profiles[1:] {
		priority := profilePriority(profile)
		if priority > bestPriority {
			best = profile
			bestPriority = priority
		}
	}
	return best
}

func profilePriority(profile agent.AgentProfile) int {
	best := 0
	for _, capability := range profile.Capabilities {
		if capability.Priority > best {
			best = capability.Priority
		}
	}
	return best
}
