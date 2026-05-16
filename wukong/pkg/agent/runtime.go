package agent

import (
	"context"
	"fmt"
	"sync"
)

// Runtime is the public facade for Agent Runtime lifecycle and execution.
type Runtime interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterAgent(profile AgentProfile) error
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	Resume(ctx context.Context, runID string, input map[string]any) (*RunResult, error)
}

// AgentRegistry stores available agent profiles.
type AgentRegistry interface {
	Register(profile AgentProfile) error
	Get(id AgentID) (AgentProfile, bool)
	List() []AgentProfile
}

// AgentRouter selects one agent profile for a run request.
type AgentRouter interface {
	Route(ctx context.Context, req RunRequest) (AgentProfile, error)
}

// Loop executes a request for one routed agent profile.
type Loop interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

// LoopFactory creates an Agent Loop for one profile.
type LoopFactory interface {
	NewLoop(profile AgentProfile) Loop
}

// AgentRuntime is the default Runtime implementation.
type AgentRuntime struct {
	mu          sync.RWMutex
	started     bool
	registry    AgentRegistry
	router      AgentRouter
	loopFactory LoopFactory
}

// NewRuntime creates an Agent Runtime with noop dependencies that can be replaced by options.
func NewRuntime(options ...Option) *AgentRuntime {
	cfg := defaultRuntimeConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	registry := cfg.registry
	if registry == nil {
		registry = NewInMemoryAgentRegistry()
	}

	router := cfg.router
	if router == nil {
		router = NewDefaultAgentRouter(registry)
	}

	loopFactory := cfg.loopFactory
	if loopFactory == nil {
		loopFactory = NoopLoopFactory{}
	}

	return &AgentRuntime{
		registry:    registry,
		router:      router,
		loopFactory: loopFactory,
	}
}

// Start marks the runtime as ready to accept runs.
func (r *AgentRuntime) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = true
	return nil
}

// Stop marks the runtime as stopped.
func (r *AgentRuntime) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = false
	return nil
}

// RegisterAgent adds or replaces an agent profile in the runtime registry.
func (r *AgentRuntime) RegisterAgent(profile AgentProfile) error {
	if profile.ID == "" {
		return fmt.Errorf("register agent: empty id")
	}
	return r.registry.Register(profile)
}

// Run routes the request to an agent and executes it through a loop.
func (r *AgentRuntime) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.isStarted() {
		return nil, ErrRuntimeNotStarted
	}

	profile, err := r.router.Route(ctx, req)
	if err != nil {
		return nil, err
	}

	loop := r.loopFactory.NewLoop(profile)
	if loop == nil {
		return nil, fmt.Errorf("agent loop factory returned nil loop for agent %q", profile.ID)
	}
	return loop.Run(ctx, req.Clone())
}

// Resume is a placeholder for checkpoint-backed resume support added in later milestones.
func (r *AgentRuntime) Resume(ctx context.Context, runID string, input map[string]any) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.isStarted() {
		return nil, ErrRuntimeNotStarted
	}
	return nil, fmt.Errorf("resume run %q: not implemented", runID)
}

// Registry exposes the current registry for adapters and tests.
func (r *AgentRuntime) Registry() AgentRegistry {
	return r.registry
}

func (r *AgentRuntime) isStarted() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.started
}

// InMemoryAgentRegistry is a small concurrency-safe profile registry.
type InMemoryAgentRegistry struct {
	mu       sync.RWMutex
	profiles map[AgentID]AgentProfile
	order    []AgentID
}

// NewInMemoryAgentRegistry creates an empty agent registry.
func NewInMemoryAgentRegistry() *InMemoryAgentRegistry {
	return &InMemoryAgentRegistry{
		profiles: make(map[AgentID]AgentProfile),
		order:    make([]AgentID, 0),
	}
}

// Register adds or replaces a profile.
func (r *InMemoryAgentRegistry) Register(profile AgentProfile) error {
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

// Get returns one profile by id.
func (r *InMemoryAgentRegistry) Get(id AgentID) (AgentProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, ok := r.profiles[id]
	return profile, ok
}

// List returns profiles in registration order.
func (r *InMemoryAgentRegistry) List() []AgentProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]AgentProfile, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.profiles[id])
	}
	return out
}

// DefaultAgentRouter routes explicit AgentID first, then capability matches, then first registered profile.
type DefaultAgentRouter struct {
	registry AgentRegistry
}

// NewDefaultAgentRouter creates a router backed by a registry.
func NewDefaultAgentRouter(registry AgentRegistry) *DefaultAgentRouter {
	return &DefaultAgentRouter{registry: registry}
}

// Route chooses a profile for the request.
func (r *DefaultAgentRouter) Route(ctx context.Context, req RunRequest) (AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return AgentProfile{}, err
	}

	if req.AgentID != "" {
		if profile, ok := r.registry.Get(req.AgentID); ok {
			return profile, nil
		}
		return AgentProfile{}, fmt.Errorf("%w: %s", ErrAgentNotFound, req.AgentID)
	}

	profiles := r.registry.List()
	for _, profile := range profiles {
		if profileMatchesRequest(profile, req) {
			return profile, nil
		}
	}
	if len(profiles) > 0 {
		return profiles[0], nil
	}
	return AgentProfile{}, ErrAgentNotFound
}

func profileMatchesRequest(profile AgentProfile, req RunRequest) bool {
	if req.SkillName != "" && containsString(profile.Skills, req.SkillName) {
		return true
	}
	if req.Action != "" {
		for _, capability := range profile.Capabilities {
			if containsString(capability.Actions, req.Action) {
				return true
			}
		}
	}
	if req.SkillName != "" {
		for _, capability := range profile.Capabilities {
			if containsString(capability.Skills, req.SkillName) {
				return true
			}
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

// NoopLoopFactory creates noop loops for early runtime assembly tests.
type NoopLoopFactory struct{}

// NewLoop creates a noop loop for one profile.
func (NoopLoopFactory) NewLoop(profile AgentProfile) Loop {
	return NoopLoop{profile: profile}
}

// NoopLoop returns a minimal successful result without external side effects.
type NoopLoop struct {
	profile AgentProfile
}

// Run returns a placeholder result for early runtime wiring.
func (l NoopLoop) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &RunResult{
		RunID:     req.RunID,
		AgentID:   l.profile.ID,
		TaskID:    req.TaskID,
		SubTaskID: req.SubTaskID,
		Status:    "completed",
		Output:    "",
	}, nil
}
