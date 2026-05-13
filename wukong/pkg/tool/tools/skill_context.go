package tools

import (
	"context"
	"path/filepath"

	wkstr "github.com/jiujuan/wukong/pkg/str"
)

type skillContextKey struct{}

type SkillContext struct {
	SkillName   string
	SkillRoot   string
	OutputDir   string
	Version     string
	SourceType  string
	PackageName string
}

func WithSkillContext(ctx context.Context, info SkillContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, skillContextKey{}, normalizeSkillContext(info))
}

func SkillContextFromContext(ctx context.Context) (SkillContext, bool) {
	if ctx == nil {
		return SkillContext{}, false
	}
	item, ok := ctx.Value(skillContextKey{}).(SkillContext)
	return item, ok && wkstr.NotEmpty(item.SkillName)
}

func normalizeSkillContext(info SkillContext) SkillContext {
	info.SkillName = wkstr.TrimLower(info.SkillName)
	info.SkillRoot = cleanPath(info.SkillRoot)
	info.OutputDir = cleanPath(info.OutputDir)
	info.Version = wkstr.Trim(info.Version)
	info.SourceType = wkstr.Trim(info.SourceType)
	info.PackageName = wkstr.Trim(info.PackageName)
	return info
}

func cleanPath(value string) string {
	value = wkstr.Trim(value)
	if value == "" {
		return ""
	}
	if abs, err := filepath.Abs(value); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(value)
}
