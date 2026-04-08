package memory

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type ShortMemoryStore struct {
	mu    sync.RWMutex
	items map[string]*WorkingMemory
}

func NewShortMemoryStore() *ShortMemoryStore {
	return &ShortMemoryStore{
		items: make(map[string]*WorkingMemory),
	}
}

func (s *ShortMemoryStore) Upsert(taskID string, userID string, windowSize int, expireAt *time.Time) *WorkingMemory {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item, ok := s.items[taskID]
	if !ok {
		if windowSize <= 0 {
			windowSize = 5
		}
		item = &WorkingMemory{
			TaskID:      taskID,
			UserID:      userID,
			WindowSize:  windowSize,
			FullHistory: make([]MemoryMessage, 0, windowSize),
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpireAt:    expireAt,
		}
		s.items[taskID] = item
		return cloneWorking(item)
	}
	if strings.TrimSpace(userID) != "" {
		item.UserID = userID
	}
	if windowSize > 0 {
		item.WindowSize = windowSize
	}
	if expireAt != nil {
		cp := *expireAt
		item.ExpireAt = &cp
	}
	item.UpdatedAt = now
	return cloneWorking(item)
}

func (s *ShortMemoryStore) Append(taskID string, msg MemoryMessage) *WorkingMemory {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[taskID]
	if !ok {
		item = &WorkingMemory{
			TaskID:      taskID,
			WindowSize:  5,
			FullHistory: make([]MemoryMessage, 0, 5),
			CreatedAt:   time.Now(),
		}
		s.items[taskID] = item
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	item.FullHistory = append(item.FullHistory, msg)
	item.UpdatedAt = time.Now()
	return cloneWorking(item)
}

func (s *ShortMemoryStore) Get(taskID string) (*WorkingMemory, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[taskID]
	if !ok {
		return nil, false
	}
	return cloneWorking(item), true
}

func (s *ShortMemoryStore) SetSummary(taskID string, summary string, compressed bool) (*WorkingMemory, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[taskID]
	if !ok {
		return nil, false
	}
	item.Summary = summary
	item.CompressFlag = compressed
	item.UpdatedAt = time.Now()
	return cloneWorking(item), true
}

func (s *ShortMemoryStore) Delete(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[taskID]; !ok {
		return false
	}
	delete(s.items, taskID)
	return true
}

func (s *ShortMemoryStore) DeleteExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for key, item := range s.items {
		if item.ExpireAt != nil && !item.ExpireAt.After(now) {
			delete(s.items, key)
			count++
		}
	}
	return count
}

func (s *ShortMemoryStore) List(userID string, taskID string, limit int) []*WorkingMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID = strings.TrimSpace(userID)
	taskID = strings.TrimSpace(taskID)
	rows := make([]*WorkingMemory, 0, len(s.items))
	for _, item := range s.items {
		if taskID != "" && item.TaskID != taskID {
			continue
		}
		if userID != "" && item.UserID != userID {
			continue
		}
		rows = append(rows, cloneWorking(item))
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	if limit <= 0 {
		limit = 20
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func cloneWorking(src *WorkingMemory) *WorkingMemory {
	if src == nil {
		return nil
	}
	dst := *src
	if src.FullHistory != nil {
		dst.FullHistory = append([]MemoryMessage(nil), src.FullHistory...)
	}
	if src.ExpireAt != nil {
		cp := *src.ExpireAt
		dst.ExpireAt = &cp
	}
	return &dst
}
