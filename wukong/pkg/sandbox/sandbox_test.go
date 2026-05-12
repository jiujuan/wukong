package sandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateCommandWhitelist(t *testing.T) {
	s := New(WithPolicy(Policy{
		AllowedCommands: map[string]struct{}{
			"go": {},
		},
	}))

	if err := s.Validate(Request{Runtime: "command", Command: "go"}); err != nil {
		t.Fatalf("Validate(go) error = %v", err)
	}
	if err := s.Validate(Request{Runtime: "command", Command: "whoami"}); err == nil || !errors.Is(err, ErrCommandNotAllowed) {
		t.Fatalf("Validate(whoami) error = %v, want ErrCommandNotAllowed", err)
	}
}

func TestWorkDirRestriction(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	script := filepath.Join(other, "main.go")
	if err := os.WriteFile(script, []byte(`package main
func main() {}
`), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	s := New(WithPolicy(Policy{
		AllowedCommands:  defaultAllowedCommands(),
		AllowedWorkRoots: []string{root},
	}))

	err := s.Validate(Request{Runtime: "go", ScriptPath: script})
	if err == nil || !errors.Is(err, ErrWorkDirNotAllowed) {
		t.Fatalf("Validate() error = %v, want ErrWorkDirNotAllowed", err)
	}
}

func TestEnvFiltering(t *testing.T) {
	got := filterAllowedEnv(map[string]string{
		"PATH":      "keep",
		"SECRET":    "drop",
		"SKILL_ENV": "drop",
	}, map[string]struct{}{
		"path": {},
	})
	if len(got) != 1 || got["PATH"] != "keep" {
		t.Fatalf("filterAllowedEnv() = %#v", got)
	}
}

func TestOutputCollectorTruncatesIntegration(t *testing.T) {
	collector := newOutputCollector(5, nil)
	if _, err := collector.stdoutWriter().Write([]byte("1234567890")); !errors.Is(err, ErrOutputLimitExceeded) {
		t.Fatalf("Write() error = %v, want ErrOutputLimitExceeded", err)
	}
	if !collector.truncated {
		t.Fatal("collector.truncated = false, want true")
	}
	if got := collector.stdout.String(); got != "12345" {
		t.Fatalf("stdout = %q, want 12345", got)
	}
}

func TestProcessRunnerTimeout(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go runtime not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "main.go")
	if err := os.WriteFile(script, []byte(`package main
import (
	"fmt"
	"time"
)
func main() {
	time.Sleep(2 * time.Second)
	fmt.Println("done")
}
`), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	s := New(WithPolicy(Policy{
		AllowedCommands:  defaultAllowedCommands(),
		AllowedWorkRoots: []string{dir},
		DefaultTimeout:   100 * time.Millisecond,
	}))

	start := time.Now()
	_, err := s.Execute(context.Background(), Request{Runtime: "go", ScriptPath: script, Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Fatal("Execute() error = nil, want timeout")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("timeout took too long: %s", time.Since(start))
	}
}

func TestRunnersExecuteIfAvailable(t *testing.T) {
	type testcase struct {
		name       string
		runtime    string
		scriptName string
		content    string
		need       func() bool
		want       string
	}
	tests := []testcase{
		{
			name:       "python",
			runtime:    "python",
			scriptName: "main.py",
			content:    `print("python-ok")`,
			need: func() bool {
				_, ok := lookPathAny("python", "python3", "py")
				return ok
			},
			want: "python-ok",
		},
		{
			name:       "bash",
			runtime:    "bash",
			scriptName: "main.sh",
			content:    `echo bash-ok`,
			need: func() bool {
				if runtime.GOOS == "windows" {
					return false
				}
				_, ok := lookPathAny("bash", "sh")
				return ok
			},
			want: "bash-ok",
		},
		{
			name:       "javascript",
			runtime:    "javascript",
			scriptName: "main.js",
			content:    `console.log("js-ok")`,
			need: func() bool {
				_, ok := lookPathAny("node")
				return ok
			},
			want: "js-ok",
		},
		{
			name:       "powershell",
			runtime:    "powershell",
			scriptName: "main.ps1",
			content:    `Write-Output "ps-ok"`,
			need: func() bool {
				_, err := findPowerShell()
				return err == nil
			},
			want: "ps-ok",
		},
		{
			name:       "go",
			runtime:    "go",
			scriptName: "main.go",
			content: `package main
import "fmt"
func main() { fmt.Println("go-ok") }`,
			need: func() bool {
				_, ok := lookPathAny("go")
				return ok
			},
			want: "go-ok",
		},
		{
			name:       "java",
			runtime:    "java",
			scriptName: "Main.java",
			content: `public class Main {
  public static void main(String[] args) {
    System.out.println("java-ok");
  }
}`,
			need: func() bool {
				_, ok := lookPathAny("javac")
				if !ok {
					return false
				}
				_, ok = lookPathAny("java")
				return ok
			},
			want: "java-ok",
		},
		{
			name:       "typescript",
			runtime:    "typescript",
			scriptName: "main.ts",
			content:    `console.log("ts-ok")`,
			need: func() bool {
				_, ok := lookPathAny("tsx", "ts-node", "deno")
				return ok
			},
			want: "ts-ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.need != nil && !tt.need() {
				t.Skip("runtime not found")
			}
			dir := t.TempDir()
			script := filepath.Join(dir, tt.scriptName)
			if err := os.WriteFile(script, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write script failed: %v", err)
			}
			s := New(WithPolicy(Policy{
				AllowedCommands:  defaultAllowedCommands(),
				AllowedWorkRoots: []string{dir},
			}))
			result, err := s.Execute(context.Background(), Request{
				Runtime:    tt.runtime,
				ScriptPath: script,
				WorkDir:    dir,
				Timeout:    10 * time.Second,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v; stdout=%q stderr=%q", err, result.Stdout, result.Stderr)
			}
			output := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
			if !strings.Contains(output, tt.want) {
				t.Fatalf("output = %q, want to contain %q", output, tt.want)
			}
		})
	}
}

func TestCommandRuntimeExec(t *testing.T) {
	if _, ok := lookPathAny("go"); !ok {
		t.Skip("go runtime not found")
	}
	s := New(WithPolicy(Policy{
		AllowedCommands: defaultAllowedCommands(),
	}))
	result, err := s.Execute(context.Background(), Request{
		Runtime: "command",
		Command: "go",
		Args:    []string{"version"},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout=%q stderr=%q", err, result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout+result.Stderr) == "" {
		t.Fatal("expected command output")
	}
}
