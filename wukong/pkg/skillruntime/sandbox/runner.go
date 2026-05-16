package sandbox

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	wksandbox "github.com/jiujuan/wukong/pkg/sandbox"
	"github.com/jiujuan/wukong/pkg/skillruntime"
)

var _ skillruntime.ScriptRunner = (*Runner)(nil)

// Executor is the sandbox capability needed by this adapter.
type Executor interface {
	Execute(ctx context.Context, req wksandbox.Request) (wksandbox.Result, error)
}

// Runner adapts pkg/sandbox to skillruntime.ScriptRunner.
type Runner struct {
	executor Executor
	timeout  time.Duration
}

// Option configures Runner.
type Option func(*Runner)

// WithExecutor configures the underlying sandbox executor.
func WithExecutor(executor Executor) Option {
	return func(r *Runner) {
		r.executor = executor
	}
}

// WithTimeout configures the script execution timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(r *Runner) {
		r.timeout = timeout
	}
}

// NewRunner creates a sandbox script runner skeleton. Without an executor it refuses execution.
func NewRunner(options ...Option) *Runner {
	r := &Runner{}
	for _, option := range options {
		if option != nil {
			option(r)
		}
	}
	return r
}

// CanRun reports whether this runner is configured and the resource looks like a script.
func (r *Runner) CanRun(script skillruntime.SkillResource) bool {
	return r != nil && r.executor != nil && strings.EqualFold(script.Kind, "script") && strings.TrimSpace(script.Path) != ""
}

// Run executes a script via the configured sandbox executor. It refuses execution when unconfigured.
func (r *Runner) Run(ctx context.Context, script skillruntime.SkillResource, input skillruntime.SkillInput, policy skillruntime.ToolPolicy) (*skillruntime.SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || r.executor == nil {
		return nil, skillruntime.ErrScriptRunnerUnsupported
	}
	if !r.CanRun(script) {
		return nil, skillruntime.ErrScriptRunnerUnsupported
	}

	req := wksandbox.Request{
		Runtime:    runtimeFromScriptPath(script.Path),
		ScriptPath: script.Path,
		WorkDir:    metadataString(input.Context.Metadata, "work_dir"),
		Input:      input.Text,
		Timeout:    r.timeout,
	}
	if req.WorkDir == "" {
		req.WorkDir = filepath.Dir(script.Path)
	}
	if req.Runtime == "" {
		return nil, skillruntime.ErrScriptRunnerUnsupported
	}

	result, err := r.executor.Execute(ctx, req)
	output := sandboxResultToSkillOutput(result, err)
	return output, err
}

func runtimeFromScriptPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python"
	case ".sh":
		return "bash"
	case ".ps1":
		return "powershell"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".go":
		return "go"
	default:
		return ""
	}
}

func sandboxResultToSkillOutput(result wksandbox.Result, err error) *skillruntime.SkillOutput {
	output := &skillruntime.SkillOutput{
		Status: "completed",
		Output: combineOutput(result.Stdout, result.Stderr),
		Result: map[string]any{
			"stdout":    result.Stdout,
			"stderr":    result.Stderr,
			"exit_code": result.ExitCode,
			"truncated": result.Truncated,
			"duration":  result.Duration.String(),
		},
	}
	if err != nil {
		output.Status = "failed"
		output.Error = err.Error()
		if output.Output == "" && result.Error != "" {
			output.Output = result.Error
		}
	}
	return output
}

func combineOutput(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout == "" {
		return stderr
	}
	if stderr == "" {
		return stdout
	}
	return stdout + "\n" + stderr
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func IsUnsupported(err error) bool {
	return errors.Is(err, skillruntime.ErrScriptRunnerUnsupported)
}
