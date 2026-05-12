package tool

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/skills"
	tooltools "github.com/jiujuan/wukong/pkg/tool/tools"
)

type testTool struct {
	name        string
	description string
	executeFn   func(context.Context, map[string]any) (map[string]any, error)
}

func (t *testTool) Name() string { return t.name }

func (t *testTool) Description() string { return t.description }

func (t *testTool) ParameterSchema() []ParamSchema { return nil }

func (t *testTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	return t.executeFn(ctx, params)
}

func TestNewManagerRegistersBuiltins(t *testing.T) {
	m := NewManager()

	for _, name := range []string{"llm_chat", "web_search", "file_read", "file_write", "http_request", "code_exec", "memory_read", "memory_write"} {
		if _, ok := m.Get(name); !ok {
			t.Fatalf("builtin tool %q not registered", name)
		}
	}

	items := m.List()
	var fileWriteSchema []ParamSchema
	for _, item := range items {
		if item.Name == "file_write" {
			fileWriteSchema = item.Schema
			break
		}
	}
	if len(fileWriteSchema) == 0 {
		t.Fatalf("file_write schema should not be empty")
	}
	hasPath := false
	hasContent := false
	for _, field := range fileWriteSchema {
		switch field.Name {
		case "path":
			hasPath = true
		case "content":
			hasContent = true
		}
	}
	if !hasPath || !hasContent {
		t.Fatalf("file_write schema missing required fields: %#v", fileWriteSchema)
	}
}

func TestManagerRegisterGetListAndExecute(t *testing.T) {
	m := NewManager()
	called := false
	m.Register(&testTool{
		name:        "echo",
		description: "echo tool",
		executeFn: func(_ context.Context, params map[string]any) (map[string]any, error) {
			called = true
			return map[string]any{"input": params["input"], "ok": true}, nil
		},
	})

	item, ok := m.Get(" ECHO ")
	if !ok {
		t.Fatalf("expected tool to be found")
	}
	if item.Name() != "echo" {
		t.Fatalf("unexpected tool name: %s", item.Name())
	}

	list := m.List()
	if len(list) == 0 {
		t.Fatalf("expected list to contain tools")
	}

	result, err := m.Execute(context.Background(), "echo", map[string]any{"input": "hello"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !called {
		t.Fatalf("tool was not executed")
	}
	if got := result["input"]; got != "hello" {
		t.Fatalf("input = %v, want hello", got)
	}
	if got := result["ok"]; got != true {
		t.Fatalf("ok = %v, want true", got)
	}
}

func TestManagerExecuteMissingTool(t *testing.T) {
	m := NewManager()
	if _, err := m.Execute(context.Background(), "missing", nil); err == nil {
		t.Fatalf("expected missing tool error")
	}
}

func TestManagerExecuteForSkillPolicy(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "report_gen")
	if err := ensureSkillFile(skillDir, "report_gen", []string{"llm_chat"}); err != nil {
		t.Fatalf("prepare skill file failed: %v", err)
	}

	registry := skills.New(skills.WithRootDir(root))
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("start skills registry failed: %v", err)
	}
	registry.Stop()

	m := NewManager(WithSkillsRegistry(registry))
	m.Register(&testTool{
		name:        "llm_chat",
		description: "llm tool",
		executeFn: func(_ context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"output": params["value"]}, nil
		},
	})

	result, err := m.ExecuteForSkill(context.Background(), "report_gen", "llm_chat", map[string]any{"value": "allowed"})
	if err != nil {
		t.Fatalf("allowed tool execution failed: %v", err)
	}
	if got := result["output"]; got != "allowed" {
		t.Fatalf("output = %v, want allowed", got)
	}

	if _, err := m.ExecuteForSkill(context.Background(), "report_gen", "blocked", nil); err == nil {
		t.Fatalf("expected blocked tool error")
	}
}

func TestManagerExecuteForUnknownSkillRejected(t *testing.T) {
	m := NewManager(WithSkillsRegistry(skills.New()))
	m.Register(&testTool{
		name:        "echo",
		description: "echo tool",
		executeFn: func(_ context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"value": params["value"]}, nil
		},
	})

	if _, err := m.ExecuteForSkill(context.Background(), "missing_skill", "echo", map[string]any{"value": "ok"}); err == nil {
		t.Fatalf("expected unknown skill to be rejected")
	}
}

func TestManagerExecuteForSkillAppliesSkillContext(t *testing.T) {
	root := t.TempDir()
	output := t.TempDir()
	skillDir := filepath.Join(root, "report_gen")
	if err := ensureSkillFile(skillDir, "report_gen", []string{"file_write"}); err != nil {
		t.Fatalf("prepare skill file failed: %v", err)
	}

	registry := skills.New(skills.WithRootDir(root))
	if err := registry.Start(context.Background()); err != nil {
		t.Fatalf("start skills registry failed: %v", err)
	}
	defer registry.Stop()

	m := NewManager(
		WithSkillsRegistry(registry),
		WithFileWriteDir(output),
	)
	toolItem, ok := m.Get("file_write")
	if !ok {
		t.Fatalf("file_write tool not found")
	}

	ctx := tooltools.WithSkillContext(context.Background(), tooltools.SkillContext{
		SkillName: "report_gen",
		SkillRoot: skillDir,
		OutputDir: filepath.Join(output, "report_gen"),
	})
	result, err := toolItem.Execute(ctx, map[string]any{
		"path":    "reports/daily.md",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("file_write execute failed: %v", err)
	}
	want := filepath.Join(output, "report_gen", "reports", "daily.md")
	if got := result["path"]; got != want {
		t.Fatalf("path = %v, want %v", got, want)
	}
}

func ensureSkillFile(dir, skillName string, tools []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Skill: " + skillName + "\n")
	b.WriteString("## Description\n")
	b.WriteString("test skill\n")
	b.WriteString("## Tools\n")
	for _, tool := range tools {
		b.WriteString("- " + tool + "\n")
	}
	b.WriteString("## Execute\n")
	b.WriteString("- run.sh\n")
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(b.String()), 0o644)
}

func TestWithLoggerAndOtherOptions(t *testing.T) {
	llmProvider := &llm.Provider{}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	store := NewInMemoryStore()
	registry := skills.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	m := NewManager(
		WithLogger(logger),
		WithLLMProvider(llmProvider),
		WithSkillsRegistry(registry),
		WithMemoryStore(store),
		WithBaseDir("assets"),
		WithFileWriteDir("output"),
		WithHTTPClient(httpClient),
		WithExecTimeout(5*time.Second),
	)

	if m.llmProvider != llmProvider {
		t.Fatalf("llm provider not applied")
	}
	if m.skillsRegistry != registry {
		t.Fatalf("skills registry not applied")
	}
	if m.memoryStore != store {
		t.Fatalf("memory store not applied")
	}
	if m.baseDir != "assets" {
		t.Fatalf("base dir = %q, want assets", m.baseDir)
	}
	if m.fileWriteDir != "output" {
		t.Fatalf("file write dir = %q, want output", m.fileWriteDir)
	}
	if m.httpClient != httpClient {
		t.Fatalf("http client not applied")
	}
	if m.execTimeout != 5*time.Second {
		t.Fatalf("exec timeout = %v, want 5s", m.execTimeout)
	}
	if m.logger == nil {
		t.Fatalf("logger should be set")
	}
}
