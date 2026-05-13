package skills

import (
	"path/filepath"
	"strings"

	"github.com/jiujuan/wukong/pkg/skills/model"
)

// ToCanonical converts a legacy Skill into the unified canonical skill model.
func (s *Skill) ToCanonical() model.CanonicalSkill {
	if s == nil {
		return model.CanonicalSkill{}
	}
	result := model.CanonicalSkill{
		Name:         strings.TrimSpace(s.SkillName),
		Description:  strings.TrimSpace(s.Description),
		Instructions: canonicalInstructions(s),
		AllowedTools: append([]string(nil), s.Tools...),
		Runtime:      canonicalRuntimeSpec(s),
		Source:       canonicalSkillSource(s),
		References:   canonicalResources("reference", s.References),
		Assets:       canonicalResources("asset", s.Assets),
		Metadata:     canonicalMetadata(s),
		Enabled:      s.Enabled,
		Version:      strings.TrimSpace(s.Version),
	}
	return result
}

// FromCanonical converts a canonical skill back into the legacy Skill shape.
func FromCanonical(src model.CanonicalSkill) *Skill {
	dst := &Skill{
		SkillName:   strings.TrimSpace(src.Name),
		Description: strings.TrimSpace(src.Description),
		Version:     strings.TrimSpace(src.Version),
		Enabled:     src.Enabled,
		Tools:       append([]string(nil), src.AllowedTools...),
		Execute:     strings.TrimSpace(src.Runtime.Entry),
	}
	if dst.SkillName == "" {
		dst.SkillName = strings.TrimSpace(src.Source.PackageName)
	}
	if dst.Description == "" {
		dst.Description = strings.TrimSpace(src.Instructions)
	}
	if dst.SkillName == "" {
		dst.SkillName = "skill"
	}
	dst.Package = PackageMeta{
		SourceType:   SourceType(strings.TrimSpace(src.Source.Type)),
		PackageName:  firstNonEmpty(src.Source.PackageName, src.Source.Name, dst.SkillName),
		Version:      strings.TrimSpace(src.Version),
		Runtime:      strings.TrimSpace(src.Runtime.Runtime),
		Entry:        strings.TrimSpace(src.Runtime.Entry),
		RootDir:      strings.TrimSpace(src.Source.RootDir),
		ManifestPath: strings.TrimSpace(src.Source.ManifestPath),
	}
	if dst.Package.RootDir == "" && strings.TrimSpace(src.Source.SourcePath) != "" {
		dst.SourcePath = filepath.Clean(src.Source.SourcePath)
		dst.Package.RootDir = filepath.Clean(filepath.Dir(dst.SourcePath))
	}
	if dst.Package.Runtime == "" {
		dst.Package.Runtime = strings.TrimSpace(src.Runtime.Runtime)
	}
	if dst.Execute == "" {
		dst.Execute = strings.TrimSpace(src.Runtime.Entry)
	}
	if len(dst.Tools) == 0 && len(src.AllowedTools) > 0 {
		dst.Tools = append([]string(nil), src.AllowedTools...)
	}
	if len(src.References) > 0 {
		dst.References = canonicalResourcePaths(src.References)
	}
	if len(src.Assets) > 0 {
		dst.Assets = canonicalResourcePaths(src.Assets)
	}
	if src.Metadata != nil {
		dst.Metadata = make(map[string]any, len(src.Metadata))
		for key, value := range src.Metadata {
			dst.Metadata[key] = value
		}
	}
	return dst
}

func canonicalInstructions(s *Skill) string {
	if s == nil {
		return ""
	}
	if strings.TrimSpace(s.Template) != "" {
		return strings.TrimSpace(s.Template)
	}
	if strings.TrimSpace(s.Description) != "" {
		return strings.TrimSpace(s.Description)
	}
	return ""
}

func canonicalRuntimeSpec(s *Skill) model.SkillRuntimeSpec {
	if s == nil {
		return model.SkillRuntimeSpec{}
	}
	runtimeName := strings.TrimSpace(s.Package.Runtime)
	entry := strings.TrimSpace(s.Package.Entry)
	if entry == "" {
		entry = strings.TrimSpace(s.Execute)
	}
	if runtimeName == "" && entry != "" {
		runtimeName = runtimeFromScript(entry)
	}
	return model.SkillRuntimeSpec{
		Runtime: runtimeName,
		Entry:   entry,
		WorkDir: strings.TrimSpace(s.Package.RootDir),
	}
}

func canonicalSkillSource(s *Skill) model.SkillSource {
	if s == nil {
		return model.SkillSource{}
	}
	sourceType := strings.TrimSpace(string(s.Package.SourceType))
	if sourceType == "" && strings.TrimSpace(s.SourcePath) != "" {
		sourceType = string(SourceLegacy)
	}
	return model.SkillSource{
		Type:         sourceType,
		Name:         strings.TrimSpace(s.SkillName),
		RootDir:      strings.TrimSpace(s.Package.RootDir),
		ManifestPath: strings.TrimSpace(s.Package.ManifestPath),
		SourcePath:   strings.TrimSpace(s.SourcePath),
		PackageName:  firstNonEmpty(s.Package.PackageName, s.SkillName),
		Version:      strings.TrimSpace(s.Version),
	}
}

func canonicalMetadata(s *Skill) map[string]any {
	if s == nil {
		return nil
	}
	meta := map[string]any{
		"enabled":     s.Enabled,
		"version":     s.Version,
		"template":    s.Template,
		"params":      append([]Param(nil), s.Params...),
		"memory":      s.Memory,
		"source_path": s.SourcePath,
		"package":     s.Package,
	}
	if len(s.Tools) > 0 {
		meta["allowed_tools"] = append([]string(nil), s.Tools...)
	}
	if len(s.References) > 0 {
		meta["references"] = append([]string(nil), s.References...)
	}
	if len(s.Assets) > 0 {
		meta["assets"] = append([]string(nil), s.Assets...)
	}
	if len(s.Metadata) > 0 {
		meta["metadata"] = cloneAnyMap(s.Metadata)
	}
	return meta
}

func canonicalResources(kind string, paths []string) []model.SkillResource {
	if len(paths) == 0 {
		return nil
	}
	result := make([]model.SkillResource, 0, len(paths))
	for _, path := range paths {
		result = append(result, model.SkillResource{
			Kind: kind,
			Name: filepath.Base(path),
			Path: path,
		})
	}
	return result
}

func canonicalResourcePaths(items []model.SkillResource) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Path) != "" {
			result = append(result, item.Path)
		}
	}
	return result
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
