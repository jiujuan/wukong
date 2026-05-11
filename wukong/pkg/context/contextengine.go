package context

import (
	stdctx "context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type BuildRequest struct {
	Scene     string
	UserID    string
	SessionID string
	TaskID    string
	SkillName string
	Query     string
	Variables map[string]any
}

type ContextBlock struct {
	Name      string
	Type      string
	Source    string
	Content   string
	Priority  int
	Tokens    int
	Timestamp int64
}

type ContextBundle struct {
	Scene  string
	Blocks []ContextBlock
	Named  map[string]string
	Text   string
	Meta   map[string]any
}

type Source interface {
	Name() string
	Load(ctx stdctx.Context, req BuildRequest) ([]ContextBlock, error)
}

type Policy interface {
	Name() string
	Apply(ctx stdctx.Context, blocks []ContextBlock, req BuildRequest) ([]ContextBlock, error)
}

type SceneConfig struct {
	Name     string
	Sources  []string
	Policies []string
	Options  map[string]any
}

type Engine struct {
	mu       sync.RWMutex
	sources  map[string]Source
	policies map[string]Policy
	scenes   map[string]SceneConfig
}

func New() *Engine {
	return &Engine{
		sources:  make(map[string]Source),
		policies: make(map[string]Policy),
		scenes:   make(map[string]SceneConfig),
	}
}

func (e *Engine) RegisterSource(source Source) error {
	if e == nil {
		return fmt.Errorf("context engine is nil")
	}
	if source == nil {
		return fmt.Errorf("source is nil")
	}
	name := strings.TrimSpace(source.Name())
	if name == "" {
		return fmt.Errorf("source name is empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.sources[name]; exists {
		return fmt.Errorf("source %q already registered", name)
	}
	e.sources[name] = source
	return nil
}

func (e *Engine) RegisterPolicy(policy Policy) error {
	if e == nil {
		return fmt.Errorf("context engine is nil")
	}
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}
	name := strings.TrimSpace(policy.Name())
	if name == "" {
		return fmt.Errorf("policy name is empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.policies[name]; exists {
		return fmt.Errorf("policy %q already registered", name)
	}
	e.policies[name] = policy
	return nil
}

func (e *Engine) RegisterScene(scene SceneConfig) error {
	if e == nil {
		return fmt.Errorf("context engine is nil")
	}
	name := strings.TrimSpace(scene.Name)
	if name == "" {
		return fmt.Errorf("scene name is empty")
	}
	scene.Name = name
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.scenes[name]; exists {
		return fmt.Errorf("scene %q already registered", name)
	}
	e.scenes[name] = cloneScene(scene)
	return nil
}

func (e *Engine) GetScene(name string) (SceneConfig, bool) {
	if e == nil {
		return SceneConfig{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	scene, ok := e.scenes[strings.TrimSpace(name)]
	if !ok {
		return SceneConfig{}, false
	}
	return cloneScene(scene), true
}

func (e *Engine) Build(ctx stdctx.Context, req BuildRequest) (*ContextBundle, error) {
	if e == nil {
		return nil, fmt.Errorf("context engine is nil")
	}
	sceneName := strings.TrimSpace(req.Scene)
	if sceneName == "" {
		return nil, fmt.Errorf("build request scene is empty")
	}
	scene, ok := e.GetScene(sceneName)
	if !ok {
		return nil, fmt.Errorf("scene %q not found", sceneName)
	}

	sources, err := e.resolveSources(scene.Sources)
	if err != nil {
		return nil, err
	}
	policies, err := e.resolvePolicies(scene.Policies)
	if err != nil {
		return nil, err
	}

	blocks := make([]ContextBlock, 0)
	for _, source := range sources {
		loaded, loadErr := source.Load(ctx, req)
		if loadErr != nil {
			return nil, fmt.Errorf("load source %q failed: %w", source.Name(), loadErr)
		}
		blocks = append(blocks, normalizeBlocks(source.Name(), loaded)...)
	}

	for _, policy := range policies {
		blocks, err = policy.Apply(ctx, blocks, req)
		if err != nil {
			return nil, fmt.Errorf("apply policy %q failed: %w", policy.Name(), err)
		}
		blocks = normalizeBlocks("", blocks)
	}

	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Priority != blocks[j].Priority {
			return blocks[i].Priority > blocks[j].Priority
		}
		if blocks[i].Timestamp != blocks[j].Timestamp {
			return blocks[i].Timestamp < blocks[j].Timestamp
		}
		if blocks[i].Name != blocks[j].Name {
			return blocks[i].Name < blocks[j].Name
		}
		return blocks[i].Source < blocks[j].Source
	})

	bundle := &ContextBundle{
		Scene:  scene.Name,
		Blocks: append([]ContextBlock(nil), blocks...),
		Named:  make(map[string]string),
		Meta: map[string]any{
			"scene":        scene.Name,
			"source_count": len(scene.Sources),
			"policy_count": len(scene.Policies),
			"block_count":  len(blocks),
		},
	}
	textParts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Name != "" {
			if _, exists := bundle.Named[block.Name]; !exists {
				bundle.Named[block.Name] = block.Content
			}
		}
		textParts = append(textParts, formatBlockText(block))
	}
	bundle.Text = strings.Join(textParts, "\n\n")
	return bundle, nil
}

func (e *Engine) resolveSources(names []string) ([]Source, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Source, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		source, ok := e.sources[name]
		if !ok {
			return nil, fmt.Errorf("source %q not registered", name)
		}
		result = append(result, source)
	}
	return result, nil
}

func (e *Engine) resolvePolicies(names []string) ([]Policy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Policy, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		policy, ok := e.policies[name]
		if !ok {
			return nil, fmt.Errorf("policy %q not registered", name)
		}
		result = append(result, policy)
	}
	return result, nil
}

func normalizeBlocks(defaultSource string, blocks []ContextBlock) []ContextBlock {
	out := make([]ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		block.Name = strings.TrimSpace(block.Name)
		block.Type = strings.TrimSpace(block.Type)
		block.Source = strings.TrimSpace(block.Source)
		block.Content = strings.TrimSpace(block.Content)
		if block.Content == "" {
			continue
		}
		if block.Source == "" {
			block.Source = defaultSource
		}
		out = append(out, block)
	}
	return out
}

func formatBlockText(block ContextBlock) string {
	if block.Name == "" {
		return block.Content
	}
	return fmt.Sprintf("[%s]\n%s", block.Name, block.Content)
}

func cloneScene(src SceneConfig) SceneConfig {
	dst := src
	if src.Sources != nil {
		dst.Sources = append([]string(nil), src.Sources...)
	}
	if src.Policies != nil {
		dst.Policies = append([]string(nil), src.Policies...)
	}
	if src.Options != nil {
		dst.Options = make(map[string]any, len(src.Options))
		for k, v := range src.Options {
			dst.Options[k] = v
		}
	}
	return dst
}
