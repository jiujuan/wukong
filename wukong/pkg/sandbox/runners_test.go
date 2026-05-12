package sandbox

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterBuiltinsRegistersCoreRunners(t *testing.T) {
	s := New()
	for _, runtime := range []string{
		"command",
		"python",
		"javascript",
		"bash",
		"powershell",
		"go",
		"java",
		"typescript",
	} {
		if s.getRunner(runtime) == nil {
			t.Fatalf("runner %q not registered", runtime)
		}
	}
}

func TestScriptCommandPlanRequiresScriptPath(t *testing.T) {
	plan := scriptCommandPlan("go")
	_, err := plan(context.Background(), Request{})
	if err == nil || !strings.Contains(err.Error(), "script path is required") {
		t.Fatalf("scriptCommandPlan error = %v, want script path is required", err)
	}
}

func TestScriptCommandPlanResolvesCommand(t *testing.T) {
	if _, ok := lookPathAny("go"); !ok {
		t.Skip("go runtime not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "main.go")
	plan := scriptCommandPlan("go")
	got, err := plan(context.Background(), Request{ScriptPath: script})
	if err != nil {
		t.Fatalf("scriptCommandPlan() error = %v", err)
	}
	if got.command == "" {
		t.Fatal("plan.command is empty")
	}
	if len(got.args) != 1 || got.args[0] != script {
		t.Fatalf("plan.args = %#v, want [%s]", got.args, script)
	}
}

func TestGoPlanInjectsCacheEnv(t *testing.T) {
	if _, ok := lookPathAny("go"); !ok {
		t.Skip("go runtime not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "main.go")
	got, err := goPlan(context.Background(), Request{ScriptPath: script})
	if err != nil {
		t.Fatalf("goPlan() error = %v", err)
	}
	if got.command == "" {
		t.Fatal("goPlan command is empty")
	}
	if got.env == nil || strings.TrimSpace(got.env["GOCACHE"]) == "" {
		t.Fatalf("goPlan env missing GOCACHE: %#v", got.env)
	}
}

func TestLocateTypeScriptCommandMissingRuntime(t *testing.T) {
	t.Setenv("PATH", "")
	if _, _, err := locateTypeScriptCommand("main.ts"); err == nil || !strings.Contains(err.Error(), "typescript runtime not found") {
		t.Fatalf("locateTypeScriptCommand() error = %v, want typescript runtime not found", err)
	}
}

func TestFindPowerShellMissingRuntime(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := findPowerShell(); err == nil || !strings.Contains(err.Error(), "powershell runtime not found") {
		t.Fatalf("findPowerShell() error = %v, want runtime not found", err)
	}
}
