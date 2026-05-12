package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
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
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "wukong-tool-*"+suffixByLanguage(language))
	if err != nil {
		t.logger.Error("[Tool] code_exec create temp file failed", "language", language, "error", err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		t.logger.Error("[Tool] code_exec write temp file failed", "file", tmpFile.Name(), "error", err)
		return nil, err
	}
	tmpFile.Close()

	cmd, err := commandByLanguage(runCtx, language, tmpFile.Name())
	if err != nil {
		t.logger.Error("[Tool] code_exec build command failed", "language", language, "error", err)
		return nil, err
	}
	output, err := cmd.CombinedOutput()
	result := map[string]any{
		"language": language,
		"output":   string(output),
	}
	if err != nil {
		result["error"] = err.Error()
		t.logger.Error("[Tool] code_exec failed", "language", language, "error", err)
		return result, err
	}
	t.logger.Info("[Tool] code_exec success", "language", language, "output_length", len(output))
	return result, nil
}
func suffixByLanguage(language string) string {
	switch language {
	case "python", "py":
		return ".py"
	case "javascript", "js", "node":
		return ".js"
	case "bash", "sh":
		return ".sh"
	case "powershell", "ps1":
		return ".ps1"
	default:
		return ".txt"
	}
}

func commandByLanguage(ctx context.Context, language, filename string) (*exec.Cmd, error) {
	switch language {
	case "python", "py":
		return exec.CommandContext(ctx, "python", filename), nil
	case "javascript", "js", "node":
		return exec.CommandContext(ctx, "node", filename), nil
	case "bash", "sh":
		return exec.CommandContext(ctx, "bash", filename), nil
	case "powershell", "ps1":
		return exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", filename), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}
