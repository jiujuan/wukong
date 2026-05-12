package tools

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

func TestFileWriteToolExecuteAutoPathUsesTitleAndTime(t *testing.T) {
	root := t.TempDir()
	tool := newTestFileWriteTool(root, time.Date(2026, 5, 12, 9, 8, 7, 123456000, time.Local))

	result, err := tool.Execute(context.Background(), map[string]any{
		"title":   "文章标题",
		"content": "正文内容",
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	gotPath := resultPath(t, result)
	wantPath := filepath.Join(root, "20260512", "文章标题-090807123456.md")
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	assertFileContent(t, gotPath, "正文内容")
	if got := result["written_bytes"]; got != len("正文内容") {
		t.Fatalf("written_bytes = %v, want %d", got, len("正文内容"))
	}
	if got := result["append"]; got != false {
		t.Fatalf("append = %v, want false", got)
	}
}

func TestFileWriteToolExecuteSanitizesAutoPathTitle(t *testing.T) {
	root := t.TempDir()
	tool := newTestFileWriteTool(root, time.Date(2026, 5, 12, 23, 48, 27, 210231000, time.Local))

	result, err := tool.Execute(context.Background(), map[string]any{
		"title":   `  文章/标题:草稿 * 版本?  `,
		"content": "safe",
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	gotPath := resultPath(t, result)
	wantPath := filepath.Join(root, "20260512", "文章_标题_草稿_版本-234827210231.md")
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	assertFileContent(t, gotPath, "safe")
}

func TestFileWriteToolExecuteAutoPathFallsBackToAliasAndUntitled(t *testing.T) {
	now := time.Date(2026, 5, 12, 1, 2, 3, 4000, time.Local)
	tests := []struct {
		name     string
		params   map[string]any
		wantName string
	}{
		{
			name:     "topic alias",
			params:   map[string]any{"topic": "主题名称", "content": "topic body"},
			wantName: "主题名称-010203000004.md",
		},
		{
			name:     "query alias",
			params:   map[string]any{"query": "查询词", "content": "query body"},
			wantName: "查询词-010203000004.md",
		},
		{
			name:     "untitled",
			params:   map[string]any{"content": ""},
			wantName: "untitled-010203000004.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tool := newTestFileWriteTool(root, now)

			result, err := tool.Execute(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}

			gotPath := resultPath(t, result)
			wantPath := filepath.Join(root, "20260512", tt.wantName)
			if gotPath != wantPath {
				t.Fatalf("path = %q, want %q", gotPath, wantPath)
			}
		})
	}
}

func TestFileWriteToolExecuteAutoPathUsesContentTitleWhenMissing(t *testing.T) {
	root := t.TempDir()
	tool := newTestFileWriteTool(root, time.Date(2026, 5, 12, 1, 2, 3, 4000, time.Local))

	result, err := tool.Execute(context.Background(), map[string]any{
		"content": "# AI 市场分析报告\n\n正文内容",
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	gotPath := resultPath(t, result)
	wantPath := filepath.Join(root, "20260512", "AI_市场分析报告-010203000004.md")
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
}

func TestFileWriteToolExecuteExplicitPathOverwriteAndAppend(t *testing.T) {
	root := t.TempDir()
	tool := newTestFileWriteTool(root, time.Date(2026, 5, 12, 10, 0, 0, 0, time.Local))
	params := map[string]any{
		"path":    "reports/result.md",
		"content": "first",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("first Execute() failed: %v", err)
	}
	target := resultPath(t, result)
	assertFileContent(t, target, "first")

	_, err = tool.Execute(context.Background(), map[string]any{
		"path":    "reports/result.md",
		"content": "second",
	})
	if err != nil {
		t.Fatalf("overwrite Execute() failed: %v", err)
	}
	assertFileContent(t, target, "second")

	appendResult, err := tool.Execute(context.Background(), map[string]any{
		"path":    "reports/result.md",
		"content": "+third",
		"append":  true,
	})
	if err != nil {
		t.Fatalf("append Execute() failed: %v", err)
	}
	assertFileContent(t, target, "second+third")
	if got := appendResult["append"]; got != true {
		t.Fatalf("append = %v, want true", got)
	}
}

func TestFileWriteToolExecuteRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	tool := newTestFileWriteTool(root, time.Date(2026, 5, 12, 10, 0, 0, 0, time.Local))

	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    filepath.Join("..", "escape.md"),
		"content": "bad",
	})
	if err == nil {
		t.Fatalf("expected path escape to fail")
	}
	if !strings.Contains(err.Error(), "path out of base dir") {
		t.Fatalf("error = %v, want path out of base dir", err)
	}
}

func TestFileWriteToolMetadataAndSchema(t *testing.T) {
	tool := NewFileWriteTool("out", pkglogger.New(pkglogger.WithOutput(io.Discard)))
	if tool.Name() != "file_write" {
		t.Fatalf("Name() = %q, want file_write", tool.Name())
	}
	if strings.TrimSpace(tool.Description()) == "" {
		t.Fatalf("Description() is empty")
	}

	schema := tool.ParameterSchema()
	assertSchemaField(t, schema, "path", "string", false)
	assertSchemaField(t, schema, "content", "string", true)
	assertSchemaField(t, schema, "append", "bool", false)
	assertSchemaField(t, schema, "title", "string", false)
}

func TestBuildAutoWritePathAndSanitizePathFragment(t *testing.T) {
	now := time.Date(2026, 5, 12, 23, 59, 58, 900001000, time.Local)
	got := buildAutoWritePath(map[string]any{
		"name": "  A/B C:D*E?F\"G<H>I|J  ",
	}, now)
	want := filepath.Join("20260512", "A_B_C_D_E_F_G_H_I_J-235958900001.md")
	if got != want {
		t.Fatalf("buildAutoWritePath() = %q, want %q", got, want)
	}

	if got := sanitizePathFragment(" .__ "); got != "" {
		t.Fatalf("sanitizePathFragment() = %q, want empty", got)
	}
	if got := sanitizePathFragment("a\tb"); got != "ab" {
		t.Fatalf("sanitizePathFragment() control chars = %q, want ab", got)
	}
}

func newTestFileWriteTool(root string, now time.Time) *FileWriteTool {
	return &FileWriteTool{
		baseDir: root,
		logger:  pkglogger.New(pkglogger.WithOutput(io.Discard)),
		now:     func() time.Time { return now },
	}
}

func resultPath(t *testing.T, result map[string]any) string {
	t.Helper()
	gotPath, ok := result["path"].(string)
	if !ok || gotPath == "" {
		t.Fatalf("path missing from result: %#v", result)
	}
	return gotPath
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q failed: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file content = %q, want %q", string(data), want)
	}
}

func assertSchemaField(t *testing.T, schema []ParamSchema, name, typ string, required bool) {
	t.Helper()
	for _, field := range schema {
		if field.Name == name {
			if field.Type != typ || field.Required != required {
				t.Fatalf("schema field %s = %#v, want type=%s required=%v", name, field, typ, required)
			}
			return
		}
	}
	t.Fatalf("schema field %s not found in %#v", name, schema)
}
