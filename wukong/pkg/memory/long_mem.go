package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jiujuan/wukong/pkg/uuid"
)

type LongMemoryStore struct {
	mu    sync.RWMutex
	items map[string]*LongTermMemory
}

func NewLongMemoryStore() *LongMemoryStore {
	return &LongMemoryStore{
		items: make(map[string]*LongTermMemory),
	}
}

func (s *LongMemoryStore) Create(userID string, skillName string, topic string, content string, sourceTaskID string) *LongTermMemory {
	now := time.Now()
	item := &LongTermMemory{
		MemoryID:     "mem_" + uuid.NewShort(),
		UserID:       userID,
		SkillName:    strings.ToLower(strings.TrimSpace(skillName)),
		Topic:        strings.TrimSpace(topic),
		Content:      content,
		SourceTaskID: sourceTaskID,
		CreatedAt:    now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.MemoryID] = item
	return cloneLong(item)
}

func (s *LongMemoryStore) Get(memoryID string) (*LongTermMemory, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[memoryID]
	if !ok {
		return nil, false
	}
	return cloneLong(item), true
}

func (s *LongMemoryStore) Delete(memoryID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[memoryID]; !ok {
		return false
	}
	delete(s.items, memoryID)
	return true
}

func (s *LongMemoryStore) Search(userID string, skillName string, keyword string, limit int) []*LongTermMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	result := make([]*LongTermMemory, 0, limit)
	userID = strings.TrimSpace(userID)
	skillName = strings.ToLower(strings.TrimSpace(skillName))
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, item := range s.items {
		if userID != "" && item.UserID != userID {
			continue
		}
		if skillName != "" && item.SkillName != skillName {
			continue
		}
		if keyword != "" {
			text := strings.ToLower(item.Topic + " " + item.Content)
			if !strings.Contains(text, keyword) {
				continue
			}
		}
		result = append(result, cloneLong(item))
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (s *LongMemoryStore) List(userID string, skillName string, keyword string, limit int) []*LongTermMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	userID = strings.TrimSpace(userID)
	skillName = strings.ToLower(strings.TrimSpace(skillName))
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	result := make([]*LongTermMemory, 0, len(s.items))
	for _, item := range s.items {
		if userID != "" && item.UserID != userID {
			continue
		}
		if skillName != "" && item.SkillName != skillName {
			continue
		}
		if keyword != "" {
			text := strings.ToLower(item.Topic + " " + item.Content)
			if !strings.Contains(text, keyword) {
				continue
			}
		}
		result = append(result, cloneLong(item))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func cloneLong(src *LongTermMemory) *LongTermMemory {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
