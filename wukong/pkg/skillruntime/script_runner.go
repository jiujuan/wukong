package skillruntime

import (
	"context"
	"errors"
)

var (
	// ErrScriptRunnerUnsupported is returned when no controlled runner is configured for a script.
	ErrScriptRunnerUnsupported = errors.New("skill script runner unsupported")
	// ErrScriptExecutionDisabled is returned by the default disabled runner.
	ErrScriptExecutionDisabled = errors.New("skill script execution disabled")
)

// ScriptRunner executes skill script resources under an explicitly controlled policy.
type ScriptRunner interface {
	CanRun(script SkillResource) bool
	Run(ctx context.Context, script SkillResource, input SkillInput, policy ToolPolicy) (*SkillOutput, error)
}

// DisabledScriptRunner is the safe default: it never executes external scripts.
type DisabledScriptRunner struct{}

// CanRun always returns false.
func (DisabledScriptRunner) CanRun(script SkillResource) bool { return false }

// Run always returns ErrScriptExecutionDisabled.
func (DisabledScriptRunner) Run(ctx context.Context, script SkillResource, input SkillInput, policy ToolPolicy) (*SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrScriptExecutionDisabled
}
