package adapter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jiujuan/wukong/pkg/skills"
)

type agentSkillsAdapter struct{}

func NewAgentSkillsAdapter() Adapter { return agentSkillsAdapter{} }

func (agentSkillsAdapter) Match(path string, content []byte) bool {
	if !isSkillMarkdown(path) || !hasFrontmatter(content) {
		return false
	}
	meta, _, err := parseFrontmatter(string(content))
	if err != nil {
		return false
	}
	return strings.TrimSpace(meta.Name) != "" && strings.TrimSpace(meta.Description) != ""
}

func (agentSkillsAdapter) Parse(path string) (*skills.Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body, err := parseFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(meta.Name) == "" {
		return nil, fmt.Errorf("agent skill name is empty: %s", path)
	}
	if strings.TrimSpace(meta.Description) == "" {
		return nil, fmt.Errorf("agent skill description is empty: %s", path)
	}
	item := &skills.Skill{
		SkillName:   normalizeSkillName(meta.Name),
		Description: strings.TrimSpace(meta.Description),
		Version:     strings.TrimSpace(meta.Version),
		Enabled:     meta.Enabled,
		Tools:       append([]string(nil), meta.AllowedTools...),
		Template:    strings.TrimSpace(body),
		SourcePath:  path,
		Package: skills.PackageMeta{
			SourceType:  skills.SourceVendor,
			PackageName: normalizeSkillName(meta.Name),
			Version:     strings.TrimSpace(meta.Version),
			RootDir:     filepath.Clean(filepath.Dir(path)),
		},
	}
	item.Package.Entry = strings.TrimSpace(meta.Entry)
	item.Package.Runtime = strings.TrimSpace(meta.Runtime)
	if item.Package.Runtime == "" && item.Package.Entry != "" {
		item.Package.Runtime = runtimeFromEntry(item.Package.Entry)
	}
	return item, nil
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

func parseFrontmatter(content string) (agentFrontmatter, string, error) {
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
	meta, err := parseFrontmatterHeader(header)
	if err != nil {
		return agentFrontmatter{}, "", err
	}
	return meta, body, nil
}

func parseFrontmatterHeader(header string) (agentFrontmatter, error) {
	meta := agentFrontmatter{Enabled: true}
	scanner := bufio.NewScanner(strings.NewReader(header))
	var inMetadata bool
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "metadata:") {
			inMetadata = true
			continue
		}
		if inMetadata {
			if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
				key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
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
				continue
			}
			inMetadata = false
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
	if err := scanner.Err(); err != nil {
		return agentFrontmatter{}, err
	}
	if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Description) == "" {
		return agentFrontmatter{}, fmt.Errorf("frontmatter missing required fields")
	}
	return meta, nil
}

func parseListValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	result := make([]string, 0, len(fields))
	for _, item := range fields {
		item = strings.TrimSpace(strings.Trim(item, "\"'"))
		if item != "" {
			result = append(result, strings.ToLower(item))
		}
	}
	return result
}
