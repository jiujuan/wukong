package agent

type runtimeConfig struct {
	registry    AgentRegistry
	router      AgentRouter
	loopFactory LoopFactory
	checkpoints CheckpointStore
	controller  LoopController
}

func defaultRuntimeConfig() runtimeConfig {
	return runtimeConfig{}
}

// Option configures AgentRuntime construction.
type Option func(*runtimeConfig)

// WithAgentRegistry injects a custom agent registry.
func WithAgentRegistry(registry AgentRegistry) Option {
	return func(cfg *runtimeConfig) {
		cfg.registry = registry
	}
}

// WithAgentRouter injects a custom routing strategy.
func WithAgentRouter(router AgentRouter) Option {
	return func(cfg *runtimeConfig) {
		cfg.router = router
	}
}

// WithLoopFactory injects a custom loop factory.
func WithLoopFactory(factory LoopFactory) Option {
	return func(cfg *runtimeConfig) {
		cfg.loopFactory = factory
	}
}

// WithCheckpointStore injects a checkpoint store for pause/resume support.
func WithCheckpointStore(store CheckpointStore) Option {
	return func(cfg *runtimeConfig) {
		cfg.checkpoints = store
	}
}

// WithLoopController injects a loop controller for resume decisions.
func WithLoopController(controller LoopController) Option {
	return func(cfg *runtimeConfig) {
		cfg.controller = controller
	}
}
