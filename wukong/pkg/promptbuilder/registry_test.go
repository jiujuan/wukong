package promptbuilder

import (
	"testing"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type registryTestAssembler struct{}

func (registryTestAssembler) BuildPromptInput(req BuildRequest, bundle *wkcontext.ContextBundle) prompt.RenderInput {
	return prompt.RenderInput{}
}

func (registryTestAssembler) Assemble(req BuildRequest, bundle *wkcontext.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	return promptMessages, nil
}

func TestRegistryBindResolveTemplate(t *testing.T) {
	registry := NewRegistry()
	registry.BindSceneTemplate(" chat ", " chat.template ")
	if got, ok := registry.ResolveTemplate("chat"); !ok || got != "chat.template" {
		t.Fatalf("ResolveTemplate() = %q, %v", got, ok)
	}
}

func TestRegistryRegisterAssembler(t *testing.T) {
	registry := NewRegistry()
	assembler := registryTestAssembler{}
	registry.RegisterAssembler("chat", assembler)
	got := registry.GetAssembler("chat")
	if got == nil {
		t.Fatal("GetAssembler() returned nil")
	}
}

func TestRegistryNilSafe(t *testing.T) {
	var registry *Registry
	registry.BindSceneTemplate("chat", "x")
	registry.RegisterAssembler("chat", registryTestAssembler{})
	if got, ok := registry.ResolveTemplate("chat"); ok || got != "" {
		t.Fatalf("nil registry ResolveTemplate() = %q, %v", got, ok)
	}
	if got := registry.GetAssembler("chat"); got != nil {
		t.Fatalf("nil registry GetAssembler() = %#v", got)
	}
}
