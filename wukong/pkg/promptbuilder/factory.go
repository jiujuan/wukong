package promptbuilder

import (
	"fmt"
	"strings"
	"sync"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type Factory struct {
	mu               sync.RWMutex
	presets          map[string]ScenePreset
	newContextEngine func() *wkcontext.Engine
	newPromptEngine  func() *prompt.Engine
}

func NewFactory(opts ...FactoryOption) *Factory {
	f := &Factory{
		presets:          make(map[string]ScenePreset),
		newContextEngine: wkcontext.New,
		newPromptEngine:  prompt.NewDefaultEngine,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *Factory) RegisterPreset(preset ScenePreset) {
	if f == nil || preset == nil {
		return
	}
	name := strings.TrimSpace(preset.Name())
	if name == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presets[name] = preset
}

func (f *Factory) GetPreset(name string) (ScenePreset, bool) {
	if f == nil {
		return nil, false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	preset, ok := f.presets[strings.TrimSpace(name)]
	return preset, ok
}

func (f *Factory) ForScene(scene string) (*Builder, error) {
	if f == nil {
		return nil, ErrBuilderNil
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, ErrSceneEmpty
	}
	preset, ok := f.GetPreset(scene)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPresetNotFound, scene)
	}
	builder := NewWithOptions(
		WithContextEngine(f.newContextEngineOrDefault()),
		WithPromptEngine(f.newPromptEngineOrDefault()),
	)
	if err := preset.Apply(builder); err != nil {
		return nil, err
	}
	return builder, nil
}

func (f *Factory) newContextEngineOrDefault() *wkcontext.Engine {
	if f != nil && f.newContextEngine != nil {
		if engine := f.newContextEngine(); engine != nil {
			return engine
		}
	}
	return wkcontext.New()
}

func (f *Factory) newPromptEngineOrDefault() *prompt.Engine {
	if f != nil && f.newPromptEngine != nil {
		if engine := f.newPromptEngine(); engine != nil {
			return engine
		}
	}
	return prompt.NewDefaultEngine()
}

func (f *Factory) MustForScene(scene string) *Builder {
	builder, err := f.ForScene(scene)
	if err != nil {
		panic(err)
	}
	return builder
}
