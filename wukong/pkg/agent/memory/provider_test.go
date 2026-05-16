package memory

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestNoopMemoryProviderDoesNotError(t *testing.T) {
	provider := NoopMemoryProvider{}
	if snapshot, err := provider.Load(context.Background(), agent.RunRequest{}, agent.AgentProfile{}); err != nil || snapshot == nil {
		t.Fatalf("Load() = %#v, %v; want empty snapshot", snapshot, err)
	}
	if err := provider.AppendEvent(context.Background(), agent.AgentEvent{Type: "test"}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	if err := provider.WriteRun(context.Background(), agent.AgentContext{}, nil, nil); err != nil {
		t.Fatalf("WriteRun() error = %v", err)
	}
	if items, err := provider.Search(context.Background(), MemoryQuery{}); err != nil || len(items) != 0 {
		t.Fatalf("Search() = %#v, %v; want empty", items, err)
	}
}

func TestStoreBackedMemoryProviderLoadAndWriteRun(t *testing.T) {
	store := newFakeStore()
	store.seed(NamespaceWorking, "run-1", map[string]any{"task_id": "task-1", "summary": "working"})
	store.seed(NamespaceLong, "agent-1:writer:topic-a", map[string]any{"memory_id": "long-1", "content": "lesson"})
	store.seed(NamespaceShared, "parent-1", map[string]any{"shared": "value"})

	provider := NewStoreBackedMemoryProvider(store)
	req := agent.RunRequest{
		RunID:       "run-1",
		TaskID:      "task-1",
		UserID:      "user-1",
		SkillName:   "writer",
		Goal:        "topic-a",
		ParentRunID: "parent-1",
	}
	snapshot, err := provider.Load(context.Background(), req, agent.AgentProfile{ID: "agent-1"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.Working) != 1 || snapshot.Working[0].Namespace != NamespaceWorking || snapshot.Working[0].Content != "working" {
		t.Fatalf("Working snapshot = %#v", snapshot.Working)
	}
	if len(snapshot.Long) != 1 || snapshot.Long[0].Namespace != NamespaceLong || snapshot.Long[0].Content != "lesson" {
		t.Fatalf("Long snapshot = %#v", snapshot.Long)
	}
	if snapshot.Shared["shared"] != "value" {
		t.Fatalf("Shared snapshot = %#v", snapshot.Shared)
	}

	err = provider.WriteRun(context.Background(), agent.AgentContext{
		Request: req,
		Agent:   agent.AgentProfile{ID: "agent-1"},
	}, &agent.ActionResult{Status: "completed", Output: "final output"}, &agent.Evaluation{Success: true})
	if err != nil {
		t.Fatalf("WriteRun() error = %v", err)
	}

	if store.updatedNamespace != NamespaceWorking || store.updatedKey != "run-1" {
		t.Fatalf("working write = %s/%s, want working/run-1", store.updatedNamespace, store.updatedKey)
	}
	if store.writtenNamespace != NamespaceLong || store.writtenKey != "agent-1:writer:topic-a" {
		t.Fatalf("long write = %s/%s, want long key", store.writtenNamespace, store.writtenKey)
	}
}

func TestStoreBackedMemoryProviderNamespaceMapping(t *testing.T) {
	store := newFakeStore()
	provider := NewStoreBackedMemoryProvider(store)

	_, _ = provider.Search(context.Background(), MemoryQuery{Namespace: NamespaceShared, Key: "share-1"})
	if store.readNamespace != NamespaceShared || store.readKey != "share-1" {
		t.Fatalf("search read = %s/%s, want shared/share-1", store.readNamespace, store.readKey)
	}

	_ = provider.AppendEvent(context.Background(), agent.AgentEvent{RunID: "run-1", Type: "phase"})
	if store.updatedNamespace != NamespaceWorking || store.updatedKey != "run-1" {
		t.Fatalf("event write = %s/%s, want working/run-1", store.updatedNamespace, store.updatedKey)
	}
}

type fakeStore struct {
	values map[string]map[string]map[string]any

	readNamespace string
	readKey       string

	writtenNamespace string
	writtenKey       string

	updatedNamespace string
	updatedKey       string
}

func newFakeStore() *fakeStore {
	return &fakeStore{values: make(map[string]map[string]map[string]any)}
}

func (s *fakeStore) seed(namespace, key string, value map[string]any) {
	if s.values[namespace] == nil {
		s.values[namespace] = make(map[string]map[string]any)
	}
	s.values[namespace][key] = cloneMap(value)
}

func (s *fakeStore) WriteMemory(_ context.Context, namespace, key string, value map[string]any) error {
	s.writtenNamespace = namespace
	s.writtenKey = key
	s.seed(namespace, key, value)
	return nil
}

func (s *fakeStore) ReadMemory(_ context.Context, namespace, key string) (map[string]any, bool, error) {
	s.readNamespace = namespace
	s.readKey = key
	value, ok := s.values[namespace][key]
	return cloneMap(value), ok, nil
}

func (s *fakeStore) UpdateMemory(_ context.Context, namespace, key string, value map[string]any) error {
	s.updatedNamespace = namespace
	s.updatedKey = key
	s.seed(namespace, key, value)
	return nil
}
