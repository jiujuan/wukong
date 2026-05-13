package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyMarkdownCanParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	content := []byte(`# Skill: Web Search
## Description
search the web
## Tools
- web_search
- llm_chat
## Execute
- run.sh
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}

	ad := NewLegacyAdapter()
	if !ad.Match(path, content) {
		t.Fatalf("legacy adapter should match")
	}
	skill, err := ad.Parse(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if skill.SkillName != "web_search" {
		t.Fatalf("skill name = %q", skill.SkillName)
	}
	if skill.Package.SourceType != "legacy" {
		t.Fatalf("source type = %q, want legacy", skill.Package.SourceType)
	}
	if skill.Execute != "run.sh" || skill.Package.Runtime != "bash" {
		t.Fatalf("runtime mismatch: %#v", skill.Package)
	}
	if len(skill.Tools) != 2 {
		t.Fatalf("tools = %#v", skill.Tools)
	}
}

func TestAgentSkillsFrontmatterCanParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	content := []byte(`---
name: report_gen
description: generate reports from input
version: 1.0.0
runtime: python
entry: scripts/generate.py
allowed-tools: llm_chat file_write
---

# Report Gen
Use the following instructions to generate reports.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}

	ad := NewAgentSkillsAdapter()
	if !ad.Match(path, content) {
		t.Fatalf("agent skills adapter should match")
	}
	skill, err := ad.Parse(path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if skill.SkillName != "report_gen" {
		t.Fatalf("skill name = %q", skill.SkillName)
	}
	if skill.Description != "generate reports from input" {
		t.Fatalf("description = %q", skill.Description)
	}
	if skill.Package.SourceType != "vendor" {
		t.Fatalf("source type = %q, want vendor", skill.Package.SourceType)
	}
	if skill.Package.Runtime != "python" || skill.Package.Entry != "scripts/generate.py" {
		t.Fatalf("runtime metadata mismatch: %#v", skill.Package)
	}
	if len(skill.Tools) != 2 {
		t.Fatalf("tools = %#v", skill.Tools)
	}
	if skill.Template == "" {
		t.Fatalf("template/body should be preserved")
	}
}

func TestInvalidSkillFileRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	content := []byte(`---
name: broken_skill
version: 1.0.0
---

missing description
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}

	ad := NewAgentSkillsAdapter()
	if ad.Match(path, content) {
		t.Fatalf("invalid frontmatter should not match")
	}
	if _, err := ad.Parse(path); err == nil {
		t.Fatalf("invalid skill should be rejected")
	}
}

func TestAdapterPriorityPrefersAgentSkillsOverLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	content := []byte(`---
name: mixed_skill
description: agent style with legacy sections
allowed-tools: llm_chat
---

# Skill: mixed_skill
## Description
legacy body
## Tools
- file_write
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write skill failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wukong.skill.json"), []byte(`{"package_name":"mixed_skill","runtime":"bash","entry":"run.sh","permissions":{"tools":["file_write"]}}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	adapters := []Adapter{NewAgentSkillsAdapter(), NewVendorAdapter(), NewLegacyAdapter()}
	for _, ad := range adapters {
		if ad.Match(path, content) {
			skill, err := ad.Parse(path)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if skill.Package.ManifestPath != "" {
				t.Fatalf("expected agent style to win before vendor manifest, got manifest: %#v", skill.Package)
			}
			if skill.Template == "" {
				t.Fatalf("agent style should preserve body as template/instructions")
			}
			return
		}
	}
	t.Fatalf("no adapter matched")
}
