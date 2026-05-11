package promptbuilder

import (
	stdctx "context"
	"fmt"
	"strings"
	"sync"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type SceneAssembler interface {
	BuildPromptInput(req BuildRequest, bundle *wkcontext.ContextBundle) prompt.RenderInput
	Assemble(req BuildRequest, bundle *wkcontext.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error)
}

type Builder struct {
	contextEngine   *wkcontext.Engine
	promptEngine    *prompt.Engine
	mu              sync.RWMutex
	sceneTemplates  map[string]string
	sceneAssemblers map[string]SceneAssembler
}

func New(contextEngine *wkcontext.Engine, promptEngine *prompt.Engine) *Builder {
	return &Builder{
		contextEngine:   contextEngine,
		promptEngine:    promptEngine,
		sceneTemplates:  make(map[string]string),
		sceneAssemblers: make(map[string]SceneAssembler),
	}
}

func (b *Builder) BindSceneTemplate(scene string, templateKey string) {
	if b == nil {
		return
	}
	scene = strings.TrimSpace(scene)
	templateKey = strings.TrimSpace(templateKey)
	if scene == "" || templateKey == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sceneTemplates[scene] = templateKey
}

func (b *Builder) ResolveTemplate(scene string) (string, bool) {
	if b == nil {
		return "", false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	key, ok := b.sceneTemplates[strings.TrimSpace(scene)]
	return key, ok
}

func (b *Builder) RegisterAssembler(scene string, assembler SceneAssembler) {
	if b == nil || assembler == nil {
		return
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sceneAssemblers[scene] = assembler
}

func (b *Builder) BuildMessages(ctx stdctx.Context, req BuildRequest) (*BuildResult, error) {
	if b == nil {
		return nil, fmt.Errorf("prompt builder is nil")
	}
	if b.promptEngine == nil {
		return nil, fmt.Errorf("prompt engine is nil")
	}

	scene := strings.TrimSpace(req.Scene)
	if scene == "" {
		scene = strings.TrimSpace(req.Context.Scene)
	}
	if scene == "" {
		return nil, fmt.Errorf("build request scene is empty")
	}

	templateKey := strings.TrimSpace(req.TemplateKey)
	if templateKey == "" {
		resolved, ok := b.ResolveTemplate(scene)
		if !ok {
			return nil, fmt.Errorf("template binding for scene %q not found", scene)
		}
		templateKey = resolved
	}

	contextReq := req.Context
	contextReq.Scene = scene

	var bundle *wkcontext.ContextBundle
	if b.contextEngine != nil {
		var err error
		bundle, err = b.contextEngine.Build(ctx, contextReq)
		if err != nil {
			return nil, err
		}
	}

	assembler := b.getAssembler(scene)
	renderInput := prompt.RenderInput{Variables: req.Variables}
	if assembler != nil {
		renderInput = assembler.BuildPromptInput(req, bundle)
		if renderInput.Variables == nil {
			renderInput.Variables = req.Variables
		}
	} else if bundle != nil {
		renderInput.Context = bundle.Named
	}

	promptMessages, err := b.promptEngine.Render(templateKey, renderInput)
	if err != nil {
		return nil, err
	}

	messages := append([]llm.Message(nil), promptMessages...)
	if assembler != nil {
		messages, err = assembler.Assemble(req, bundle, promptMessages)
		if err != nil {
			return nil, err
		}
	}

	return &BuildResult{
		Messages:       append([]llm.Message(nil), messages...),
		ContextBundle:  bundle,
		PromptMessages: append([]llm.Message(nil), promptMessages...),
	}, nil
}

func (b *Builder) getAssembler(scene string) SceneAssembler {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sceneAssemblers[strings.TrimSpace(scene)]
}
