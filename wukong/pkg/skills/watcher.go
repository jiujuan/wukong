package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (r *Registry) watch(ctx context.Context) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.reload(ctx)
		}
	}
}

func (r *Registry) reload(ctx context.Context) error {
	loaded, err := r.loadFromDisk()
	if err != nil {
		return err
	}
	for _, item := range defaultBuiltins() {
		if _, ok := loaded[item.SkillName]; !ok {
			loaded[item.SkillName] = item
		}
	}
	items := make([]*Skill, 0, len(loaded))
	for _, item := range loaded {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SkillName < items[j].SkillName
	})

	r.mu.Lock()
	changed := changedWithCurrent(r.skills, loaded)
	if changed {
		r.skills = loaded
	}
	r.mu.Unlock()

	if changed && r.store != nil {
		upsertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := r.store.BatchUpsertSkills(upsertCtx, items)
		cancel()
		if err != nil && r.logger != nil {
			r.logger.Warn("[skills] batch upsert skill meta failed", "error", err)
		}
	}
	return nil
}

func (r *Registry) loadFromDisk() (map[string]*Skill, error) {
	result := make(map[string]*Skill)
	roots := r.skillRoots()
	for _, root := range roots {
		rootDir := strings.TrimSpace(root.Dir)
		if rootDir == "" {
			continue
		}
		rootInfo, err := os.Stat(rootDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !rootInfo.IsDir() {
			continue
		}
		entries, err := os.ReadDir(rootDir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(rootDir, entry.Name())
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			item, err := r.parseSkillWithAdapters(skillFile)
			if err != nil {
				if r.logger != nil {
					r.logger.Warn("[skills] parse skill failed", "file", skillFile, "error", err)
				}
				continue
			}
			references, assets, metadata, err := resolveSkillResources(skillDir)
			if err != nil {
				if r.logger != nil {
					r.logger.Warn("[skills] resolve skill resources failed", "dir", skillDir, "error", err)
				}
			} else {
				item.References = references
				item.Assets = assets
				item.Metadata = mergeSkillMetadata(item.Metadata, metadata)
			}
			item.Package.SourceType = root.Type
			item.Package.RootDir = skillDir
			if strings.TrimSpace(item.Package.PackageName) == "" {
				item.Package.PackageName = item.SkillName
			}
			if strings.TrimSpace(item.Package.ManifestPath) == "" {
				manifestPath := filepath.Join(skillDir, "wukong.skill.json")
				if _, err := os.Stat(manifestPath); err == nil {
					item.Package.ManifestPath = manifestPath
				}
			}
			if _, exists := result[item.SkillName]; exists {
				continue
			}
			result[item.SkillName] = item
		}
	}
	return result, nil
}

func mergeSkillMetadata(base, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	result := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}

func (r *Registry) skillRoots() []SkillRoot {
	if len(r.roots) > 0 {
		return append([]SkillRoot(nil), r.roots...)
	}
	if strings.TrimSpace(r.rootDir) == "" {
		return defaultSkillRoots("skills")
	}
	return defaultSkillRoots(r.rootDir)
}

func loadSkillPackage(skillFile string, sourceType SourceType) (*Skill, error) {
	rootDir := skillPackageRoot(skillFile)
	if strings.TrimSpace(rootDir) == "" {
		return nil, fmt.Errorf("skill root empty for %s", skillFile)
	}
	dirName := filepath.Base(rootDir)
	item, err := parseSkillFile(skillFile, dirName)
	if err != nil {
		return nil, err
	}
	manifest, manifestPath, err := loadPackageManifest(rootDir)
	if err != nil {
		return nil, err
	}
	if manifest.PackageName != "" {
		item.SkillName = normalizeSkillName(manifest.PackageName)
	}
	if item.SkillName == "" {
		item.SkillName = normalizeSkillName(dirName)
	}
	if manifest.Version != "" {
		item.Version = manifest.Version
	}
	if manifest.Homepage != "" {
		item.Package.Homepage = manifest.Homepage
	}
	if manifest.Runtime != "" {
		item.Package.Runtime = manifest.Runtime
	}
	if manifest.Entry != "" {
		item.Execute = manifest.Entry
	}
	if len(manifest.Tools) > 0 {
		item.Tools = append([]string(nil), manifest.Tools...)
	}
	references, assets, metadata, err := resolveSkillResources(rootDir)
	if err != nil {
		return nil, err
	}
	item.Package = PackageMeta{
		SourceType:   sourceType,
		PackageName:  item.SkillName,
		Version:      item.Version,
		Homepage:     item.Package.Homepage,
		Runtime:      item.Package.Runtime,
		Entry:        item.Execute,
		RootDir:      rootDir,
		ManifestPath: manifestPath,
	}
	item.References = references
	item.Assets = assets
	item.Metadata = metadata
	if strings.TrimSpace(item.Package.Runtime) == "" && strings.TrimSpace(item.Execute) != "" {
		item.Package.Runtime = runtimeFromScript(item.Execute)
	}
	if err := validateSkillPackage(rootDir, item); err != nil {
		return nil, err
	}
	return item, nil
}

// LoadSkillPackage loads a skill package from SKILL.md and its adjacent manifest.
func LoadSkillPackage(skillFile string, sourceType SourceType) (*Skill, error) {
	return loadSkillPackage(skillFile, sourceType)
}

// LoadSkillPackage loads a skill package with registry context.
func (r *Registry) LoadSkillPackage(skillFile string, sourceType SourceType) (*Skill, error) {
	return loadSkillPackage(skillFile, sourceType)
}
