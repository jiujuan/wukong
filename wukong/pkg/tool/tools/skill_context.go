package tools

import (
	"context"
	"path/filepath"
	"strings"
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
	return item, ok && strings.TrimSpace(item.SkillName) != ""
}

func normalizeSkillContext(info SkillContext) SkillContext {
	info.SkillName = strings.ToLower(strings.TrimSpace(info.SkillName))
	info.SkillRoot = cleanPath(info.SkillRoot)
	info.OutputDir = cleanPath(info.OutputDir)
	info.Version = strings.TrimSpace(info.Version)
	info.SourceType = strings.TrimSpace(info.SourceType)
	info.PackageName = strings.TrimSpace(info.PackageName)
	return info
}

func cleanPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if abs, err := filepath.Abs(value); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(value)
}
