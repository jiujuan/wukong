package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/skills"
)

func TestLLMPlannerFallbackWhenInvalidJSON(t *testing.T) {
	provider := newTestProvider(t, `not-json`)
	planner := NewLLMPlannerWithRegistry(provider, NewTplPlanner(), skills.New())

	defs, err := planner.PlanSubTasks(context.Background(), newSearchTask())
	if err != nil {
		t.Fatalf("plan should fallback without error: %v", err)
	}
	assertSearchTemplateFallback(t, defs)
}

func TestLLMPlannerFallbackWhenMissingDependency(t *testing.T) {
	provider := newTestProvider(t, `{"thought":"x","steps":[{"id":"s1","action":"web_search","params":{},"depends_on":[]},{"id":"s2","action":"report_gen","params":{},"depends_on":["missing"]}]}`)
	planner := NewLLMPlanner(provider, NewTplPlanner())

	defs, err := planner.PlanSubTasks(context.Background(), newSearchTask())
	if err != nil {
		t.Fatalf("plan should fallback without error: %v", err)
	}
	assertSearchTemplateFallback(t, defs)
}

func TestLLMPlannerFallbackWhenProviderNil(t *testing.T) {
	planner := NewLLMPlanner(nil, NewTplPlanner())

	defs, err := planner.PlanSubTasks(context.Background(), newSearchTask())
	if err != nil {
		t.Fatalf("plan should fallback without error: %v", err)
	}
	assertSearchTemplateFallback(t, defs)
}

func TestLLMPlannerUsesPromptEngineMessages(t *testing.T) {
	server, requests := newPlannerCaptureServer(t, `{"thought":"plan","steps":[{"id":"s1","action":"web_search","params":{"query":"ai agent"},"depends_on":[],"thought":"search first"}]}`)
	provider := llm.New(
		llm.WithProviderType(llm.ProviderTypeOpenAPI),
		llm.WithBaseURL(server.URL),
		llm.WithModel("test-model"),
	)
	planner := NewLLMPlanner(provider, NewTplPlanner())

	defs, err := planner.PlanSubTasks(context.Background(), newSearchTask())
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(defs) != 1 || defs[0].Action != "web_search" {
		t.Fatalf("unexpected defs: %#v", defs)
	}
	if len(*requests) != 1 || len((*requests)[0].Messages) != 2 {
		t.Fatalf("unexpected llm requests: %#v", *requests)
	}
	if (*requests)[0].Messages[0].Role != "system" || (*requests)[0].Messages[1].Role != "user" {
		t.Fatalf("unexpected planner messages: %#v", (*requests)[0].Messages)
	}
	if (*requests)[0].Messages[1].Content == "" || !containsAll((*requests)[0].Messages[1].Content, []string{"task_id=task_test_1", "skill=search", "task_state=", "skill_spec="}) {
		t.Fatalf("unexpected planner user prompt: %q", (*requests)[0].Messages[1].Content)
	}
	if !strings.Contains((*requests)[0].Messages[1].Content, "skill_name: search") {
		t.Fatalf("expected planner prompt to include skill spec context: %q", (*requests)[0].Messages[1].Content)
	}
}

func TestLLMPlannerIncludesCanonicalSkillResourcesInPrompt(t *testing.T) {
	root := t.TempDir()
	writePlannerSkillPackage(t, filepath.Join(root, "local"), "search", "search skill", "run.sh", "#!/usr/bin/env bash\nprintf 'ok'\n")

	registry := skills.New(skills.WithRootDir(root))
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("start registry failed: %v", err)
	}
	defer registry.Stop()

	server, requests := newPlannerCaptureServer(t, `{"thought":"plan","steps":[{"id":"s1","action":"web_search","params":{"query":"ai agent"},"depends_on":[]}]}`)
	provider := llm.New(
		llm.WithProviderType(llm.ProviderTypeOpenAPI),
		llm.WithBaseURL(server.URL),
		llm.WithModel("test-model"),
	)
	planner := NewLLMPlannerWithRegistry(provider, NewTplPlanner(), registry)

	if _, err := planner.PlanSubTasks(context.Background(), newSearchTask()); err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("unexpected llm requests: %#v", *requests)
	}
	content := (*requests)[0].Messages[1].Content
	if !containsAll(content, []string{
		"source_type: local",
		"root_dir: " + filepath.Join(root, "local", "search"),
		"runtime: bash",
		"entry: run.sh",
		"references: " + filepath.Join(root, "local", "search", "references", "guide.md"),
		"assets: " + filepath.Join(root, "local", "search", "assets", "theme.json"),
	}) {
		t.Fatalf("planner prompt missing canonical skill fields: %q", content)
	}
	if !strings.Contains(content, `"references_count":1`) || !strings.Contains(content, `"assets_count":1`) {
		t.Fatalf("planner prompt missing resource metadata: %q", content)
	}
}

func writePlannerSkillPackage(t *testing.T, root, name, description, exec, script string) {
	t.Helper()
	skillDir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir references failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write reference failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "assets", "theme.json"), []byte(`{"theme":"light"}`), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := strings.Join([]string{
		"# Skill: " + name,
		"## Description",
		description,
		"## Tools",
		"- llm_chat",
		"## Execute",
		"- " + exec,
	}, "\n")
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, exec), []byte(script), 0o644); err != nil {
		t.Fatalf("write skill script failed: %v", err)
	}
}

func newSearchTask() *Task {
	return &Task{
		TaskID:    "task_test_1",
		SkillName: "search",
		Params: map[string]any{
			"query": "ai agent",
		},
	}
}

func assertSearchTemplateFallback(t *testing.T, defs []SubTaskDef) {
	t.Helper()
	if len(defs) != 3 {
		t.Fatalf("fallback should produce 3 subtasks, got %d", len(defs))
	}
	if defs[0].Action != "search_prepare" || defs[1].Action != "search_execute" || defs[2].Action != "search_aggregate" {
		t.Fatalf("unexpected fallback actions: %s, %s, %s", defs[0].Action, defs[1].Action, defs[2].Action)
	}
}

func newTestProvider(t *testing.T, content string) *llm.Provider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl_test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":` + quoteJSONString(content) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)
	return llm.New(
		llm.WithProviderType(llm.ProviderTypeOpenAPI),
		llm.WithBaseURL(server.URL),
		llm.WithModel("test-model"),
	)
}

func quoteJSONString(raw string) string {
	b := make([]byte, 0, len(raw)+2)
	b = append(b, '"')
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch ch {
		case '\\':
			b = append(b, '\\', '\\')
		case '"':
			b = append(b, '\\', '"')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, ch)
		}
	}
	b = append(b, '"')
	return string(b)
}

func newPlannerCaptureServer(t *testing.T, content string) (*httptest.Server, *[]llm.ChatRequest) {
	t.Helper()
	requests := make([]llm.ChatRequest, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		requests = append(requests, req)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl_test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":` + quoteJSONString(content) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func containsAll(raw string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(raw, part) {
			return false
		}
	}
	return true
}
