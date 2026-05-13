package skills

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveRuntimeEntryKeepsCommonRuntimes(t *testing.T) {
	tests := []struct {
		name     string
		runtime  string
		execute  string
		wantRun  string
		wantEntr string
	}{
		{name: "python", runtime: "python", execute: "main.py", wantRun: "python", wantEntr: "main.py"},
		{name: "bash", runtime: "bash", execute: "run.sh", wantRun: "bash", wantEntr: "run.sh"},
		{name: "node", runtime: "javascript", execute: "main.js", wantRun: "javascript", wantEntr: "main.js"},
		{name: "ts", runtime: "typescript", execute: "main.ts", wantRun: "typescript", wantEntr: "main.ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := &Skill{
				SkillName: "demo",
				Execute:   tt.execute,
				Package: PackageMeta{
					Runtime: tt.runtime,
					Entry:   tt.execute,
					RootDir: t.TempDir(),
				},
			}
			gotRuntime, gotEntry, err := resolveRuntimeEntry(skill)
			if err != nil {
				t.Fatalf("resolveRuntimeEntry() error = %v", err)
			}
			if gotRuntime != tt.wantRun {
				t.Fatalf("runtime = %q, want %q", gotRuntime, tt.wantRun)
			}
			if gotEntry != tt.wantEntr {
				t.Fatalf("entry = %q, want %q", gotEntry, tt.wantEntr)
			}
		})
	}
}

func TestResolveRuntimeEntryDerivesRuntimeFromEntry(t *testing.T) {
	skill := &Skill{
		SkillName: "demo",
		Execute:   "scripts/main.py",
		Package: PackageMeta{
			RootDir: t.TempDir(),
		},
	}
	gotRuntime, gotEntry, err := resolveRuntimeEntry(skill)
	if err != nil {
		t.Fatalf("resolveRuntimeEntry() error = %v", err)
	}
	if gotRuntime != "python" {
		t.Fatalf("runtime = %q, want python", gotRuntime)
	}
	if gotEntry != "scripts/main.py" {
		t.Fatalf("entry = %q, want scripts/main.py", gotEntry)
	}
}

func TestExecuteWithSkillUsesSkillOutputDirAndEnv(t *testing.T) {
	runtimeName, scriptName, scriptBody := runtimeFixture()
	if runtimeName == "" {
		t.Skip("no supported runtime found")
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	workdir := t.TempDir()
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	skillDir := filepath.Join(workdir, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	scriptPath := filepath.Join(skillDir, scriptName)
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	skill := &Skill{
		SkillName:   "report_gen",
		Description: "generate report",
		Version:     "1.0.0",
		Enabled:     true,
		Execute:     scriptName,
		Tools:       []string{"file_write"},
		References:  []string{filepath.Join(skillDir, "references", "guide.md")},
		Assets:      []string{filepath.Join(skillDir, "assets", "cover.png")},
		Package: PackageMeta{
			SourceType:  SourceVendor,
			PackageName: "report_gen",
			Runtime:     runtimeName,
			Entry:       scriptName,
			RootDir:     skillDir,
		},
		Metadata: map[string]any{"kind": "report"},
	}

	result, err := New().ExecuteWithSkill(context.Background(), skill, map[string]any{"title": "demo"})
	if err != nil {
		t.Fatalf("ExecuteWithSkill failed: %v, result=%#v", err, result)
	}
	outputDir, _ := result["output_dir"].(string)
	if outputDir == "" {
		t.Fatalf("output_dir is empty")
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Fatalf("output dir does not exist: %v", err)
	}
	output := strings.TrimSpace(result["output"].(string))
	if !strings.Contains(output, "report_gen") {
		t.Fatalf("output = %q, want skill name", output)
	}
	if !strings.Contains(output, outputDir) {
		t.Fatalf("output = %q, want output dir %q", output, outputDir)
	}
}

func runtimeFixture() (string, string, string) {
	switch {
	case hasPythonRuntime():
		return "python", "main.py", `import os
print(os.getenv("SKILL_NAME", ""))
print(os.getenv("WUKONG_OUTPUT_DIR", ""))
`
	case hasNodeRuntime():
		return "javascript", "main.js", `console.log(process.env.SKILL_NAME || "")
console.log(process.env.WUKONG_OUTPUT_DIR || "")
`
	case hasBashRuntime():
		return "bash", "main.sh", `echo "${SKILL_NAME}"
echo "${WUKONG_OUTPUT_DIR}"
`
	default:
		return "", "", ""
	}
}

func hasPythonRuntime() bool {
	return hasAnyExecutable("python", "python3", "py")
}

func hasNodeRuntime() bool {
	return hasAnyExecutable("node")
}

func hasBashRuntime() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return hasAnyExecutable("bash", "sh")
}

func hasAnyExecutable(names ...string) bool {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}
