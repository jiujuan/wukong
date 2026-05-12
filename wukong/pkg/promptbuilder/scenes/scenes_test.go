package scenes

import (
	"testing"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
)

func TestPresetName(t *testing.T) {
	preset := NewChatPreset("chat.template", nil)
	if preset.Name() != ChatSceneName {
		t.Fatalf("Name() = %q, want %q", preset.Name(), ChatSceneName)
	}
}

func TestPresetApply(t *testing.T) {
	builder := promptbuilder.New(wkcontext.New(), prompt.NewDefaultEngine())
	preset := NewPlannerPreset("planner.template", func(b *promptbuilder.Builder) error {
		return b.RegisterPromptTemplate(&prompt.Template{
			Key:      "planner.template",
			Messages: []prompt.MessageTemplate{{Role: "user", Content: "ok"}},
		})
	})
	if err := preset.Apply(builder); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got, ok := builder.ResolveTemplate(PlannerSceneName); !ok || got != "planner.template" {
		t.Fatalf("ResolveTemplate() = %q, %v", got, ok)
	}
}
