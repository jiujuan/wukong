package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseSkillFile(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, "SKILL.md")
	content := strings.Join([]string{
		"# Skill: Web Search",
		"## Description",
		"用于联网搜索并总结结果",
		"## Params",
		"- query: string(必填, 默认值 golang)",
		"## Tools",
		"- web_search",
		"- llm_chat",
		"## Execute",
		"- run.ps1",
		"## Template",
		"- search_template.md",
		"## Memory Config",
		"- memory_type: working",
		"- window_size: 8",
		"- compress_switch: true",
		"- rag_collection: web_index",
		"- expire_time: 24h",
	}, "\n")
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	item, err := parseSkillFile(skillPath, "web_search")
	if err != nil {
		t.Fatalf("parse skill file failed: %v", err)
	}
	if item.SkillName != "web_search" {
		t.Fatalf("unexpected skill name: %s", item.SkillName)
	}
	if item.Execute != "run.ps1" {
		t.Fatalf("unexpected execute: %s", item.Execute)
	}
	if len(item.Params) != 1 || item.Params[0].Name != "query" || !item.Params[0].Required {
		t.Fatalf("unexpected params: %+v", item.Params)
	}
	if item.Memory.WindowSize != 8 || item.Memory.MemoryType != "working" || !item.Memory.CompressSwitch {
		t.Fatalf("unexpected memory config: %+v", item.Memory)
	}
}

func TestRegistryReloadAndCanUseTool(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "web_search")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := strings.Join([]string{
		"# Skill: web_search",
		"## Description",
		"搜索技能",
		"## Tools",
		"- web_search",
		"- http_request",
		"## Execute",
		"- run.ps1",
	}, "\n")
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	r := New(WithRootDir(root))
	if err := r.reload(context.Background()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if _, ok := r.Get("web_search"); !ok {
		t.Fatalf("custom skill not loaded")
	}
	if _, ok := r.Get("chat"); !ok {
		t.Fatalf("builtin skill not loaded")
	}
	if !r.CanUseTool("web_search", "WEB_SEARCH") {
		t.Fatalf("tool whitelist should be case-insensitive")
	}
	if r.CanUseTool("web_search", "file_write") {
		t.Fatalf("unexpected tool allowed")
	}
}

func TestExecuteWithParams(t *testing.T) {
	root := t.TempDir()
	skillName := "echo_skill"
	skillDir := filepath.Join(root, skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	scriptName := "run.sh"
	scriptBody := "#!/usr/bin/env bash\nprintf \"%s|%s\\n\" \"$SKILL_NAME\" \"$SKILL_PARAMS\"\n"
	if runtime.GOOS == "windows" {
		scriptName = "run.ps1"
		scriptBody = `Write-Output "$env:SKILL_NAME|$env:SKILL_PARAMS"`
	}
	scriptPath := filepath.Join(skillDir, scriptName)
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	content := strings.Join([]string{
		"# Skill: echo_skill",
		"## Description",
		"echo env",
		"## Tools",
		"- llm_chat",
		"## Execute",
		"- " + scriptName,
	}, "\n")
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	r := New(WithRootDir(root))
	if err := r.reload(context.Background()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	result, err := r.ExecuteWithParams(context.Background(), skillName, map[string]any{
		"query": "golang",
	})
	if err != nil {
		t.Fatalf("execute with params failed: %v", err)
	}
	output := strings.TrimSpace(result["output"].(string))
	stdout := strings.TrimSpace(result["stdout"].(string))
	if output == "" && stdout == "" {
		t.Fatalf("expected non-empty execution output: %#v", result)
	}
	if result["skill_name"] != skillName {
		t.Fatalf("unexpected skill name: %#v", result["skill_name"])
	}
	if strings.TrimSpace(result["skill_root"].(string)) == "" {
		t.Fatalf("skill root should not be empty: %#v", result)
	}
	if strings.TrimSpace(result["output_dir"].(string)) == "" {
		t.Fatalf("output dir should not be empty: %#v", result)
	}
	if result["package"] == nil {
		t.Fatalf("package metadata should not be nil: %#v", result)
	}
}

func TestRegistryAdapterPriority(t *testing.T) {
	dir := t.TempDir()
	skillFile := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	r := New()
	r.adapters = nil
	r.RegisterAdapter(testAdapter{
		match: true,
		parse: func(path string) (*Skill, error) {
			return &Skill{SkillName: "first", Enabled: true}, nil
		},
	})
	r.RegisterAdapter(testAdapter{
		match: true,
		parse: func(path string) (*Skill, error) {
			return &Skill{SkillName: "second", Enabled: true}, nil
		},
	})

	got, err := r.parseSkillWithAdapters(skillFile)
	if err != nil {
		t.Fatalf("parseSkillWithAdapters failed: %v", err)
	}
	if got.SkillName != "first" {
		t.Fatalf("skill name = %q, want first", got.SkillName)
	}
}

func TestRegistryDuplicateSkillNameKeepsFirstLoaded(t *testing.T) {
	root := t.TempDir()
	mustWriteSkillPackage(t, filepath.Join(root, "local", "shared_skill"), skillPackageSpec{
		name:   "shared_skill",
		tools:  []string{"llm_chat"},
		exec:   "run.sh",
		script: "#!/usr/bin/env bash\nprintf 'local'\n",
	})
	mustWriteSkillPackage(t, filepath.Join(root, "vendor", "shared_skill"), skillPackageSpec{
		name:     "shared_skill",
		tools:    []string{"file_write"},
		exec:     "run.sh",
		script:   "#!/usr/bin/env bash\nprintf 'vendor'\n",
		manifest: `{"package_name":"shared_skill","runtime":"bash","entry":"run.sh","permissions":{"tools":["file_write"]}}`,
	})

	r := New(WithRootDir(root))
	if err := r.reload(context.Background()); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	got, ok := r.Get("shared_skill")
	if !ok {
		t.Fatalf("shared skill not loaded")
	}
	if got.Package.SourceType != SourceLocal {
		t.Fatalf("source type = %q, want local", got.Package.SourceType)
	}
	if !r.CanUseTool("shared_skill", "llm_chat") {
		t.Fatalf("local skill tools should win")
	}
	if r.CanUseTool("shared_skill", "file_write") {
		t.Fatalf("vendor duplicate should be ignored")
	}
}

type testAdapter struct {
	match bool
	parse func(path string) (*Skill, error)
}

func (a testAdapter) Match(path string, content []byte) bool { return a.match }

func (a testAdapter) Parse(path string) (*Skill, error) {
	if a.parse == nil {
		return nil, nil
	}
	return a.parse(path)
}
