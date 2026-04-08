package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jiujuan/wukong/internal/model"
	dbpkg "github.com/jiujuan/wukong/pkg/database"
	"github.com/jiujuan/wukong/pkg/skills"
)

type SkillRepository struct {
	db *dbpkg.DB
}

func NewSkillRepository(db *dbpkg.DB) *SkillRepository {
	return &SkillRepository{db: db}
}

func (r *SkillRepository) BatchUpsertSkills(ctx context.Context, items []*skills.Skill) error {
	if r == nil || r.db == nil || len(items) == 0 {
		return nil
	}
	values := make([]string, 0, len(items))
	args := make([]any, 0, len(items)*8)
	now := time.Now()
	index := 1
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.SkillName) == "" {
			continue
		}
		values = append(values, skillPlaceholderTuple(index, 8))
		index += 8
		args = append(args,
			item.SkillName,
			item.Description,
			item.Version,
			item.Enabled,
			item.Memory.MemoryType,
			item.Memory.WindowSize,
			item.Memory.CompressSwitch,
			now,
		)
	}
	if len(values) == 0 {
		return nil
	}
	query := `
		INSERT INTO skill_meta (
			skill_name, description, version, enabled, memory_type, memory_window, memory_compress, updated_at
		) VALUES ` + strings.Join(values, ",") + `
		ON CONFLICT (skill_name) DO UPDATE SET
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			enabled = EXCLUDED.enabled,
			memory_type = EXCLUDED.memory_type,
			memory_window = EXCLUDED.memory_window,
			memory_compress = EXCLUDED.memory_compress,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func skillPlaceholderTuple(start int, n int) string {
	holders := make([]string, n)
	for i := 0; i < n; i++ {
		holders[i] = fmt.Sprintf("$%d", start+i)
	}
	return "(" + strings.Join(holders, ", ") + ")"
}

func (r *SkillRepository) ListSkills(ctx context.Context, limit int) ([]*model.SkillMeta, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, skill_name, description, version, enabled, memory_type, memory_window, memory_compress, created_at, updated_at
		FROM skill_meta
		ORDER BY skill_name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*model.SkillMeta, 0, limit)
	for rows.Next() {
		item := &model.SkillMeta{}
		if err := rows.Scan(
			&item.ID,
			&item.SkillName,
			&item.Description,
			&item.Version,
			&item.Enabled,
			&item.MemoryType,
			&item.MemoryWindow,
			&item.MemoryCompress,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
