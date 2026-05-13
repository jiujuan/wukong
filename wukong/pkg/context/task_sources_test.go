package context

import (
	stdctx "context"
	"encoding/json"
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
			SourceType:     "vendor",
			RootDir:        "/tmp/skills/web_search",
			Runtime:        "bash",
			Entry:          "run.sh",
			Tools:          []string{"web_search", "llm_chat"},
			MemoryType:     "working",
			MemoryWindow:   10,
			MemoryCompress: true,
			Params: []SkillParam{
				{Name: "query", Type: "string", Required: true},
			},
			References: []string{"/tmp/skills/web_search/references/guide.md"},
			Assets:     []string{"/tmp/skills/web_search/assets/cover.png"},
			Metadata: map[string]any{
				"owner": "platform",
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
	if !strings.Contains(blocks[0].Content, "source_type: vendor") || !strings.Contains(blocks[0].Content, "root_dir: /tmp/skills/web_search") {
		t.Fatalf("expected runtime/source fields in skill spec content: %q", blocks[0].Content)
	}
	if !strings.Contains(blocks[0].Content, "runtime: bash") || !strings.Contains(blocks[0].Content, "entry: run.sh") {
		t.Fatalf("expected runtime entry in skill spec content: %q", blocks[0].Content)
	}
	if !strings.Contains(blocks[0].Content, "references: /tmp/skills/web_search/references/guide.md") || !strings.Contains(blocks[0].Content, "assets: /tmp/skills/web_search/assets/cover.png") {
		t.Fatalf("expected resources in skill spec content: %q", blocks[0].Content)
	}
	if !strings.Contains(blocks[0].Content, `"owner":"platform"`) {
		t.Fatalf("expected metadata in skill spec content: %q", blocks[0].Content)
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

func TestFormatSkillSpecIncludesResourcesAndMetadata(t *testing.T) {
	spec := &SkillSpec{
		SkillName:   "pptx",
		Description: "generate slides",
		Version:     "2.0.1",
		Enabled:     true,
		SourceType:  "vendor",
		RootDir:     "/tmp/skills/pptx",
		Runtime:     "node",
		Entry:       "generate.js",
		Tools:       []string{"generate_ppt"},
		References:  []string{"/tmp/skills/pptx/references/spec.md"},
		Assets:      []string{"/tmp/skills/pptx/assets/theme.json"},
		Metadata: map[string]any{
			"labels": []string{"slides", "report"},
		},
	}
	got := formatSkillSpec(spec)
	if !strings.Contains(got, "skill_name: pptx") || !strings.Contains(got, "source_type: vendor") {
		t.Fatalf("formatSkillSpec missing core fields: %q", got)
	}
	if !strings.Contains(got, "references: /tmp/skills/pptx/references/spec.md") || !strings.Contains(got, "assets: /tmp/skills/pptx/assets/theme.json") {
		t.Fatalf("formatSkillSpec missing resource fields: %q", got)
	}
	if !strings.Contains(got, `"labels":["slides","report"]`) {
		t.Fatalf("formatSkillSpec missing metadata json: %q", got)
	}
}

func TestRequestJSONLikeHandlesStructuredMetadata(t *testing.T) {
	got := requestJSONLike(BuildRequest{Variables: map[string]any{"meta": map[string]any{"a": 1}}}, "meta")
	if !json.Valid([]byte(got)) {
		t.Fatalf("requestJSONLike should emit json, got %q", got)
	}
}
