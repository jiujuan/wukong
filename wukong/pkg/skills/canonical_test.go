package skills

import (
	"testing"

	"github.com/jiujuan/wukong/pkg/skills/model"
)

func TestSkillCanonicalMappingComplete(t *testing.T) {
	src := &Skill{
		SkillName:   "report_gen",
		Description: "generate report",
		Version:     "1.2.3",
		Enabled:     true,
		Params: []Param{{
			Name:     "title",
			Type:     "string",
			Required: true,
		}},
		Tools:    []string{"llm_chat", "file_write"},
		Execute:  "run.sh",
		Template: "template.md",
		Memory: MemoryConfig{
			MemoryType:     "working",
			WindowSize:     8,
			CompressSwitch: true,
		},
		SourcePath: "/tmp/skills/report_gen/SKILL.md",
		Package: PackageMeta{
			SourceType:   SourceVendor,
			PackageName:  "report_gen",
			Version:      "1.2.3",
			Runtime:      "bash",
			Entry:        "run.sh",
			RootDir:      "/tmp/skills/report_gen",
			ManifestPath: "/tmp/skills/report_gen/wukong.skill.json",
		},
	}

	got := src.Canonical()
	assertCanonicalSkill(t, got, model.CanonicalSkill{
		Name:         "report_gen",
		Description:  "generate report",
		Instructions: "template.md",
		Version:      "1.2.3",
		Enabled:      true,
	})
	if len(got.AllowedTools) != 2 || got.AllowedTools[0] != "llm_chat" || got.AllowedTools[1] != "file_write" {
		t.Fatalf("allowed tools mismatch: %#v", got.AllowedTools)
	}
	if got.Runtime.Runtime != "bash" || got.Runtime.Entry != "run.sh" || got.Runtime.WorkDir != "/tmp/skills/report_gen" {
		t.Fatalf("runtime mismatch: %#v", got.Runtime)
	}
	if got.Source.Type != string(SourceVendor) || got.Source.RootDir != "/tmp/skills/report_gen" || got.Source.ManifestPath != "/tmp/skills/report_gen/wukong.skill.json" {
		t.Fatalf("source mismatch: %#v", got.Source)
	}
	if got.Metadata == nil {
		t.Fatalf("metadata should not be nil")
	}
	if got.Metadata["memory"] == nil || got.Metadata["package"] == nil {
		t.Fatalf("metadata missing fields: %#v", got.Metadata)
	}
}

func TestLegacyAndVendorSkillsMapToCanonicalModel(t *testing.T) {
	legacy := &Skill{
		SkillName:   "chat",
		Description: "legacy chat",
		Enabled:     true,
		Tools:       []string{"llm_chat"},
		Execute:     "run.sh",
		Package: PackageMeta{
			SourceType: SourceLegacy,
			RootDir:    "/tmp/skills/chat",
			Runtime:    "bash",
			Entry:      "run.sh",
		},
	}
	vendor := &Skill{
		SkillName:   "pptx",
		Description: "vendor pptx",
		Enabled:     true,
		Tools:       []string{"generate_ppt"},
		Execute:     "scripts/wukong/generate.py",
		Package: PackageMeta{
			SourceType: SourceVendor,
			RootDir:    "/tmp/skills/vendor/pptx",
			Runtime:    "python",
			Entry:      "scripts/wukong/generate.py",
		},
	}

	legacyCanon := legacy.Canonical()
	vendorCanon := vendor.Canonical()

	if legacyCanon.Name != "chat" || vendorCanon.Name != "pptx" {
		t.Fatalf("canonical names mismatch: %#v %#v", legacyCanon, vendorCanon)
	}
	if legacyCanon.Runtime.Entry == "" || vendorCanon.Runtime.Entry == "" {
		t.Fatalf("canonical runtime entry should not be empty: %#v %#v", legacyCanon.Runtime, vendorCanon.Runtime)
	}
	if legacyCanon.Source.Type != string(SourceLegacy) {
		t.Fatalf("legacy source type mismatch: %#v", legacyCanon.Source)
	}
	if vendorCanon.Source.Type != string(SourceVendor) {
		t.Fatalf("vendor source type mismatch: %#v", vendorCanon.Source)
	}
	if legacyCanon.Metadata == nil || vendorCanon.Metadata == nil {
		t.Fatalf("metadata should not be nil")
	}
}

func assertCanonicalSkill(t *testing.T, got, want model.CanonicalSkill) {
	t.Helper()
	if got.Name != want.Name {
		t.Fatalf("name = %q, want %q", got.Name, want.Name)
	}
	if got.Description != want.Description {
		t.Fatalf("description = %q, want %q", got.Description, want.Description)
	}
	if got.Instructions != want.Instructions {
		t.Fatalf("instructions = %q, want %q", got.Instructions, want.Instructions)
	}
	if got.Version != want.Version {
		t.Fatalf("version = %q, want %q", got.Version, want.Version)
	}
	if got.Enabled != want.Enabled {
		t.Fatalf("enabled = %v, want %v", got.Enabled, want.Enabled)
	}
}
