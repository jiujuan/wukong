package tool

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

func TestFileWriteToolExplicitPath(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{
		baseDir: root,
		logger:  pkglogger.New(pkglogger.WithOutput(io.Discard)),
		now:     func() time.Time { return time.Date(2026, 5, 12, 23, 48, 27, 210231000, time.Local) },
	}

	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    "manual/report.md",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	want := filepath.Join(root, "manual", "report.md")
	if got := result["path"]; got != want {
		t.Fatalf("path = %v, want %v", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content = %q, want %q", string(data), "hello")
	}
}

func TestFileWriteToolAutoPath(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{
		baseDir: root,
		logger:  pkglogger.New(pkglogger.WithOutput(io.Discard)),
		now:     func() time.Time { return time.Date(2026, 5, 12, 23, 48, 27, 210231000, time.Local) },
	}

	result, err := tool.Execute(context.Background(), map[string]any{
		"title":   "日报/总结",
		"content": "auto",
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	gotPath, ok := result["path"].(string)
	if !ok {
		t.Fatalf("path missing from result: %#v", result)
	}
	wantSuffix := filepath.Join("20260512", "日报_总结-234827210231.md")
	if !strings.HasSuffix(gotPath, wantSuffix) {
		t.Fatalf("path suffix = %q, want suffix %q", gotPath, wantSuffix)
	}
	if !strings.HasPrefix(gotPath, filepath.Clean(root)) {
		t.Fatalf("path = %q, want under %q", gotPath, root)
	}
	data, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(data) != "auto" {
		t.Fatalf("content = %q, want %q", string(data), "auto")
	}
}

func TestResolvePathBlocksEscape(t *testing.T) {
	root := t.TempDir()
	_, err := resolvePath(root, filepath.Join("..", "escape.md"))
	if err == nil {
		t.Fatalf("expected escape path to fail")
	}
}
