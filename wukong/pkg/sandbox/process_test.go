package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateAllowedCommand(t *testing.T) {
	policy := Policy{AllowedCommands: map[string]struct{}{"go": {}}}

	if err := validateAllowedCommand(policy, "go"); err != nil {
		t.Fatalf("validateAllowedCommand(go) error = %v", err)
	}
	if err := validateAllowedCommand(policy, "whoami"); err == nil || !strings.Contains(err.Error(), ErrCommandNotAllowed.Error()) {
		t.Fatalf("validateAllowedCommand(whoami) error = %v, want command not allowed", err)
	}
	if err := validateAllowedCommand(policy, ""); err == nil || !strings.Contains(err.Error(), ErrInvalidRequest.Error()) {
		t.Fatalf("validateAllowedCommand(empty) error = %v, want invalid request", err)
	}
}

func TestBuildCommandEnvFiltersAndOverrides(t *testing.T) {
	t.Setenv("PATH", "system-path")
	t.Setenv("SECRET", "should-not-pass")

	env := buildCommandEnv(Policy{
		AllowedEnvKeys: map[string]struct{}{
			"path": {},
			"foo":  {},
		},
	}, map[string]string{
		"FOO":    "override",
		"SECRET": "drop",
	})

	got := make(map[string]string)
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			t.Fatalf("invalid env item: %q", item)
		}
		got[key] = value
	}
	if got["PATH"] != "system-path" {
		t.Fatalf("PATH = %q, want system-path", got["PATH"])
	}
	if got["FOO"] != "override" {
		t.Fatalf("FOO = %q, want override", got["FOO"])
	}
	if _, ok := got["SECRET"]; ok {
		t.Fatalf("SECRET should be filtered out: %#v", got)
	}
}

func TestFilterAllowedEnv(t *testing.T) {
	got := filterAllowedEnv(map[string]string{
		"PATH":   "keep",
		"HOME":   "drop",
		"SECRET": "drop",
	}, map[string]struct{}{
		"path": {},
	})
	if len(got) != 1 || got["PATH"] != "keep" {
		t.Fatalf("filterAllowedEnv() = %#v", got)
	}
}

func TestWithinAllowedRoots(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "nested", "file.txt")
	outside := filepath.Join(t.TempDir(), "other.txt")

	if !withinAllowedRoots(inside, []string{root}) {
		t.Fatalf("inside path should be allowed")
	}
	if withinAllowedRoots(outside, []string{root}) {
		t.Fatalf("outside path should not be allowed")
	}
}

func TestOutputCollectorTruncates(t *testing.T) {
	collector := newOutputCollector(5, nil)
	if _, err := collector.stdoutWriter().Write([]byte("123456789")); err == nil || !strings.Contains(err.Error(), ErrOutputLimitExceeded.Error()) {
		t.Fatalf("Write() error = %v, want output limit exceeded", err)
	}
	if !collector.truncated {
		t.Fatal("collector.truncated = false, want true")
	}
	if got := collector.stdout.String(); got != "12345" {
		t.Fatalf("stdout = %q, want 12345", got)
	}
}

func TestRunProcessCommandSuccess(t *testing.T) {
	if _, ok := lookPathAny("go"); !ok {
		t.Skip("go runtime not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "main.go")
	if err := os.WriteFile(script, []byte(`package main

import "fmt"

func main() {
	fmt.Println("process-ok")
}
`), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	result, err := runProcess(context.Background(), Request{
		Runtime:    "go",
		Command:    "go",
		Args:       []string{"run", script},
		ScriptPath: script,
		WorkDir:    dir,
		Env: map[string]string{
			"GOCACHE": filepath.Join(dir, "gocache"),
		},
	}, Policy{
		AllowedCommands:  defaultAllowedCommands(),
		AllowedEnvKeys:   defaultAllowedEnvKeys(),
		AllowedWorkRoots: []string{dir},
		DefaultTimeout:   10 * time.Second,
		MaxOutputBytes:   1 << 20,
	})
	if err != nil {
		t.Fatalf("runProcess() error = %v, stdout=%q stderr=%q", err, result.Stdout, result.Stderr)
	}
	if !strings.Contains(strings.TrimSpace(result.Stdout), "process-ok") {
		t.Fatalf("stdout = %q, want process-ok", result.Stdout)
	}
}

func TestRunProcessTimeout(t *testing.T) {
	if _, ok := lookPathAny("go"); !ok {
		t.Skip("go runtime not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "main.go")
	if err := os.WriteFile(script, []byte(`package main

import "time"

func main() {
	time.Sleep(2 * time.Second)
}
`), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := runProcess(ctx, Request{
		Runtime:    "go",
		Command:    "go",
		Args:       []string{"run", script},
		ScriptPath: script,
		WorkDir:    dir,
		Env: map[string]string{
			"GOCACHE": filepath.Join(dir, "gocache"),
		},
	}, Policy{
		AllowedCommands:  defaultAllowedCommands(),
		AllowedEnvKeys:   defaultAllowedEnvKeys(),
		AllowedWorkRoots: []string{dir},
		DefaultTimeout:   10 * time.Second,
		MaxOutputBytes:   1 << 20,
	})
	if err == nil {
		t.Fatalf("runProcess() error = nil, result=%#v", result)
	}
}
