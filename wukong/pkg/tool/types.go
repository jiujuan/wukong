package tool

import (
	"context"

	tooltools "github.com/jiujuan/wukong/pkg/tool/tools"
)

type ParamSchema = tooltools.ParamSchema
type ToolInfo = tooltools.ToolInfo

type Tool interface {
	Name() string
	Description() string
	ParameterSchema() []ParamSchema
	Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}

type MemoryStore = tooltools.MemoryStore
type InMemoryStore = tooltools.InMemoryStore

var NewInMemoryStore = tooltools.NewInMemoryStore
