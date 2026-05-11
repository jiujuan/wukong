package service

import (
	"context"
	"strings"

	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/internal/repository"
	"github.com/jiujuan/wukong/pkg/skills"
)

type SkillService struct {
	repo     *repository.SkillRepository
	registry *skills.Registry
}

func NewSkillService(repo *repository.SkillRepository, registry *skills.Registry) *SkillService {
	return &SkillService{repo: repo, registry: registry}
}

func (s *SkillService) ListSkills(ctx context.Context) ([]*model.SkillMeta, error) {
	if s == nil {
		return nil, nil
	}
	if s.repo != nil {
		list, err := s.repo.ListSkills(ctx, 500)
		if err == nil && len(list) > 0 {
			return list, nil
		}
		if err != nil {
			return nil, err
		}
	}
	if s.registry == nil {
		return nil, nil
	}
	items := s.registry.List()
	out := make([]*model.SkillMeta, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &model.SkillMeta{
			SkillName:      item.SkillName,
			Description:    item.Description,
			Version:        item.Version,
			Enabled:        item.Enabled,
			MemoryType:     item.Memory.MemoryType,
			MemoryWindow:   item.Memory.WindowSize,
			MemoryCompress: item.Memory.CompressSwitch,
		})
	}
	return out, nil
}

func (s *SkillService) GetSkill(ctx context.Context, skillName string) (*model.SkillMeta, error) {
	if s == nil {
		return nil, nil
	}
	if s.repo != nil {
		item, err := s.repo.GetSkill(ctx, skillName)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	if s.registry != nil {
		if item, ok := s.registry.Get(skillName); ok && item != nil {
			return &model.SkillMeta{
				SkillName:      item.SkillName,
				Description:    item.Description,
				Version:        item.Version,
				Enabled:        item.Enabled,
				MemoryType:     item.Memory.MemoryType,
				MemoryWindow:   item.Memory.WindowSize,
				MemoryCompress: item.Memory.CompressSwitch,
			}, nil
		}
	}
	return nil, nil
}

func (s *SkillService) UpdateSkill(ctx context.Context, item *model.SkillMeta) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if item != nil {
		item.SkillName = strings.TrimSpace(item.SkillName)
		if item.Version == "" {
			item.Version = "v1"
		}
	}
	return s.repo.UpdateSkill(ctx, item)
}
