package adapter

import (
	"path/filepath"
	"strings"

	"github.com/jiujuan/wukong/pkg/skills"
)

type legacyAdapter struct{}

func NewLegacyAdapter() Adapter { return legacyAdapter{} }

func (legacyAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) {
		return false
	}
	text := strings.ToLower(string(content))
	if strings.Contains(text, "## tools") || strings.Contains(text, "## execute") || strings.Contains(text, "# skill:") {
		return true
	}
	return !hasFrontmatter(content)
}

func (legacyAdapter) Parse(path string) (*skills.Skill, error) {
	dirName := filepath.Base(filepath.Dir(path))
	item, err := skills.ParseSkillFile(path, dirName)
	if err != nil {
		return nil, err
	}
	applyLegacyPackage(item, path)
	return item, nil
}

func isSkillMarkdown(path string) bool {
	return strings.EqualFold(filepath.Base(path), "SKILL.md")
}

func hasFrontmatter(content []byte) bool {
	text := strings.TrimSpace(string(content))
	return strings.HasPrefix(text, "---")
}

func applyLegacyPackage(item *skills.Skill, skillFile string) {
	if item == nil {
		return
	}
	rootDir := filepath.Clean(filepath.Dir(skillFile))
	item.Package.SourceType = skills.SourceLegacy
	item.Package.PackageName = item.SkillName
	item.Package.RootDir = rootDir
	item.Package.Entry = item.Execute
	if item.Package.Runtime == "" && item.Execute != "" {
		item.Package.Runtime = runtimeFromEntry(item.Execute)
	}
	item.SourcePath = skillFile
}
