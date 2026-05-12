package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type FileWriteTool struct {
	baseDir string
	logger  *pkglogger.Logger
	now     func() time.Time
}

func (t *FileWriteTool) Name() string { return "file_write" }

func (t *FileWriteTool) Description() string { return "写入本地文件内容" }

func (t *FileWriteTool) Execute(_ context.Context, params map[string]any) (map[string]any, error) {
	path := readString(params, "path")
	content := readString(params, "content")
	if t.now == nil {
		t.now = time.Now
	}
	if strings.TrimSpace(path) == "" {
		path = buildAutoWritePath(params, t.now())
	}
	if strings.TrimSpace(path) == "" {
		t.logger.Warn("[Tool] file_write invalid params: path is empty")
		return nil, fmt.Errorf("path is required")
	}
	t.logger.Info("[Tool] file_write start", "path", path, "content_length", len(content))
	target, err := resolvePath(t.baseDir, path)
	if err != nil {
		t.logger.Error("[Tool] file_write resolve path failed", "path", path, "error", err)
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.logger.Error("[Tool] file_write mkdir failed", "path", target, "error", err)
		return nil, err
	}
	appendMode, _ := params["append"].(bool)
	var f *os.File
	if appendMode {
		f, err = os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	} else {
		f, err = os.Create(target)
	}
	if err != nil {
		t.logger.Error("[Tool] file_write open file failed", "path", target, "append", appendMode, "error", err)
		return nil, err
	}
	defer f.Close()
	n, err := f.WriteString(content)
	if err != nil {
		t.logger.Error("[Tool] file_write write failed", "path", target, "error", err)
		return nil, err
	}
	result := map[string]any{
		"path":          target,
		"written_bytes": n,
		"append":        appendMode,
	}
	t.logger.Info("[Tool] file_write success", "path", target, "written_bytes", n, "append", appendMode)
	return result, nil
}
