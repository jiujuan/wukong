package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/tool"
)

type fakeSubTask struct {
	subTaskID string
	taskID    string
	action    string
	params    map[string]any
	result    map[string]any
	errMsg    string
	updatedAt bool
}

func (s *fakeSubTask) GetSubTaskID() string      { return s.subTaskID }
func (s *fakeSubTask) GetTaskID() string         { return s.taskID }
func (s *fakeSubTask) GetAction() string         { return s.action }
func (s *fakeSubTask) GetParams() map[string]any { return s.params }
func (s *fakeSubTask) SetResult(result map[string]any) {
	s.result = result
}
func (s *fakeSubTask) SetError(msg string) { s.errMsg = msg }
func (s *fakeSubTask) SetUpdatedAt(_ time.Time) {
	s.updatedAt = true
}

type fakeActionExecutor struct {
	result map[string]any
	err    error
	called int
}

func (e *fakeActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	e.called++
	if e.err != nil {
		return nil, e.err
	}
	if e.result == nil {
		return map[string]any{"ok": true}, nil
	}
	out := make(map[string]any, len(e.result))
	for k, v := range e.result {
		out[k] = v
	}
	return out, nil
}

type fakeTool struct {
	name        string
	description string
	called      int
	result      map[string]any
}

func (t *fakeTool) Name() string                        { return t.name }
func (t *fakeTool) Description() string                 { return t.description }
func (t *fakeTool) ParameterSchema() []tool.ParamSchema { return nil }
func (t *fakeTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	t.called++
	out := map[string]any{"tool": t.name}
	for k, v := range t.result {
		out[k] = v
	}
	if q, ok := params["query"]; ok {
		out["query"] = q
	}
	return out, nil
}

type staticPromptBuilder struct {
	messages []llm.Message
	err      error
}

func (b *staticPromptBuilder) BuildMessages(ctx context.Context, subTask executableSubTask) ([]llm.Message, error) {
	if b.err != nil {
		return nil, b.err
	}
	return append([]llm.Message(nil), b.messages...), nil
}

type llmServerScript struct {
	mu       sync.Mutex
	requests []llm.ChatRequest
	replies  []llm.ChatResponse
	status   int
}

func newLLMServerScript(replies []llm.ChatResponse) (*httptest.Server, *llmServerScript) {
	script := &llmServerScript{
		replies: append([]llm.ChatResponse(nil), replies...),
		status:  http.StatusOK,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		script.mu.Lock()
		script.requests = append(script.requests, req)
		index := len(script.requests) - 1
		reply := llm.ChatResponse{}
		if index < len(script.replies) {
			reply = script.replies[index]
		}
		status := script.status
		script.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = fmt.Fprintf(w, `{"error":"status %d"}`, status)
			return
		}
		_ = json.NewEncoder(w).Encode(reply)
	}))
	return server, script
}

func newTestProvider(serverURL string) *llm.Provider {
	return llm.New(
		llm.WithProviderType(llm.ProviderTypeOpenAPI),
		llm.WithBaseURL(serverURL),
		llm.WithModel("test-model"),
		llm.WithHTTPClient(&http.Client{}),
	)
}

func newTestRegistry() *skills.Registry {
	return skills.New()
}

func TestExtractStringParam(t *testing.T) {
	params := map[string]any{"a": "  hello  ", "b": 123}
	if got := extractStringParam(params, "missing", "a"); got != "  hello  " {
		t.Fatalf("unexpected value: %q", got)
	}
	if got := extractStringParam(params, "b"); got != "123" {
		t.Fatalf("unexpected numeric conversion: %q", got)
	}
}

func TestResolveSkillAndToolName(t *testing.T) {
	if got := resolveSkillName("Web_Search", map[string]any{"skillName": "Chat"}); got != "chat" {
		t.Fatalf("unexpected skill name: %q", got)
	}
	if got := resolveToolName("report_gen", nil); got != "llm_chat" {
		t.Fatalf("unexpected tool name: %q", got)
	}
}

func TestCloneParams(t *testing.T) {
	src := map[string]any{"a": 1}
	dst := cloneParams(src)
	dst["a"] = 2
	if src["a"].(int) != 1 {
		t.Fatalf("clone should not mutate source")
	}
}

