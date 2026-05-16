package internaltest

import (
	"context"

	"github.com/jiujuan/wukong/pkg/agent"
)

var _ agent.Runtime = (*Runtime)(nil)
var _ agent.AgentRegistry = (*Registry)(nil)
var _ agent.AgentRouter = (*Router)(nil)
var _ agent.LoopFactory = (*LoopFactory)(nil)
var _ agent.Loop = (*Loop)(nil)

// Runtime is a test double for code that depends on agent.Runtime.
type Runtime struct {
	Started     bool
	Profiles    []agent.AgentProfile
	RunRequests []agent.RunRequest
	ResumeCalls []ResumeCall

	StartErr     error
	StopErr      error
	RegisterErr  error
	RunResult    *agent.RunResult
	RunErr       error
	ResumeResult *agent.RunResult
	ResumeErr    error
}

// ResumeCall records one Resume invocation.
type ResumeCall struct {
	RunID string
	Input agent.HumanInput
}

// Start records the runtime as started.
func (r *Runtime) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.StartErr != nil {
		return r.StartErr
	}
	r.Started = true
	return nil
}

// Stop records the runtime as stopped.
func (r *Runtime) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.StopErr != nil {
		return r.StopErr
	}
	r.Started = false
	return nil
}

// RegisterAgent records a registered profile.
func (r *Runtime) RegisterAgent(profile agent.AgentProfile) error {
	if r.RegisterErr != nil {
		return r.RegisterErr
	}
	r.Profiles = append(r.Profiles, profile)
	return nil
}

// Run records a cloned request and returns the configured result.
func (r *Runtime) Run(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.RunRequests = append(r.RunRequests, req.Clone())
	return r.RunResult, r.RunErr
}

// Resume records a resume request and returns the configured result.
func (r *Runtime) Resume(ctx context.Context, runID string, input agent.HumanInput) (*agent.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.ResumeCalls = append(r.ResumeCalls, ResumeCall{
		RunID: runID,
		Input: cloneHumanInput(input),
	})
	return r.ResumeResult, r.ResumeErr
}

// Registry is a lightweight in-memory registry test double.
type Registry struct {
	Profiles    []agent.AgentProfile
	RegisterErr error
}

// Register records or replaces a profile by id.
func (r *Registry) Register(profile agent.AgentProfile) error {
	if r.RegisterErr != nil {
		return r.RegisterErr
	}
	for i, existing := range r.Profiles {
		if existing.ID == profile.ID {
			r.Profiles[i] = profile
			return nil
		}
	}
	r.Profiles = append(r.Profiles, profile)
	return nil
}

// Get returns a profile by id.
func (r *Registry) Get(id agent.AgentID) (agent.AgentProfile, bool) {
	for _, profile := range r.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return agent.AgentProfile{}, false
}

// List returns registered profiles in insertion order.
func (r *Registry) List() []agent.AgentProfile {
	out := make([]agent.AgentProfile, len(r.Profiles))
	copy(out, r.Profiles)
	return out
}

// Router is a test double for agent.AgentRouter.
type Router struct {
	Profile  agent.AgentProfile
	Err      error
	Requests []agent.RunRequest
}

// Route records a cloned request and returns the configured profile.
func (r *Router) Route(ctx context.Context, req agent.RunRequest) (agent.AgentProfile, error) {
	if err := ctx.Err(); err != nil {
		return agent.AgentProfile{}, err
	}
	r.Requests = append(r.Requests, req.Clone())
	return r.Profile, r.Err
}

// LoopFactory is a test double for agent.LoopFactory.
type LoopFactory struct {
	Loop     agent.Loop
	Profiles []agent.AgentProfile
}

// NewLoop records the profile and returns the configured loop.
func (f *LoopFactory) NewLoop(profile agent.AgentProfile) agent.Loop {
	f.Profiles = append(f.Profiles, profile)
	return f.Loop
}

// Loop is a test double for agent.Loop.
type Loop struct {
	Result   *agent.RunResult
	Err      error
	Requests []agent.RunRequest
}

// Run records a cloned request and returns the configured result.
func (l *Loop) Run(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.Requests = append(l.Requests, req.Clone())
	return l.Result, l.Err
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneHumanInput(in agent.HumanInput) agent.HumanInput {
	out := in
	out.Patch = cloneMap(in.Patch)
	out.Metadata = cloneMap(in.Metadata)
	return out
}
