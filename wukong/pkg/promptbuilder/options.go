package promptbuilder

import (
	"github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type FactoryOption func(*Factory)

type Option func(*Builder)

func WithContextEngine(engine *context.Engine) Option {
	return func(b *Builder) {
		if b != nil && engine != nil {
			b.contextEngine = engine
		}
	}
}

func WithPromptEngine(engine *prompt.Engine) Option {
	return func(b *Builder) {
		if b != nil && engine != nil {
			b.promptEngine = engine
		}
	}
}

func WithRegistry(registry *Registry) Option {
	return func(b *Builder) {
		if b != nil && registry != nil {
			b.registry = registry
		}
	}
}

func WithContextEngineFactory(fn func() *context.Engine) FactoryOption {
	return func(f *Factory) {
		if f != nil && fn != nil {
			f.newContextEngine = fn
		}
	}
}

func WithPromptEngineFactory(fn func() *prompt.Engine) FactoryOption {
	return func(f *Factory) {
		if f != nil && fn != nil {
			f.newPromptEngine = fn
		}
	}
}

func WithPreset(preset ScenePreset) FactoryOption {
	return func(f *Factory) {
		if f != nil && preset != nil {
			f.RegisterPreset(preset)
		}
	}
}
