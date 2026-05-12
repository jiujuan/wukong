package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type FileReadTool struct {
	baseDir string
	logger  *pkglogger.Logger
}

func NewFileReadTool(baseDir string, logger *pkglogger.Logger) *FileReadTool {
	return &FileReadTool{baseDir: baseDir, logger: logger}
}

func (t *FileReadTool) Name() string { return "file_read" }

func (t *FileReadTool) Description() string { return "读取本地文件内容" }

func (t *FileReadTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("path", "string", true, "relative path within base directory", nil, "docs/readme.md"),
	}
}

func (t *FileReadTool) Execute(_ context.Context, params map[string]any) (map[string]any, error) {
	path := readString(params, "path")
	if strings.TrimSpace(path) == "" {
		t.logger.Warn("[Tool] file_read invalid params: path is empty")
		return nil, fmt.Errorf("path is required")
	}
	t.logger.Info("[Tool] file_read start", "path", path)
	target, err := resolvePath(t.baseDir, path)
	if err != nil {
		t.logger.Error("[Tool] file_read resolve path failed", "path", path, "error", err)
		return nil, err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.logger.Error("[Tool] file_read read file failed", "path", target, "error", err)
		return nil, err
	}
	result := map[string]any{
		"path":    target,
		"content": string(content),
		"size":    len(content),
	}
	t.logger.Info("[Tool] file_read success", "path", target, "size", len(content))
	return result, nil
}
