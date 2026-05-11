package messagebuilder

import (
	stdctx "context"
	"errors"
	"reflect"
	"testing"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type staticSource struct {
	name   string
	blocks []wkcontext.ContextBlock
	err    error
}

func (s *staticSource) Name() string { return s.name }

func (s *staticSource) Load(ctx stdctx.Context, req wkcontext.BuildRequest) ([]wkcontext.ContextBlock, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]wkcontext.ContextBlock(nil), s.blocks...), nil
}

type testAssembler struct{}

func (a testAssembler) BuildPromptInput(req BuildRequest, bundle *wkcontext.ContextBundle) prompt.RenderInput {
	var blockText string
	if bundle != nil {
		blockText = bundle.Named["history"]
	}
	return prompt.RenderInput{
		Variables: map[string]any{
			"question": req.Variables["question"],
			"history":  blockText,
		},
	}
}

func (a testAssembler) Assemble(req BuildRequest, bundle *wkcontext.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	return append([]llm.Message{{Role: "system", Content: "assembled"}}, promptMessages...), nil
}

func TestBuilderBindAndResolveTemplate(t *testing.T) {
	builder := New(nil, prompt.NewDefaultEngine())
	builder.BindSceneTemplate("chat", "chat.session.default")
	got, ok := builder.ResolveTemplate("chat")
	if !ok || got != "chat.session.default" {
		t.Fatalf("ResolveTemplate() = %q, %v", got, ok)
	}
}

func TestBuilderBuildMessagesWithAssembler(t *testing.T) {
	contextEngine := wkcontext.New()
	if err := contextEngine.RegisterSource(&staticSource{
		name: "history",
		blocks: []wkcontext.ContextBlock{{
			Name:    "history",
			Content: "previous turn",
		}},
	}); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := contextEngine.RegisterScene(wkcontext.SceneConfig{Name: "chat", Sources: []string{"history"}}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	promptEngine := prompt.New()
	if err := promptEngine.Register(&prompt.Template{
		Key: "chat.template",
		Messages: []prompt.MessageTemplate{
			{Role: "user", Content: "q={{question}}\nh={{history}}"},
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	builder := New(contextEngine, promptEngine)
	builder.BindSceneTemplate("chat", "chat.template")
	builder.RegisterAssembler("chat", testAssembler{})

	result, err := builder.BuildMessages(stdctx.Background(), BuildRequest{
		Scene: "chat",
		Context: wkcontext.BuildRequest{
			UserID: "u1",
		},
		Variables: map[string]any{
			"question": "what changed",
		},
	})
	if err != nil {
		t.Fatalf("BuildMessages() error = %v", err)
	}
	if result.ContextBundle == nil || result.ContextBundle.Named["history"] != "previous turn" {
		t.Fatalf("unexpected bundle: %#v", result.ContextBundle)
	}
	if len(result.PromptMessages) != 1 || result.PromptMessages[0].Content != "q=what changed\nh=previous turn" {
		t.Fatalf("unexpected prompt messages: %#v", result.PromptMessages)
	}
	want := []llm.Message{
		{Role: "system", Content: "assembled"},
		{Role: "user", Content: "q=what changed\nh=previous turn"},
	}
	if !reflect.DeepEqual(result.Messages, want) {
		t.Fatalf("Messages = %#v, want %#v", result.Messages, want)
	}
}

func TestBuilderBuildMessagesErrorPaths(t *testing.T) {
	builder := New(nil, prompt.NewDefaultEngine())
	if _, err := builder.BuildMessages(stdctx.Background(), BuildRequest{}); err == nil {
		t.Fatal("expected empty scene error")
	}

	builder.BindSceneTemplate("chat", "chat.session.default")
	if _, err := builder.BuildMessages(stdctx.Background(), BuildRequest{Scene: "chat"}); err == nil {
		t.Fatal("expected render error because variables are missing")
	}

	contextEngine := wkcontext.New()
	if err := contextEngine.RegisterSource(&staticSource{name: "broken", err: errors.New("boom")}); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := contextEngine.RegisterScene(wkcontext.SceneConfig{Name: "broken", Sources: []string{"broken"}}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	promptEngine := prompt.New()
	if err := promptEngine.Register(&prompt.Template{Key: "ok", Messages: []prompt.MessageTemplate{{Role: "user", Content: "x"}}}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	builder = New(contextEngine, promptEngine)
	builder.BindSceneTemplate("broken", "ok")
	if _, err := builder.BuildMessages(stdctx.Background(), BuildRequest{Scene: "broken"}); err == nil {
		t.Fatal("expected source error")
	}
}
