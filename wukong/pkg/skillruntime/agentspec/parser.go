package agentspec

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jiujuan/wukong/pkg/skillruntime"
	"go.yaml.in/yaml/v3"
)

const (
	// RuntimeName is the skillruntime name for Agent Skills Spec packages.
	RuntimeName = "agentspec"
	// SkillFileName is the canonical Agent Skills Spec entry file.
	SkillFileName = "SKILL.md"
)

// Parser parses Agent Skills Spec SKILL.md files.
type Parser struct {
	validator Validator
}

// NewParser creates a parser with the default validator.
func NewParser() Parser {
	return Parser{validator: Validator{}}
}

// ParseFile reads and parses one SKILL.md file.
func (p Parser) ParseFile(path string) (*skillruntime.SkillSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	spec, err := p.Parse(path, raw)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

// ParseManifestFile reads only frontmatter and returns a lightweight manifest.
func (p Parser) ParseManifestFile(path string) (skillruntime.SkillManifest, error) {
	header, err := readFrontmatterHeader(path)
	if err != nil {
		return skillruntime.SkillManifest{}, err
	}

	var front frontmatter
	if err := yaml.Unmarshal([]byte(header), &front); err != nil {
		return skillruntime.SkillManifest{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	spec := front.toSkillSpec(filepath.Clean(filepath.Dir(path)), "")
	if err := p.validator.Validate(spec); err != nil {
		return skillruntime.SkillManifest{}, err
	}
	return spec.Manifest, nil
}

// Parse parses a SKILL.md document from bytes.
func (p Parser) Parse(path string, content []byte) (*skillruntime.SkillSpec, error) {
	header, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var front frontmatter
	if err := yaml.Unmarshal([]byte(header), &front); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	spec := front.toSkillSpec(filepath.Clean(filepath.Dir(path)), strings.TrimSpace(body))
	if err := p.validator.Validate(spec); err != nil {
		return nil, err
	}
	return spec, nil
}

type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	Version       string            `yaml:"version"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	AllowedTools  allowedTools      `yaml:"allowed-tools"`
	AllowedTools2 allowedTools      `yaml:"allowed_tools"`
	Metadata      map[string]string `yaml:"metadata"`
	Tags          []string          `yaml:"tags"`
}

func (f frontmatter) toSkillSpec(rootDir, instructions string) *skillruntime.SkillSpec {
	allowed := f.AllowedTools.Values()
	if len(allowed) == 0 {
		allowed = f.AllowedTools2.Values()
	}
	metadata := make(map[string]any, len(f.Metadata))
	for key, value := range f.Metadata {
		metadata[key] = value
	}
	if len(metadata) == 0 {
		metadata = nil
	}

	return &skillruntime.SkillSpec{
		Manifest: skillruntime.SkillManifest{
			Name:          strings.TrimSpace(f.Name),
			Description:   strings.TrimSpace(f.Description),
			Runtime:       RuntimeName,
			Version:       strings.TrimSpace(f.Version),
			License:       strings.TrimSpace(f.License),
			Compatibility: strings.TrimSpace(f.Compatibility),
			Tags:          trimStringSlice(f.Tags),
			Metadata:      cloneStringMap(f.Metadata),
		},
		Instructions: strings.TrimSpace(instructions),
		AllowedTools: trimStringSlice(allowed),
		RootDir:      rootDir,
		Metadata:     metadata,
	}
}

type allowedTools []string

func (a allowedTools) Values() []string {
	return []string(a)
}

func (a *allowedTools) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if strings.TrimSpace(value.Value) == "" {
			*a = nil
			return nil
		}
		*a = splitAllowedTools(value.Value)
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			tool := strings.TrimSpace(item.Value)
			if tool != "" {
				out = append(out, tool)
			}
		}
		*a = out
		return nil
	default:
		return fmt.Errorf("allowed-tools must be a string or string list")
	}
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", ErrFrontmatterNotFound
	}
	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", ErrFrontmatterEndNotFound
	}
	header := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return header, body, nil
}

func readFrontmatterHeader(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", ErrFrontmatterNotFound
	}
	first := strings.TrimPrefix(scanner.Text(), "\ufeff")
	if strings.TrimSpace(first) != "---" {
		return "", ErrFrontmatterNotFound
	}

	lines := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			return strings.Join(lines, "\n"), nil
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", ErrFrontmatterEndNotFound
}

func splitAllowedTools(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(strings.TrimSpace(field), "\"'")
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func trimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
