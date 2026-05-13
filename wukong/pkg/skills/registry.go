package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	wkstr "github.com/jiujuan/wukong/pkg/str"
)

type Registry struct {
	rootDir      string
	roots        []SkillRoot
	pollInterval time.Duration
	execTimeout  time.Duration
	logger       *slog.Logger
	store        MetaStore
	adapters     []Adapter

	mu      sync.RWMutex
	skills  map[string]*Skill
	started bool
	cancel  context.CancelFunc
}

func New(opts ...Option) *Registry {
	r := &Registry{
		rootDir:      "skills",
		pollInterval: 3 * time.Second,
		execTimeout:  60 * time.Second,
		logger:       slog.Default(),
		skills:       make(map[string]*Skill),
	}
	for _, opt := range opts {
		opt(r)
	}
	if len(r.roots) == 0 {
		r.roots = defaultSkillRoots(r.rootDir)
	}
	if len(r.adapters) == 0 {
		r.adapters = defaultAdapters()
	}
	for _, item := range defaultBuiltins() {
		r.skills[item.SkillName] = cloneSkill(item)
	}
	return r
}

func (r *Registry) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.started = true
	r.mu.Unlock()

	if err := r.reload(runCtx); err != nil && r.logger != nil {
		r.logger.Warn("[skills] initial reload failed", "error", err)
	}

	go r.watch(runCtx)
	return nil
}

func (r *Registry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		return
	}
	r.started = false
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}

func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Skill, 0, len(r.skills))
	for _, item := range r.skills {
		result = append(result, cloneSkill(item))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SkillName < result[j].SkillName
	})
	return result
}

func (r *Registry) Get(skillName string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.skills[normalizeSkillName(skillName)]
	if !ok {
		return nil, false
	}
	return cloneSkill(item), true
}

func (r *Registry) CanUseTool(skillName, tool string) bool {
	item, ok := r.Get(skillName)
	if !ok {
		return false
	}
	target := wkstr.TrimLower(tool)
	for _, allowed := range item.Tools {
		if wkstr.TrimLower(allowed) == target {
			return true
		}
	}
	return false
}

func (r *Registry) RegisterAdapter(adapter Adapter) {
	if adapter == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = append(r.adapters, adapter)
}

func (r *Registry) parseSkillWithAdapters(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	adapters := append([]Adapter(nil), r.adapters...)
	r.mu.RUnlock()
	if len(adapters) == 0 {
		adapters = defaultAdapters()
	}
	for _, adapter := range adapters {
		if adapter == nil || !adapter.Match(path, content) {
			continue
		}
		item, err := adapter.Parse(path)
		if err != nil {
			return nil, err
		}
		return item, nil
	}
	return nil, fmt.Errorf("no skill adapter matched: %s", path)
}
