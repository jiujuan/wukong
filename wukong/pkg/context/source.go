package context

import stdctx "context"

type Source interface {
	Name() string
	Load(ctx stdctx.Context, req BuildRequest) ([]ContextBlock, error)
}
