package agentspec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jiujuan/wukong/pkg/skillruntime"
)

func TestRuntimeDiscoverFindsMultipleSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "paper"), "paper_reader", "Read papers", "Read WebSearch", "# Paper\nFull body")
	writeSkill(t, filepath.Join(root, "writer"), "writer", "Write reports", "Write", "# Writer\nFull body")

	runtime := NewRuntime(WithRoots(root))
	manifests, err := runtime.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(manifests) != 2 {
		t.Fatalf("Discover() returned %d manifests, want 2: %#v", len(manifests), manifests)
	}
	if manifests[0].Name != "paper_reader" || manifests[1].Name != "writer" {
		t.Fatalf("Discover() manifests = %#v", manifests)
	}
	if manifests[0].Runtime != RuntimeName {
		t.Fatalf("Runtime = %q, want %q", manifests[0].Runtime, RuntimeName)
	}
}

func TestRuntimeDiscoverCachesOnlyManifestAndGetLoadsBody(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "paper"), "paper_reader", "Read papers", "Read", "# Paper\nFull body")

	runtime := NewRuntime(WithRoots(root))
	manifests, err := runtime.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if manifests[0].Description != "Read papers" {
		t.Fatalf("manifest = %#v", manifests[0])
	}

	spec, err := runtime.Get(context.Background(), "paper_reader")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if spec.Instructions != "# Paper\nFull body" {
		t.Fatalf("Instructions = %q, want full body", spec.Instructions)
	}
}

func TestRuntimePrepareBuildsPolicyContextAndResources(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "paper")
	writeSkill(t, dir, "paper_reader", "Read papers", "Read WebSearch Bash(git:*)", "# Paper\nFull body")
	writeFile(t, filepath.Join(dir, "scripts", "parse.py"), "print('ok')")
	writeFile(t, filepath.Join(dir, "references", "guide.md"), "guide")
	writeFile(t, filepath.Join(dir, "assets", "cover.png"), "png")

	runtime := NewRuntime(WithRoots(root))
	prepared, err := runtime.Prepare(context.Background(), skillruntime.SkillActivation{
		SkillName: "paper_reader",
	}, skillruntime.RuntimeContext{
		Caller: skillruntime.Caller{Type: skillruntime.CallerTypeAgent, ID: "agent-1"},
		TaskID: "task-1",
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	wantAllowed := []string{skillruntime.ToolFileRead, skillruntime.ToolWebSearch, skillruntime.ToolCodeExec}
	if !reflect.DeepEqual(prepared.ToolPolicy.AllowedTools, wantAllowed) {
		t.Fatalf("AllowedTools = %#v, want %#v", prepared.ToolPolicy.AllowedTools, wantAllowed)
	}
	if prepared.ContextBlocks[0].Name != "skill_instructions" || prepared.ContextBlocks[0].Content != "# Paper\nFull body" {
		t.Fatalf("ContextBlocks = %#v", prepared.ContextBlocks)
	}
	if len(prepared.Spec.Scripts) != 1 || prepared.Spec.Scripts[0].Path != "scripts/parse.py" {
		t.Fatalf("Scripts = %#v", prepared.Spec.Scripts)
	}
	if len(prepared.Spec.References) != 1 || prepared.Spec.References[0].Path != "references/guide.md" {
		t.Fatalf("References = %#v", prepared.Spec.References)
	}
	if len(prepared.Spec.Assets) != 1 || prepared.Spec.Assets[0].Path != "assets/cover.png" {
		t.Fatalf("Assets = %#v", prepared.Spec.Assets)
	}
	if len(prepared.Resources) != 3 {
		t.Fatalf("Resources = %#v, want scripts + references + assets", prepared.Resources)
	}
	if prepared.ExecutionMode != skillruntime.SkillExecutionModeContextOnly {
		t.Fatalf("ExecutionMode = %q", prepared.ExecutionMode)
	}
}

func TestRuntimeMatchUsesSkillNameActionTagsAndText(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "paper"), "paper_reader", "Read academic papers", "Read", "# Paper")
	writeSkill(t, filepath.Join(root, "writer"), "writer", "Write reports", "Write", "# Writer")

	runtime := NewRuntime(WithRoots(root))
	candidates, err := runtime.Match(context.Background(), skillruntime.SkillMatchRequest{SkillName: "writer"})
	if err != nil {
		t.Fatalf("Match(skill name) error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Manifest.Name != "writer" || candidates[0].Score != 1.0 {
		t.Fatalf("skill name candidates = %#v", candidates)
	}

	candidates, err = runtime.Match(context.Background(), skillruntime.SkillMatchRequest{Query: "academic"})
	if err != nil {
		t.Fatalf("Match(query) error = %v", err)
	}
	if len(candidates) == 0 || candidates[0].Manifest.Name != "paper_reader" {
		t.Fatalf("query candidates = %#v", candidates)
	}
}

func TestRuntimeGetMissingSkill(t *testing.T) {
	runtime := NewRuntime(WithRoots(t.TempDir()))

	_, err := runtime.Get(context.Background(), "missing")
	if !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("Get() error = %v, want ErrSkillNotFound", err)
	}
}

func TestRuntimeExecuteUnsupported(t *testing.T) {
	runtime := NewRuntime()

	_, err := runtime.Execute(context.Background(), nil, skillruntime.SkillInput{})
	if !errors.Is(err, ErrExecuteUnsupported) {
		t.Fatalf("Execute() error = %v, want ErrExecuteUnsupported", err)
	}
}

func writeSkill(t *testing.T, dir, name, description, allowedTools, body string) {
	t.Helper()
	content := `---
name: ` + name + `
description: ` + description + `
allowed-tools: ` + allowedTools + `
tags:
  - test
---
` + body + `
`
	writeFile(t, filepath.Join(dir, SkillFileName), content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
