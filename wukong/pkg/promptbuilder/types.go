package promptbuilder

import (
	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type BuildRequest struct {
	Scene       string
	TemplateKey string
	Context     wkcontext.BuildRequest
	Variables   map[string]any
	Meta        map[string]any
}

type BuildResult struct {
	Scene          string
	TemplateKey    string
	Messages       []llm.Message
	ContextBundle  *wkcontext.ContextBundle
	PromptMessages []llm.Message
	Meta           map[string]any
}

type SceneAssembler interface {
	BuildPromptInput(req BuildRequest, bundle *wkcontext.ContextBundle) prompt.RenderInput
	Assemble(req BuildRequest, bundle *wkcontext.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error)
}
