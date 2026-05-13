package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func defaultAdapters() []Adapter {
	return []Adapter{
		agentSkillsAdapter{},
		vendorSkillAdapter{},
		legacySkillAdapter{},
	}
}

type legacySkillAdapter struct{}

func (legacySkillAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) {
		return false
	}
	text := strings.ToLower(string(content))
	return strings.Contains(text, "## tools") || strings.Contains(text, "## execute") || strings.Contains(text, "# skill:")
}

func (legacySkillAdapter) Parse(path string) (*Skill, error) {
	dirName := filepath.Base(filepath.Dir(path))
	item, err := ParseSkillFile(path, dirName)
	if err != nil {
		return nil, err
	}
	item.Package.RootDir = filepath.Clean(filepath.Dir(path))
	item.Package.PackageName = item.SkillName
	item.Package.Entry = item.Execute
	if item.Package.Runtime == "" && item.Execute != "" {
		item.Package.Runtime = runtimeFromScript(item.Execute)
	}
	return item, nil
}

type agentSkillsAdapter struct{}

func (agentSkillsAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) || !hasFrontmatter(content) {
		return false
	}
	meta, _, err := parseAgentFrontmatter(string(content))
	if err != nil {
		return false
	}
	return strings.TrimSpace(meta.Name) != "" && strings.TrimSpace(meta.Description) != ""
}

func (agentSkillsAdapter) Parse(path string) (*Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body, err := parseAgentFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	item := &Skill{
		SkillName:   normalizeSkillName(meta.Name),
		Description: strings.TrimSpace(meta.Description),
		Version:     strings.TrimSpace(meta.Version),
		Enabled:     meta.Enabled,
		Tools:       append([]string(nil), meta.AllowedTools...),
		Template:    body,
		SourcePath:  path,
		Package: PackageMeta{
			SourceType:  SourceVendor,
			PackageName: normalizeSkillName(meta.Name),
			Version:     strings.TrimSpace(meta.Version),
			Runtime:     strings.TrimSpace(meta.Runtime),
			Entry:       strings.TrimSpace(meta.Entry),
			RootDir:     filepath.Clean(filepath.Dir(path)),
		},
	}
	if item.Package.Runtime == "" && item.Package.Entry != "" {
		item.Package.Runtime = runtimeFromScript(item.Package.Entry)
	}
	return item, nil
}

type vendorSkillAdapter struct{}

func (vendorSkillAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) {
		return false
	}
	if hasVendorManifest(filepath.Dir(path)) {
		return true
	}
	return hasFrontmatter(content)
}

func (vendorSkillAdapter) Parse(path string) (*Skill, error) {
	if hasVendorManifest(filepath.Dir(path)) {
		return LoadSkillPackage(path, SourceVendor)
	}
	if hasFrontmatterFromFile(path) {
		item, err := agentSkillsAdapter{}.Parse(path)
		if err != nil {
			return nil, err
		}
		item.Package.SourceType = SourceVendor
		return item, nil
	}
	item, err := ParseSkillFile(path, filepath.Base(filepath.Dir(path)))
	if err != nil {
		return nil, err
	}
	item.Package.SourceType = SourceVendor
	item.Package.RootDir = filepath.Clean(filepath.Dir(path))
	item.Package.PackageName = item.SkillName
	item.Package.Entry = item.Execute
	if item.Package.Runtime == "" && item.Execute != "" {
		item.Package.Runtime = runtimeFromScript(item.Execute)
	}
	return item, nil
}

func parseAgentFrontmatter(content string) (agentFrontmatter, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---\n") {
		return agentFrontmatter{}, "", fmt.Errorf("frontmatter not found")
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return agentFrontmatter{}, "", fmt.Errorf("frontmatter end marker not found")
	}
	header := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---"):])
	meta, err := parseAgentFrontmatterHeader(header)
	if err != nil {
		return agentFrontmatter{}, "", err
	}
	return meta, body, nil
}

func parseAgentFrontmatterHeader(header string) (agentFrontmatter, error) {
	meta := agentFrontmatter{Enabled: true}
	lines := strings.Split(header, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "metadata:") {
			for j := i + 1; j < len(lines); j++ {
				child := lines[j]
				if !strings.HasPrefix(child, "  ") && !strings.HasPrefix(child, "\t") {
					break
				}
				key, value, ok := strings.Cut(strings.TrimSpace(child), ":")
				if !ok {
					continue
				}
				key = strings.ToLower(strings.TrimSpace(key))
				value = strings.Trim(strings.TrimSpace(value), "\"'")
				switch key {
				case "entry":
					meta.Entry = value
				case "runtime":
					meta.Runtime = value
				case "version":
					meta.Version = value
				case "enabled":
					meta.Enabled = !strings.EqualFold(value, "false")
				}
			}
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		switch key {
		case "name":
			meta.Name = value
		case "description":
			meta.Description = value
		case "version":
			meta.Version = value
		case "runtime":
			meta.Runtime = value
		case "entry":
			meta.Entry = value
		case "enabled":
			meta.Enabled = !strings.EqualFold(value, "false")
		case "allowed-tools", "allowed_tools":
			meta.AllowedTools = parseListValue(value)
		}
	}
	if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Description) == "" {
		return agentFrontmatter{}, fmt.Errorf("frontmatter missing required fields")
	}
	return meta, nil
}

type agentFrontmatter struct {
	Name         string
	Description  string
	Version      string
	Runtime      string
	Entry        string
	Enabled      bool
	AllowedTools []string
}

func isSkillMarkdown(path string) bool {
	return strings.EqualFold(filepath.Base(path), "SKILL.md")
}

func hasFrontmatter(content []byte) bool {
	text := strings.TrimSpace(string(content))
	return strings.HasPrefix(text, "---")
}

func hasFrontmatterFromFile(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return hasFrontmatter(raw)
}

func hasVendorManifest(dir string) bool {
	manifestPath := filepath.Join(dir, "wukong.skill.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}
