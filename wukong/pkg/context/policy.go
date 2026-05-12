package context

import stdctx "context"

type Policy interface {
	Name() string
	Apply(ctx stdctx.Context, blocks []ContextBlock, req BuildRequest) ([]ContextBlock, error)
}
