package skillruntime

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryResolveUsesLowerPriorityForDuplicateSkill(t *testing.T) {
	ctx := context.Background()
	lowPriority := &fakeRuntime{
		name: "builtin",
		manifests: []SkillManifest{
			{Name: "search", Description: "builtin search", Runtime: "builtin"},
		},
	}
	highPriority := &fakeRuntime{
		name: "agentspec",
		manifests: []SkillManifest{
			{Name: "search", Description: "custom search", Runtime: "agentspec"},
		},
	}
	registry := NewRegistry()

	if err := registry.Register(highPriority, 20); err != nil {
		t.Fatalf("Register(highPriority) error = %v", err)
	}
	if err := registry.Register(lowPriority, 10); err != nil {
		t.Fatalf("Register(lowPriority) error = %v", err)
	}
	if _, err := registry.List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	got, ok := registry.Resolve("search")
	if !ok {
		t.Fatal("Resolve(search) returned false")
	}
	if got.Name() != lowPriority.Name() {
		t.Fatalf("Resolve(search) runtime = %q, want %q", got.Name(), lowPriority.Name())
	}
}

func TestRegistryResolveMissingReturnsFalse(t *testing.T) {
	registry := NewRegistry()

	got, ok := registry.Resolve("missing")
	if ok {
		t.Fatalf("Resolve(missing) = (%v, true), want false", got)
	}
}

func TestRegistryListMergesRuntimeManifests(t *testing.T) {
	ctx := context.Background()
	first := &fakeRuntime{
		name: "builtin",
		manifests: []SkillManifest{
			{Name: "read", Description: "read files"},
		},
	}
	second := &fakeRuntime{
		name: "agentspec",
		manifests: []SkillManifest{
			{Name: "paper", Description: "read papers"},
		},
	}
	registry := NewRegistry()
	_ = registry.Register(first, 10)
	_ = registry.Register(second, 20)

	manifests, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("List() returned %d manifests, want 2: %#v", len(manifests), manifests)
	}
	if manifests[0].Name != "read" || manifests[0].Runtime != first.Name() {
		t.Fatalf("first manifest = %#v, want read from builtin", manifests[0])
	}
	if manifests[1].Name != "paper" || manifests[1].Runtime != second.Name() {
		t.Fatalf("second manifest = %#v, want paper from agentspec", manifests[1])
	}
}

func TestRegistryListRefreshesDirtyDiscoverCache(t *testing.T) {
	ctx := context.Background()
	runtime := &fakeRuntime{
		name: "dynamic",
		manifests: []SkillManifest{
			{Name: "old", Description: "old skill"},
		},
	}
	registry := NewRegistry()
	_ = registry.Register(runtime, 10)

	if _, err := registry.List(ctx); err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	if got, ok := registry.Resolve("old"); !ok || got.Name() != runtime.Name() {
		t.Fatalf("Resolve(old) = (%v, %t), want dynamic runtime", got, ok)
	}

	runtime.manifests = []SkillManifest{
		{Name: "new", Description: "new skill"},
	}
	manifests, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("cached List() error = %v", err)
	}
	if manifests[0].Name != "old" {
		t.Fatalf("cached List() manifest = %#v, want old before Refresh", manifests[0])
	}

	if err := registry.Refresh(ctx); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	manifests, err = registry.List(ctx)
	if err != nil {
		t.Fatalf("List() after Refresh error = %v", err)
	}
	if len(manifests) != 1 || manifests[0].Name != "new" {
		t.Fatalf("List() after Refresh = %#v, want only new manifest", manifests)
	}
	if got, ok := registry.Resolve("new"); !ok || got.Name() != runtime.Name() {
		t.Fatalf("Resolve(new) = (%v, %t), want dynamic runtime", got, ok)
	}
}

func TestRegistryMatchMergesCandidatesByScoreThenPriority(t *testing.T) {
	ctx := context.Background()
	builtin := &fakeRuntime{
		name: "builtin",
		candidates: []SkillCandidate{
			{Manifest: SkillManifest{Name: "builtin-low"}, Score: 0.4},
			{Manifest: SkillManifest{Name: "builtin-tie"}, Score: 0.8},
		},
	}
	agentspec := &fakeRuntime{
		name: "agentspec",
		candidates: []SkillCandidate{
			{Manifest: SkillManifest{Name: "agent-high"}, Score: 0.9},
			{Manifest: SkillManifest{Name: "agent-tie"}, Score: 0.8},
		},
	}
	registry := NewRegistry()
	_ = registry.Register(agentspec, 20)
	_ = registry.Register(builtin, 10)

	candidates, err := registry.Match(ctx, SkillMatchRequest{Action: "search"})
	if err != nil {
		t.Fatalf("Match() error = %v", err)
	}
	names := candidateNames(candidates)
	want := []string{"agent-high", "builtin-tie", "agent-tie", "builtin-low"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("Match() names = %#v, want %#v", names, want)
		}
	}
	if len(builtin.matchRequests) != 1 || builtin.matchRequests[0].Action != "search" {
		t.Fatalf("builtin match requests = %#v, want one search request", builtin.matchRequests)
	}
}

func TestRegistryReturnsRuntimeErrorsWithContext(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	discoverErr := errors.New("boom")
	_ = registry.Register(&fakeRuntime{name: "broken", discoverErr: discoverErr}, 10)

	_, err := registry.List(ctx)
	if err == nil {
		t.Fatal("List() error = nil, want discover error")
	}
	if !errors.Is(err, discoverErr) {
		t.Fatalf("List() error = %v, want wrapped discover error", err)
	}
}

type fakeRuntime struct {
	name          string
	manifests     []SkillManifest
	candidates    []SkillCandidate
	discoverErr   error
	matchErr      error
	matchRequests []SkillMatchRequest
}

func (r *fakeRuntime) Name() string                    { return r.name }
func (r *fakeRuntime) Start(ctx context.Context) error { return ctx.Err() }
func (r *fakeRuntime) Stop(ctx context.Context) error  { return ctx.Err() }

func (r *fakeRuntime) Discover(ctx context.Context) ([]SkillManifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.discoverErr != nil {
		return nil, r.discoverErr
	}
	out := make([]SkillManifest, len(r.manifests))
	copy(out, r.manifests)
	return out, nil
}

func (r *fakeRuntime) Get(ctx context.Context, name string) (*SkillSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SkillSpec{
		Manifest: SkillManifest{Name: name, Runtime: r.name},
	}, nil
}

func (r *fakeRuntime) Match(ctx context.Context, req SkillMatchRequest) ([]SkillCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.matchRequests = append(r.matchRequests, req)
	if r.matchErr != nil {
		return nil, r.matchErr
	}
	out := make([]SkillCandidate, len(r.candidates))
	copy(out, r.candidates)
	return out, nil
}

func (r *fakeRuntime) Prepare(ctx context.Context, activation SkillActivation, runtimeCtx RuntimeContext) (*PreparedSkill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &PreparedSkill{Activation: activation}, nil
}

func (r *fakeRuntime) Execute(ctx context.Context, prepared *PreparedSkill, input SkillInput) (*SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SkillOutput{Status: "completed"}, nil
}

func candidateNames(candidates []SkillCandidate) []string {
	out := make([]string, len(candidates))
	for i, candidate := range candidates {
		out[i] = candidate.Manifest.Name
	}
	return out
}
