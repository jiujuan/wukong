package promptbuilder

import (
	"context"
	"reflect"
	"testing"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type factoryTestAssembler struct{}

func (factoryTestAssembler) BuildPromptInput(req BuildRequest, bundle *wkcontext.ContextBundle) prompt.RenderInput {
	return prompt.RenderInput{
		Variables: req.Variables,
	}
}

func (factoryTestAssembler) Assemble(req BuildRequest, bundle *wkcontext.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	return append([]llm.Message{{Role: "system", Content: "factory"}}, promptMessages...), nil
}

func TestFactoryForScene(t *testing.T) {
	factory := NewFactory(
		WithContextEngineFactory(wkcontext.New),
		WithPromptEngineFactory(prompt.New),
	)
	factory.RegisterPreset(FuncPreset{
		SceneName: "chat",
		Setup: func(b *Builder) error {
			if err := b.RegisterPromptTemplate(&prompt.Template{
				Key:      "chat.template",
				Messages: []prompt.MessageTemplate{{Role: "user", Content: "hello {{name}}"}},
			}); err != nil {
				return err
			}
			b.BindSceneTemplate("chat", "chat.template")
			b.RegisterAssembler("chat", factoryTestAssembler{})
			return nil
		},
	})

	builder, err := factory.ForScene("chat")
	if err != nil {
		t.Fatalf("ForScene() error = %v", err)
	}

	result, err := builder.BuildMessages(context.Background(), BuildRequest{
		Scene: "chat",
		Variables: map[string]any{
			"name": "wukong",
		},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	if result.TemplateKey != "chat.template" || result.Scene != "chat" {
		t.Fatalf("unexpected result meta: %#v", result)
	}
	want := []llm.Message{
		{Role: "system", Content: "factory"},
		{Role: "user", Content: "hello wukong"},
	}
	if !reflect.DeepEqual(result.Messages, want) {
		t.Fatalf("Messages = %#v, want %#v", result.Messages, want)
	}
}

func TestFactoryMissingPreset(t *testing.T) {
	factory := NewFactory()
	if _, err := factory.ForScene("missing"); err == nil {
		t.Fatal("expected missing preset error")
	}
}
