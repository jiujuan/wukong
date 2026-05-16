package skillruntime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrRuntimeNil is returned when registering a nil runtime.
var ErrRuntimeNil = errors.New("skill runtime is nil")

// Registry manages multiple skill runtimes behind one lookup and matching facade.
type Registry interface {
	Register(runtime SkillRuntime, priority int) error
	Resolve(skillName string) (SkillRuntime, bool)
	Match(ctx context.Context, req SkillMatchRequest) ([]SkillCandidate, error)
	List(ctx context.Context) ([]SkillManifest, error)
	Refresh(ctx context.Context) error
}

// RuntimeRegistry is the default concurrency-safe Registry implementation.
type RuntimeRegistry struct {
	mu      sync.RWMutex
	entries []registryEntry
	cache   registryCache
	nextSeq int
	version int
}

type registryEntry struct {
	runtime  SkillRuntime
	priority int
	seq      int
}

type registryCache struct {
	dirty     bool
	manifests []SkillManifest
	byName    map[string]cachedSkill
}

type cachedSkill struct {
	runtime  SkillRuntime
	manifest SkillManifest
	priority int
	seq      int
}

// NewRegistry creates an empty skill runtime registry.
func NewRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		entries: make([]registryEntry, 0),
		cache: registryCache{
			dirty:  true,
			byName: make(map[string]cachedSkill),
		},
	}
}

// Register adds a runtime with a priority. Lower priority values win conflicts.
func (r *RuntimeRegistry) Register(runtime SkillRuntime, priority int) error {
	if runtime == nil {
		return ErrRuntimeNil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, registryEntry{
		runtime:  runtime,
		priority: priority,
		seq:      r.nextSeq,
	})
	r.nextSeq++
	r.version++
	sortRegistryEntries(r.entries)
	r.cache.dirty = true
	return nil
}

// Resolve returns the runtime selected for a skill name from the latest cache.
func (r *RuntimeRegistry) Resolve(skillName string) (SkillRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if skillName == "" || r.cache.byName == nil {
		return nil, false
	}
	item, ok := r.cache.byName[skillName]
	if !ok {
		return nil, false
	}
	return item.runtime, true
}

// Match asks each registered runtime for candidates and returns a priority-aware merged list.
func (r *RuntimeRegistry) Match(ctx context.Context, req SkillMatchRequest) ([]SkillCandidate, error) {
	if err := r.ensureFresh(ctx); err != nil {
		return nil, err
	}

	entries := r.snapshotEntries()
	out := make([]candidateWithRank, 0)
	for _, entry := range entries {
		candidates, err := entry.runtime.Match(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("match runtime %q: %w", entry.runtime.Name(), err)
		}
		for _, candidate := range candidates {
			out = append(out, candidateWithRank{
				candidate: candidate,
				priority:  entry.priority,
				seq:       entry.seq,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].candidate.Score != out[j].candidate.Score {
			return out[i].candidate.Score > out[j].candidate.Score
		}
		if out[i].priority != out[j].priority {
			return out[i].priority < out[j].priority
		}
		return out[i].seq < out[j].seq
	})

	candidates := make([]SkillCandidate, len(out))
	for i, item := range out {
		candidates[i] = item.candidate
	}
	return candidates, nil
}

// List returns all discovered manifests and refreshes stale runtime discovery results.
func (r *RuntimeRegistry) List(ctx context.Context) ([]SkillManifest, error) {
	if err := r.ensureFresh(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]SkillManifest, len(r.cache.manifests))
	copy(out, r.cache.manifests)
	return out, nil
}

// Refresh reloads manifests from every runtime and rebuilds the skill lookup cache.
func (r *RuntimeRegistry) Refresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.refresh(ctx)
}

func (r *RuntimeRegistry) ensureFresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	if !r.cache.dirty {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	if !r.cache.dirty {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	return r.refresh(ctx)
}

func (r *RuntimeRegistry) refresh(ctx context.Context) error {
	r.mu.RLock()
	entries := make([]registryEntry, len(r.entries))
	copy(entries, r.entries)
	version := r.version
	r.mu.RUnlock()

	manifests := make([]SkillManifest, 0)
	byName := make(map[string]cachedSkill)
	for _, entry := range entries {
		discovered, err := entry.runtime.Discover(ctx)
		if err != nil {
			return fmt.Errorf("discover runtime %q: %w", entry.runtime.Name(), err)
		}
		for _, manifest := range discovered {
			if manifest.Runtime == "" {
				manifest.Runtime = entry.runtime.Name()
			}
			manifests = append(manifests, manifest)
			if manifest.Name == "" {
				continue
			}
			next := cachedSkill{
				runtime:  entry.runtime,
				manifest: manifest,
				priority: entry.priority,
				seq:      entry.seq,
			}
			if current, exists := byName[manifest.Name]; !exists || lessCachedSkill(next, current) {
				byName[manifest.Name] = next
			}
		}
	}

	r.mu.Lock()
	r.cache = registryCache{
		dirty:     r.version != version,
		manifests: manifests,
		byName:    byName,
	}
	r.mu.Unlock()
	return nil
}

type candidateWithRank struct {
	candidate SkillCandidate
	priority  int
	seq       int
}

func (r *RuntimeRegistry) snapshotEntries() []registryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]registryEntry, len(r.entries))
	copy(entries, r.entries)
	return entries
}

func sortRegistryEntries(entries []registryEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].priority != entries[j].priority {
			return entries[i].priority < entries[j].priority
		}
		return entries[i].seq < entries[j].seq
	})
}

func lessCachedSkill(left, right cachedSkill) bool {
	if left.priority != right.priority {
		return left.priority < right.priority
	}
	return left.seq < right.seq
}
