package context

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"
)

type fakeSkillSpecLoader struct {
	spec *SkillSpec
	err  error
}

func (l *fakeSkillSpecLoader) LoadSkillSpec(ctx stdctx.Context, skillName string) (*SkillSpec, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.spec, nil
}

func TestTaskStateSourceLoad(t *testing.T) {
	source := NewTaskStateSource()
	blocks, err := source.Load(stdctx.Background(), BuildRequest{
		Scene:     "worker",
		UserID:    "user-1",
		SessionID: "session-1",
		TaskID:    "task-1",
		SkillName: "web_search",
		Query:     "golang",
		Variables: map[string]any{
			"sub_task_id":     "sub-1",
			"action":          "web_search",
			"task_status":     "RUNNING",
			"params_json":     `{"query":"golang"}`,
			"depends_on_json": `["sub-0"]`,
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].Name != BlockTaskStateText {
		t.Fatalf("unexpected block name: %q", blocks[0].Name)
	}
	if !strings.Contains(blocks[0].Content, "task_id: task-1") || !strings.Contains(blocks[0].Content, "action: web_search") {
		t.Fatalf("unexpected block content: %q", blocks[0].Content)
	}
}

func TestSkillSpecSourceLoad(t *testing.T) {
	source := NewSkillSpecSource(&fakeSkillSpecLoader{
		spec: &SkillSpec{
			SkillName:      "web_search",
			Description:    "search on the web",
			Version:        "v1",
			Enabled:        true,
			Tools:          []string{"web_search", "llm_chat"},
			MemoryType:     "working",
			MemoryWindow:   10,
			MemoryCompress: true,
			Params: []SkillParam{
				{Name: "query", Type: "string", Required: true},
			},
		},
	})
	blocks, err := source.Load(stdctx.Background(), BuildRequest{
		SkillName: "web_search",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if !strings.Contains(blocks[0].Content, "tools: web_search, llm_chat") {
		t.Fatalf("unexpected skill spec content: %q", blocks[0].Content)
	}
}

func TestSkillSpecSourceFallbackAndError(t *testing.T) {
	source := NewSkillSpecSource(&fakeSkillSpecLoader{err: errors.New("boom")})
	blocks, err := source.Load(stdctx.Background(), BuildRequest{SkillName: "chat"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1", len(blocks))
	}
	if blocks[0].Content != "skill_name: chat" {
		t.Fatalf("unexpected fallback content: %q", blocks[0].Content)
	}
}
