package skills

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
	rootInfo, err := os.Stat(r.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	if !rootInfo.IsDir() {
		return result, nil
	}
	entries, err := os.ReadDir(r.rootDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(r.rootDir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}
		item, err := parseSkillFile(skillFile, entry.Name())
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("[skills] parse skill failed", "file", skillFile, "error", err)
			}
			continue
		}
		result[item.SkillName] = item
	}
	return result, nil
}
