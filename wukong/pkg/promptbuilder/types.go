package promptbuilder

import (
	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
)

type BuildRequest struct {
	Scene       string
	TemplateKey string
	Context     wkcontext.BuildRequest
	Variables   map[string]any
}

type BuildResult struct {
	Messages       []llm.Message
	ContextBundle  *wkcontext.ContextBundle
	PromptMessages []llm.Message
}
