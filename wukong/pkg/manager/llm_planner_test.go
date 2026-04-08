package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jiujuan/wukong/pkg/llm"
)

func TestLLMPlannerFallbackWhenInvalidJSON(t *testing.T) {
	provider := newTestProvider(t, `not-json`)
	planner := NewLLMPlanner(provider, NewTplPlanner())

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
