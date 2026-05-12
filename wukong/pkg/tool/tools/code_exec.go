package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/sandbox"
)

type CodeExecTool struct {
	timeout time.Duration
	logger  *pkglogger.Logger
}

func NewCodeExecTool(timeout time.Duration, logger *pkglogger.Logger) *CodeExecTool {
	return &CodeExecTool{timeout: timeout, logger: logger}
}

func (t *CodeExecTool) Name() string { return "code_exec" }

func (t *CodeExecTool) Description() string { return "执行代码片段" }

func (t *CodeExecTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("language", "string", true, "runtime language", nil, "python"),
		schemaItem("lang", "string", false, "alias for language", nil, "python"),
		schemaItem("code", "string", true, "source code to execute", nil, "print('hello')"),
	}
}

func (t *CodeExecTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	language := strings.ToLower(strings.TrimSpace(readString(params, "language", "lang")))
	code := readString(params, "code")
	if language == "" || strings.TrimSpace(code) == "" {
		t.logger.Warn("[Tool] code_exec invalid params", "language", language, "has_code", strings.TrimSpace(code) != "")
		return nil, fmt.Errorf("language and code are required")
	}
	timeout := t.timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	t.logger.Info("[Tool] code_exec start", "language", language, "timeout", timeout)
	tmpDir, err := os.MkdirTemp("", "wukong-tool-*")
	if err != nil {
		t.logger.Error("[Tool] code_exec create temp dir failed", "language", language, "error", err)
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	filename, err := codeExecFilename(language)
	if err != nil {
		t.logger.Error("[Tool] code_exec resolve filename failed", "language", language, "error", err)
		return nil, err
	}
	scriptPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(scriptPath, []byte(code), 0o644); err != nil {
		t.logger.Error("[Tool] code_exec write temp file failed", "file", scriptPath, "error", err)
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sandboxResult, err := sandbox.Execute(runCtx, sandbox.Request{
		Runtime:    language,
		ScriptPath: scriptPath,
		WorkDir:    tmpDir,
		Timeout:    timeout,
	})
	result := map[string]any{
		"language":  language,
		"stdout":    sandboxResult.Stdout,
		"stderr":    sandboxResult.Stderr,
		"output":    combineOutput(sandboxResult.Stdout, sandboxResult.Stderr),
		"exit_code": sandboxResult.ExitCode,
		"duration":  sandboxResult.Duration.String(),
		"truncated": sandboxResult.Truncated,
	}
	if err != nil {
		result["error"] = err.Error()
		if output, ok := result["output"].(string); !ok || strings.TrimSpace(output) == "" {
			if sandboxResult.Error != "" {
				result["output"] = sandboxResult.Error
			}
		}
		t.logger.Error("[Tool] code_exec failed", "language", language, "error", err)
		return result, err
	}
	t.logger.Info("[Tool] code_exec success", "language", language, "output_length", len(result["output"].(string)))
	return result, nil
}

func codeExecFilename(language string) (string, error) {
	switch language {
	case "python", "py":
		return "main.py", nil
	case "javascript", "js", "node":
		return "main.js", nil
	case "bash", "sh":
		return "main.sh", nil
	case "powershell", "ps1":
		return "main.ps1", nil
	case "go":
		return "main.go", nil
	case "java":
		return "Main.java", nil
	case "typescript", "ts":
		return "main.ts", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func combineOutput(stdout, stderr string) string {
	if strings.TrimSpace(stdout) == "" {
		return stderr
	}
	if strings.TrimSpace(stderr) == "" {
		return stdout
	}
	return stdout + "\n" + stderr
}
