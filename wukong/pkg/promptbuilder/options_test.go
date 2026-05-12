package promptbuilder

import (
	"testing"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
)

func TestBuilderOptions(t *testing.T) {
	contextEngine := wkcontext.New()
	promptEngine := prompt.New()
	registry := NewRegistry()

	builder := NewWithOptions(
		WithContextEngine(contextEngine),
		WithPromptEngine(promptEngine),
		WithRegistry(registry),
	)

	if builder.contextEngine != contextEngine {
		t.Fatalf("context engine not assigned")
	}
	if builder.promptEngine != promptEngine {
		t.Fatalf("prompt engine not assigned")
	}
	if builder.registry != registry {
		t.Fatalf("registry not assigned")
	}
}

func TestFactoryOptions(t *testing.T) {
	contextFactoryCalled := false
	promptFactoryCalled := false

	factory := NewFactory(
		WithContextEngineFactory(func() *wkcontext.Engine {
			contextFactoryCalled = true
			return wkcontext.New()
		}),
		WithPromptEngineFactory(func() *prompt.Engine {
			promptFactoryCalled = true
			return prompt.New()
		}),
		WithPreset(FuncPreset{SceneName: "chat"}),
	)

	if _, err := factory.ForScene("chat"); err != nil {
		t.Fatalf("ForScene() failed: %v", err)
	}
	if !contextFactoryCalled {
		t.Fatal("context factory not called")
	}
	if !promptFactoryCalled {
		t.Fatal("prompt factory not called")
	}
	if _, ok := factory.GetPreset("chat"); !ok {
		t.Fatal("preset not registered")
	}
}