func TestActionPromptBuilderBuildMessages(t *testing.T) {
	builder := NewActionPromptBuilder()
	msgs, err := builder.BuildMessages(context.Background(), &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "web_search",
		params:    map[string]any{"query": "golang"},
	})
	if err != nil {
		t.Fatalf("build messages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("unexpected message count: %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("unexpected message roles: %#v", msgs)
	}
	if !strings.Contains(msgs[1].Content, "sub-1") || !strings.Contains(msgs[1].Content, "golang") {
		t.Fatalf("user prompt missing key fields: %q", msgs[1].Content)
	}
}

func TestActionPromptBuilderBuildMessagesIncludesContextEngineBlocks(t *testing.T) {
	builder := NewActionPromptBuilderWithRegistry(newTestRegistry())
	msgs, err := builder.BuildMessages(context.Background(), &fakeSubTask{
		subTaskID: "sub-ctx-1",
		taskID:    "task-ctx-1",
		action:    "web_search",
		params: map[string]any{
			"query":       "context prompt",
			"task_status": "RUNNING",
		},
	})
	if err != nil {
		t.Fatalf("build messages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("unexpected message count: %d", len(msgs))
	}
	if !strings.Contains(msgs[1].Content, "TaskState:") || !strings.Contains(msgs[1].Content, "task_id: task-ctx-1") {
		t.Fatalf("expected task state context in prompt: %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "SkillSpec:") || !strings.Contains(msgs[1].Content, "skill_name: web_search") {
		t.Fatalf("expected skill spec context in prompt: %q", msgs[1].Content)
	}
}

func TestLLMActionExecutorExecute(t *testing.T) {
	server, script := newLLMServerScript([]llm.ChatResponse{{
		Model: "test-model",
		Choices: []llm.Choice{{
			Message: llm.Message{Role: "assistant", Content: "final output"},
		}},
		Usage: llm.Usage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7},
	}})
	defer server.Close()

	provider := newTestProvider(server.URL)
	executor := NewLLMActionExecutor(provider, &staticPromptBuilder{
		messages: []llm.Message{{Role: "system", Content: "ctx"}, {Role: "user", Content: "prompt"}},
	})

	result, err := executor.Execute(context.Background(), &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "execute",
		params:    map[string]any{"x": "y"},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result["output"] != "final output" {
		t.Fatalf("unexpected output: %#v", result)
	}
	if result["model"] != "test-model" || result["total_tokens"] != 7 {
		t.Fatalf("unexpected metadata: %#v", result)
	}
	if len(script.requests) != 1 || len(script.requests[0].Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", script.requests)
	}
	if script.requests[0].Messages[0].Content != "ctx" || script.requests[0].Messages[1].Content != "prompt" {
		t.Fatalf("request should use prompt builder output, got %#v", script.requests[0].Messages)
	}
}

func TestSubTaskExecutorHandleRoutesAndAnnotatesResult(t *testing.T) {
	exec := NewSubTaskExecutor(nil, nil, nil)
	fake := &fakeActionExecutor{result: map[string]any{"output": "ok"}}
	exec.RegisterActionExecutor("custom", fake)

	task := &queue.Task{
		TaskID: "task-1",
		Data: &fakeSubTask{
			subTaskID: "sub-1",
			taskID:    "task-1",
			action:    "CUSTOM",
			params:    map[string]any{"a": 1},
		},
	}

	if err := exec.Handle(context.Background(), task); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if fake.called != 1 {
		t.Fatalf("executor should be called once")
	}
	sub := task.Data.(*fakeSubTask)
	if sub.result["sub_task_id"] != "sub-1" || sub.result["action"] != "CUSTOM" {
		t.Fatalf("result not annotated: %#v", sub.result)
	}
	if !sub.updatedAt {
		t.Fatalf("updatedAt should be set")
	}
}

func TestToolActionExecutorExecuteQueryFallback(t *testing.T) {
	manager := tool.NewManager()
	fake := &fakeTool{name: "web_search", result: map[string]any{"answer": "found"}}
	manager.Register(fake)

	executor := NewToolActionExecutor(manager, "web_search", "web_search")
	result, err := executor.Execute(context.Background(), &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "web_search",
		params:    map[string]any{"prompt": "golang"},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result["tool"] != "web_search" || result["query"] != "golang" {
		t.Fatalf("unexpected tool result: %#v", result)
	}
	if fake.called != 1 {
		t.Fatalf("tool should be called once")
	}
}

func TestCompositeActionExecutorFallback(t *testing.T) {
	primary := &fakeActionExecutor{err: fmt.Errorf("boom")}
	fallback := &fakeActionExecutor{result: map[string]any{"output": "fallback"}}
	executor := NewCompositeActionExecutor(primary, fallback)

	result, err := executor.Execute(context.Background(), &fakeSubTask{})
	if err != nil {
		t.Fatalf("fallback execute failed: %v", err)
	}
	if result["output"] != "fallback" || primary.called != 1 || fallback.called != 1 {
		t.Fatalf("unexpected composite behavior: %#v", result)
	}
}

func TestSkillAwareActionExecutor(t *testing.T) {
	manager := tool.NewManager()
	fake := &fakeTool{name: "llm_chat", result: map[string]any{"output": "tool"}}
	manager.Register(fake)
	registry := newTestRegistry()
	executor := NewSkillAwareActionExecutor(manager, registry, &fakeActionExecutor{result: map[string]any{"output": "fallback"}})

	result, err := executor.Execute(context.Background(), &fakeSubTask{
		action: "chat",
		params: map[string]any{"tool_name": "llm_chat"},
	})
	if err != nil {
		t.Fatalf("skill aware execute failed: %v", err)
	}
	if result["tool"] != "llm_chat" || fake.called != 1 {
		t.Fatalf("tool path should win: %#v", result)
	}

	fallback := NewSkillAwareActionExecutor(nil, registry, &fakeActionExecutor{result: map[string]any{"output": "fallback"}})
	result, err = fallback.Execute(context.Background(), &fakeSubTask{
		action: "chat",
		params: map[string]any{"tool_name": "web_search"},
	})
	if err != nil {
		t.Fatalf("fallback execute failed: %v", err)
	}
	if result["output"] != "fallback" {
		t.Fatalf("unexpected fallback result: %#v", result)
	}
}

func TestIntegrationFileSkillWritesOnlyUnderAllowedDirAndReturnsTaskResult(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "file_skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := strings.Join([]string{
		"# Skill: file_skill",
		"## Description",
		"file write skill",
		"## Tools",
		"- file_write",
	}, "\n")
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	registry := skills.New(skills.WithRootDir(root))
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("start registry failed: %v", err)
	}
	defer registry.Stop()

	outputDir := t.TempDir()
	manager := tool.NewManager(
		tool.WithSkillsRegistry(registry),
		tool.WithFileWriteDir(outputDir),
	)
	executor := NewSubTaskExecutorWithTools(nil, nil, nil, manager, registry)
	executor.RegisterActionExecutor("file_skill", NewSkillAwareActionExecutor(manager, registry, nil))

	task := &queue.Task{
		TaskID: "task-file-1",
		Data: &fakeSubTask{
			subTaskID: "sub-file-1",
			taskID:    "task-file-1",
			action:    "file_skill",
			params: map[string]any{
				"skill_name": "file_skill",
				"tool_name":  "file_write",
				"path":       "reports/daily.md",
				"content":    "hello file skill",
			},
		},
	}

	if err := executor.Handle(context.Background(), task); err != nil {
		t.Fatalf("handle failed: %v", err)
	}

	sub := task.Data.(*fakeSubTask)
	if sub.result == nil {
		t.Fatal("result should be set")
	}
	if sub.result["tool"] != "file_write" {
		t.Fatalf("unexpected tool annotation: %#v", sub.result)
	}
	gotPath, ok := sub.result["path"].(string)
	if !ok || gotPath == "" {
		t.Fatalf("path missing from result: %#v", sub.result)
	}
	wantPath := filepath.Join(outputDir, "file_skill", "reports", "daily.md")
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("written file missing: %v", err)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read written file failed: %v", err)
	}
	if string(data) != "hello file skill" {
		t.Fatalf("file content = %q, want %q", string(data), "hello file skill")
	}
	if !sub.updatedAt {
		t.Fatalf("updatedAt should be set")
	}
	if sub.errMsg != "" {
		t.Fatalf("unexpected subtask error: %q", sub.errMsg)
	}

	_, err = manager.ExecuteForSkill(context.Background(), "file_skill", "file_write", map[string]any{
		"path":    "../escape.md",
		"content": "nope",
	})
	if err == nil {
		t.Fatalf("expected escape write to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "out of base dir") {
		t.Fatalf("unexpected escape error: %v", err)
	}
}
