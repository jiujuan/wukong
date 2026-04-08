package service

import (
	"context"

	"github.com/jiujuan/wukong/pkg/memory"
)

type MemoryService struct {
	manager *memory.Manager
}

func NewMemoryService(manager *memory.Manager) *MemoryService {
	return &MemoryService{manager: manager}
}

func (s *MemoryService) ListWorking(ctx context.Context, userID string, taskID string, limit int) []*memory.WorkingMemory {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.ListWorking(ctx, userID, taskID, limit)
}

func (s *MemoryService) ListLong(ctx context.Context, userID string, skillName string, keyword string, limit int) []*memory.LongTermMemory {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.ListLong(ctx, userID, skillName, keyword, limit)
}
