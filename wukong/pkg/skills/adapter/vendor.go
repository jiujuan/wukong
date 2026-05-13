package adapter

import (
	"os"
	"path/filepath"

	"github.com/jiujuan/wukong/pkg/skills"
)

type vendorAdapter struct{}

func NewVendorAdapter() Adapter { return vendorAdapter{} }

func (vendorAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) {
		return false
	}
	if hasVendorManifest(filepath.Dir(path)) {
		return true
	}
	return hasFrontmatter(content)
}

func (vendorAdapter) Parse(path string) (*skills.Skill, error) {
	if hasVendorManifest(filepath.Dir(path)) {
		return skills.LoadSkillPackage(path, skills.SourceVendor)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if hasFrontmatter(raw) {
		item, err := NewAgentSkillsAdapter().Parse(path)
		if err != nil {
			return nil, err
		}
		item.Package.SourceType = skills.SourceVendor
		item.Package.RootDir = filepath.Clean(filepath.Dir(path))
		return item, nil
	}
	item, err := skills.ParseSkillFile(path, filepath.Base(filepath.Dir(path)))
	if err != nil {
		return nil, err
	}
	applyLegacyPackage(item, path)
	item.Package.SourceType = skills.SourceVendor
	return item, nil
}

func hasVendorManifest(dir string) bool {
	manifestPath := filepath.Join(dir, "wukong.skill.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}
