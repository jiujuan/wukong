package sandbox

import (
	"context"
	"errors"
	"testing"

	wksandbox "github.com/jiujuan/wukong/pkg/sandbox"
	"github.com/jiujuan/wukong/pkg/skillruntime"
)

func TestRunnerWithoutExecutorRefusesScripts(t *testing.T) {
	runner := NewRunner()
	script := skillruntime.SkillResource{Kind: "script", Path: "scripts/run.sh"}

	if runner.CanRun(script) {
		t.Fatal("CanRun() = true, want false without executor")
	}

	_, err := runner.Run(context.Background(), script, skillruntime.SkillInput{}, skillruntime.ToolPolicy{})
	if !errors.Is(err, skillruntime.ErrScriptRunnerUnsupported) {
		t.Fatalf("Run() error = %v, want ErrScriptRunnerUnsupported", err)
	}
}

func TestRunnerRejectsUnsupportedScriptKind(t *testing.T) {
	runner := NewRunner(WithExecutor(fakeExecutor{}))
	script := skillruntime.SkillResource{Kind: "reference", Path: "references/guide.md"}

	if runner.CanRun(script) {
		t.Fatal("CanRun() = true, want false for non-script resource")
	}
	_, err := runner.Run(context.Background(), script, skillruntime.SkillInput{}, skillruntime.ToolPolicy{})
	if !errors.Is(err, skillruntime.ErrScriptRunnerUnsupported) {
		t.Fatalf("Run() error = %v, want ErrScriptRunnerUnsupported", err)
	}
}

type fakeExecutor struct{}

func (fakeExecutor) Execute(ctx context.Context, req wksandbox.Request) (wksandbox.Result, error) {
	if err := ctx.Err(); err != nil {
		return wksandbox.Result{}, err
	}
	return wksandbox.Result{Stdout: "ok"}, nil
}
