package agentspec

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParserParsesValidSkillFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SkillFileName)
	content := []byte(`---
name: paper_reader
description: Read papers for non-academics
version: 1.0.0
license: MIT
compatibility: wukong
allowed-tools: Read WebSearch Bash(git:*)
tags:
  - paper
  - reading
metadata:
  owner: research
  tier: beta
---

# Paper Reader

Extract usable ideas.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write SKILL.md error = %v", err)
	}

	spec, err := NewParser().ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if spec.Manifest.Name != "paper_reader" {
		t.Fatalf("Manifest.Name = %q, want paper_reader", spec.Manifest.Name)
	}
	if spec.Manifest.Description != "Read papers for non-academics" {
		t.Fatalf("Description = %q", spec.Manifest.Description)
	}
	if spec.Manifest.Runtime != RuntimeName {
		t.Fatalf("Runtime = %q, want %q", spec.Manifest.Runtime, RuntimeName)
	}
	if spec.Manifest.License != "MIT" || spec.Manifest.Compatibility != "wukong" {
		t.Fatalf("manifest metadata = %#v", spec.Manifest)
	}
	if !reflect.DeepEqual(spec.AllowedTools, []string{"Read", "WebSearch", "Bash(git:*)"}) {
		t.Fatalf("AllowedTools = %#v", spec.AllowedTools)
	}
	if !reflect.DeepEqual(spec.Manifest.Tags, []string{"paper", "reading"}) {
		t.Fatalf("Tags = %#v", spec.Manifest.Tags)
	}
	if spec.Manifest.Metadata["owner"] != "research" || spec.Metadata["tier"] != "beta" {
		t.Fatalf("Metadata mismatch: manifest=%#v spec=%#v", spec.Manifest.Metadata, spec.Metadata)
	}
	if spec.RootDir != filepath.Clean(dir) {
		t.Fatalf("RootDir = %q, want %q", spec.RootDir, filepath.Clean(dir))
	}
	if spec.Instructions != "# Paper Reader\n\nExtract usable ideas." {
		t.Fatalf("Instructions = %q", spec.Instructions)
	}
}

func TestParserParsesAllowedToolsList(t *testing.T) {
	spec, err := NewParser().Parse("SKILL.md", []byte(`---
name: tool_skill
description: uses tools
allowed_tools:
  - Read
  - Write
  - WebSearch
---
Body
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []string{"Read", "Write", "WebSearch"}
	if !reflect.DeepEqual(spec.AllowedTools, want) {
		t.Fatalf("AllowedTools = %#v, want %#v", spec.AllowedTools, want)
	}
}

func TestParserParseManifestFileReadsFrontmatterOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SkillFileName)
	content := []byte(`---
name: manifest_only
description: manifest parsing
allowed-tools: Read
---

This body is intentionally not parsed by ParseManifestFile.
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write SKILL.md error = %v", err)
	}

	manifest, err := NewParser().ParseManifestFile(path)
	if err != nil {
		t.Fatalf("ParseManifestFile() error = %v", err)
	}
	if manifest.Name != "manifest_only" {
		t.Fatalf("Manifest.Name = %q, want manifest_only", manifest.Name)
	}
	if manifest.Runtime != RuntimeName {
		t.Fatalf("Manifest.Runtime = %q, want %q", manifest.Runtime, RuntimeName)
	}
}

func TestParserRequiresName(t *testing.T) {
	_, err := NewParser().Parse("SKILL.md", []byte(`---
description: missing name
---
Body
`))
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf("Parse() error = %v, want ErrNameRequired", err)
	}
}

func TestParserRequiresDescription(t *testing.T) {
	_, err := NewParser().Parse("SKILL.md", []byte(`---
name: missing_description
---
Body
`))
	if !errors.Is(err, ErrDescriptionRequired) {
		t.Fatalf("Parse() error = %v, want ErrDescriptionRequired", err)
	}
}

func TestParserRequiresFrontmatter(t *testing.T) {
	_, err := NewParser().Parse("SKILL.md", []byte(`# No frontmatter

Body
`))
	if !errors.Is(err, ErrFrontmatterNotFound) {
		t.Fatalf("Parse() error = %v, want ErrFrontmatterNotFound", err)
	}
}

func TestParserRequiresFrontmatterEnd(t *testing.T) {
	_, err := NewParser().Parse("SKILL.md", []byte(`---
name: broken
description: missing end marker
`))
	if !errors.Is(err, ErrFrontmatterEndNotFound) {
		t.Fatalf("Parse() error = %v, want ErrFrontmatterEndNotFound", err)
	}
}
