package promptbuilder

import (
	stdctx "context"
	"fmt"
	"reflect"
	"strings"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type Builder struct {
	contextEngine *wkcontext.Engine
	promptEngine  *prompt.Engine
	registry      *Registry
}

func New(contextEngine *wkcontext.Engine, promptEngine *prompt.Engine) *Builder {
	return NewWithOptions(
		WithContextEngine(contextEngine),
		WithPromptEngine(promptEngine),
	)
}

func NewWithOptions(opts ...Option) *Builder {
	builder := &Builder{
		registry: NewRegistry(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(builder)
		}
	}
	if builder.registry == nil {
		builder.registry = NewRegistry()
	}
	return builder
}

func (b *Builder) RegisterContextSource(source wkcontext.Source) error {
	if b == nil || b.contextEngine == nil {
		return ErrContextEngineNil
	}
	return b.contextEngine.RegisterSource(source)
}

func (b *Builder) RegisterContextPolicy(policy wkcontext.Policy) error {
	if b == nil || b.contextEngine == nil {
		return ErrContextEngineNil
	}
	return b.contextEngine.RegisterPolicy(policy)
}

func (b *Builder) RegisterScene(scene wkcontext.SceneConfig) error {
	if b == nil || b.contextEngine == nil {
		return ErrContextEngineNil
	}
	if existing, ok := b.contextEngine.GetScene(scene.Name); ok {
		if reflect.DeepEqual(existing, scene) {
			return nil
		}
		return fmt.Errorf("scene %q already registered", strings.TrimSpace(scene.Name))
	}
	return b.contextEngine.RegisterScene(scene)
}

func (b *Builder) RegisterPromptTemplate(t *prompt.Template) error {
	if b == nil || b.promptEngine == nil {
		return ErrPromptEngineNil
	}
	return b.promptEngine.Register(t)
}

func (b *Builder) BindSceneTemplate(scene string, templateKey string) {
	if b == nil || b.registry == nil {
		return
	}
	b.registry.BindSceneTemplate(scene, templateKey)
}

func (b *Builder) ResolveTemplate(scene string) (string, bool) {
	if b == nil || b.registry == nil {
		return "", false
	}
	return b.registry.ResolveTemplate(scene)
}

func (b *Builder) RegisterAssembler(scene string, assembler SceneAssembler) {
	if b == nil || b.registry == nil {
		return
	}
	b.registry.RegisterAssembler(scene, assembler)
}

func (b *Builder) BuildMessages(ctx stdctx.Context, req BuildRequest) (*BuildResult, error) {
	if b == nil {
		return nil, ErrBuilderNil
	}
	if b.promptEngine == nil {
		return nil, ErrPromptEngineNil
	}

	scene := strings.TrimSpace(req.Scene)
	if scene == "" {
		scene = strings.TrimSpace(req.Context.Scene)
	}
	if scene == "" {
		return nil, ErrSceneEmpty
	}

	templateKey := strings.TrimSpace(req.TemplateKey)
	if templateKey == "" {
		resolved, ok := b.ResolveTemplate(scene)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, scene)
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

	meta := map[string]any{}
	for k, v := range req.Meta {
		meta[k] = v
	}
	if bundle != nil && bundle.Meta != nil {
		for k, v := range bundle.Meta {
			meta[k] = v
		}
	}

	return &BuildResult{
		Scene:          scene,
		TemplateKey:    templateKey,
		Messages:       append([]llm.Message(nil), messages...),
		ContextBundle:  bundle,
		PromptMessages: append([]llm.Message(nil), promptMessages...),
		Meta:           meta,
	}, nil
}

func (b *Builder) getAssembler(scene string) SceneAssembler {
	if b == nil || b.registry == nil {
		return nil
	}
	return b.registry.GetAssembler(scene)
}
