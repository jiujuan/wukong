package agent

import "context"

// LoopHook observes Agent Loop phase lifecycle events.
type LoopHook interface {
	BeforePhase(ctx context.Context, state *LoopState, phase LoopPhase) error
	AfterPhase(ctx context.Context, state *LoopState, phase LoopPhase) error
}
