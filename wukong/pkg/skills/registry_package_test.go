package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryLoadsLocalVendorAndLegacyPackages(t *testing.T) {
	root := t.TempDir()

	mustWriteSkillPackage(t, filepath.Join(root, "local", "local_echo"), skillPackageSpec{
		name:   "local_echo",
		tools:  []string{"llm_chat"},
		exec:   "run.sh",
		script: "#!/usr/bin/env bash\nprintf 'local-ok'\n",
	})
	mustWriteSkillPackage(t, filepath.Join(root, "vendor", "vendor_echo"), skillPackageSpec{
		name:     "vendor_echo",
		tools:    []string{"file_write"},
		exec:     "scripts/run.sh",
		script:   "#!/usr/bin/env bash\nprintf 'vendor-ok'\n",
		manifest: `{"package_name":"vendor_echo","version":"0.1.0","homepage":"https://example.com","runtime":"bash","entry":"scripts/run.sh","permissions":{"tools":["file_write"],"network":false}}`,
	})
	mustWriteSkillPackage(t, filepath.Join(root, "legacy_echo"), skillPackageSpec{
		name:   "legacy_echo",
		tools:  []string{"memory_write"},
		exec:   "run.sh",
		script: "#!/usr/bin/env bash\nprintf 'legacy-ok'\n",
	})

	r := New(WithRootDir(root))
	if err := r.reload(context.Background()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	for _, name := range []string{"local_echo", "vendor_echo", "legacy_echo"} {
		if _, ok := r.Get(name); !ok {
			t.Fatalf("skill %q not loaded", name)
		}
	}

	vendor, _ := r.Get("vendor_echo")
	if vendor.Package.SourceType != SourceVendor {
		t.Fatalf("vendor source type = %q, want vendor", vendor.Package.SourceType)
	}
	if vendor.Package.Version != "0.1.0" {
		t.Fatalf("vendor version = %q, want 0.1.0", vendor.Package.Version)
	}
	if vendor.Package.Runtime != "bash" {
		t.Fatalf("vendor runtime = %q, want bash", vendor.Package.Runtime)
	}
	if vendor.Package.Entry != "scripts/run.sh" {
		t.Fatalf("vendor entry = %q, want scripts/run.sh", vendor.Package.Entry)
	}
	if vendor.Package.ManifestPath == "" {
		t.Fatalf("vendor manifest path should be set")
	}

	legacy, _ := r.Get("legacy_echo")
	if legacy.Package.SourceType != SourceLegacy {
		t.Fatalf("legacy source type = %q, want legacy", legacy.Package.SourceType)
	}
	if !r.CanUseTool("vendor_echo", "FILE_WRITE") {
		t.Fatalf("tool whitelist should be case insensitive")
	}
}

func TestLoadSkillPackageRejectsOutOfRootEntry(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "escape_skill")
	mustWriteSkillPackage(t, skillDir, skillPackageSpec{
		name:   "escape_skill",
		tools:  []string{"llm_chat"},
		exec:   filepath.Join("..", "escape.sh"),
		script: "#!/usr/bin/env bash\nprintf 'escape'\n",
	})

	_, err := loadSkillPackage(filepath.Join(skillDir, "SKILL.md"), SourceVendor)
	if err == nil {
		t.Fatalf("expected out-of-root entry to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "outside root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSkillPackageRejectsInvalidRuntime(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "invalid_runtime")
	mustWriteSkillPackage(t, skillDir, skillPackageSpec{
		name:     "invalid_runtime",
		tools:    []string{"llm_chat"},
		exec:     "run.sh",
		script:   "#!/usr/bin/env bash\nprintf 'bad'\n",
		manifest: `{"package_name":"invalid_runtime","runtime":"brainfuck","entry":"run.sh","permissions":{"tools":["llm_chat"]}}`,
	})

	_, err := loadSkillPackage(filepath.Join(skillDir, "SKILL.md"), SourceVendor)
	if err == nil {
		t.Fatalf("expected invalid runtime to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "runtime not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSkillPackageRejectsInvalidTool(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "invalid_tool")
	mustWriteSkillPackage(t, skillDir, skillPackageSpec{
		name:     "invalid_tool",
		tools:    []string{"llm_chat", "danger_tool"},
		exec:     "run.sh",
		script:   "#!/usr/bin/env bash\nprintf 'bad'\n",
		manifest: `{"package_name":"invalid_tool","runtime":"bash","entry":"run.sh","permissions":{"tools":["llm_chat","danger_tool"]}}`,
	})

	_, err := loadSkillPackage(filepath.Join(skillDir, "SKILL.md"), SourceVendor)
	if err == nil {
		t.Fatalf("expected invalid tool to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tool not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSkillResources(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "resource_skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir references failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "assets", "images"), 0o755); err != nil {
		t.Fatalf("mkdir assets failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write reference failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "assets", "images", "cover.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}

	references, assets, metadata, err := resolveSkillResources(skillDir)
	if err != nil {
		t.Fatalf("resolveSkillResources failed: %v", err)
	}
	if len(references) != 1 {
		t.Fatalf("references = %#v", references)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %#v", assets)
	}
	if metadata["references_count"] != 1 || metadata["assets_count"] != 1 {
		t.Fatalf("metadata counts mismatch: %#v", metadata)
	}
	if !strings.HasPrefix(references[0], skillDir) || !strings.HasPrefix(assets[0], skillDir) {
		t.Fatalf("resource paths should stay inside skill dir: %#v %#v", references, assets)
	}
}

func TestResolveSkillResourcePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	escape := filepath.Join(root, "..", "escape.txt")
	if _, err := resolveSkillResourcePath(root, escape); err == nil {
		t.Fatalf("expected escape path to be rejected")
	}
}

type skillPackageSpec struct {
	name     string
	tools    []string
	exec     string
	script   string
	manifest string
}

func mustWriteSkillPackage(t *testing.T, dir string, spec skillPackageSpec) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	skillFile := filepath.Join(dir, "SKILL.md")
	var b strings.Builder
	b.WriteString("# Skill: " + spec.name + "\n")
	b.WriteString("## Description\n")
	b.WriteString("test skill\n")
	b.WriteString("## Tools\n")
	for _, tool := range spec.tools {
		b.WriteString("- " + tool + "\n")
	}
	if strings.TrimSpace(spec.exec) != "" {
		b.WriteString("## Execute\n")
		b.WriteString("- " + spec.exec + "\n")
	}
	if err := os.WriteFile(skillFile, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	if strings.TrimSpace(spec.script) != "" && strings.TrimSpace(spec.exec) != "" {
		scriptPath := filepath.Join(dir, spec.exec)
		if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
			t.Fatalf("mkdir script dir failed: %v", err)
		}
		if err := os.WriteFile(scriptPath, []byte(spec.script), 0o644); err != nil {
			t.Fatalf("write script failed: %v", err)
		}
	}
	if strings.TrimSpace(spec.manifest) != "" {
		if err := os.WriteFile(filepath.Join(dir, "wukong.skill.json"), []byte(spec.manifest), 0o644); err != nil {
			t.Fatalf("write manifest failed: %v", err)
		}
	}
}
