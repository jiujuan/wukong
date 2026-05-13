package skills

import (
	"log/slog"
	"path/filepath"
	"time"

	wkstr "github.com/jiujuan/wukong/pkg/str"
)

type Option func(*Registry)

func WithRootDir(rootDir string) Option {
	return func(r *Registry) {
		if wkstr.NotEmpty(rootDir) {
			r.rootDir = rootDir
			if len(r.roots) == 0 {
				r.roots = defaultSkillRoots(rootDir)
			}
		}
	}
}

func WithSkillRoots(roots ...SkillRoot) Option {
	return func(r *Registry) {
		cleaned := make([]SkillRoot, 0, len(roots))
		for _, root := range roots {
			dir := wkstr.Trim(root.Dir)
			if dir == "" {
				continue
			}
			cleaned = append(cleaned, SkillRoot{
				Type: normalizeSourceType(root.Type),
				Dir:  dir,
			})
		}
		if len(cleaned) > 0 {
			r.roots = cleaned
		}
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(r *Registry) {
		if interval > 0 {
			r.pollInterval = interval
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(r *Registry) {
		if timeout > 0 {
			r.execTimeout = timeout
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(r *Registry) {
		if logger != nil {
			r.logger = logger
		}
	}
}

func WithMetaStore(store MetaStore) Option {
	return func(r *Registry) {
		r.store = store
	}
}

func defaultSkillRoots(rootDir string) []SkillRoot {
	rootDir = wkstr.Trim(rootDir)
	if rootDir == "" {
		rootDir = "skills"
	}
	return []SkillRoot{
		{Type: SourceLocal, Dir: filepath.Join(rootDir, "local")},
		{Type: SourceVendor, Dir: filepath.Join(rootDir, "vendor")},
		{Type: SourceLegacy, Dir: rootDir},
	}
}

func normalizeSourceType(value SourceType) SourceType {
	switch wkstr.TrimLower(string(value)) {
	case string(SourceBuiltin):
		return SourceBuiltin
	case string(SourceLocal):
		return SourceLocal
	case string(SourceVendor):
		return SourceVendor
	case string(SourceLegacy):
		return SourceLegacy
	default:
		return SourceLegacy
	}
}
