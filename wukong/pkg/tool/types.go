package tool

import "context"

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}
