package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type MemoryStore interface {
	Write(ctx context.Context, namespace, key string, value map[string]any) error
	Read(ctx context.Context, namespace, key string) (map[string]any, bool, error)
}

type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string]map[string]map[string]any
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]map[string]map[string]any),
	}
}

func (s *InMemoryStore) Write(_ context.Context, namespace, key string, value map[string]any) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("namespace or key empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[namespace]; !ok {
		s.data[namespace] = make(map[string]map[string]any)
	}
	cp := make(map[string]any, len(value))
	for k, v := range value {
		cp[k] = v
	}
	s.data[namespace][key] = cp
	return nil
}

func (s *InMemoryStore) Read(_ context.Context, namespace, key string) (map[string]any, bool, error) {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(key) == "" {
		return nil, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.data[namespace]
	if !ok {
		return nil, false, nil
	}
	item, ok := group[key]
	if !ok {
		return nil, false, nil
	}
	cp := make(map[string]any, len(item))
	for k, v := range item {
		cp[k] = v
	}
	return cp, true, nil
}
