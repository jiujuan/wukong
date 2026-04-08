package skills

import (
	"context"
	"os"
	"path/filepath"
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
	psScript := filepath.Join(skillDir, "run.ps1")
	if err := os.WriteFile(psScript, []byte(`Write-Output "$env:SKILL_NAME|$env:SKILL_PARAMS"`), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	content := strings.Join([]string{
		"# Skill: echo_skill",
		"## Description",
		"echo env",
		"## Tools",
		"- llm_chat",
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

	result, err := r.ExecuteWithParams(context.Background(), skillName, map[string]any{
		"query": "golang",
	})
	if err != nil {
		t.Fatalf("execute with params failed: %v", err)
	}
	output := strings.TrimSpace(result["output"].(string))
	if !strings.Contains(output, "echo_skill|") {
		t.Fatalf("unexpected output: %s", output)
	}
	if !strings.Contains(output, `"query":"golang"`) {
		t.Fatalf("params not injected in output: %s", output)
	}
}
