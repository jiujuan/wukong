package skillruntime

import (
	"context"
	"errors"
	"testing"
)

func TestDisabledScriptRunnerDoesNotRunScripts(t *testing.T) {
	runner := DisabledScriptRunner{}
	script := SkillResource{Kind: "script", Path: "scripts/run.sh"}

	if runner.CanRun(script) {
		t.Fatal("CanRun() = true, want false")
	}

	_, err := runner.Run(context.Background(), script, SkillInput{}, ToolPolicy{})
	if !errors.Is(err, ErrScriptExecutionDisabled) {
		t.Fatalf("Run() error = %v, want ErrScriptExecutionDisabled", err)
	}
}
