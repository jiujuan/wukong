package memory

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
	basememory "github.com/jiujuan/wukong/pkg/memory"
)

func TestStoreBackedMemoryProviderNilStoreIsSafe(t *testing.T) {
	provider := NewStoreBackedMemoryProvider(nil)

	snapshot, err := provider.Load(context.Background(), agent.RunRequest{RunID: "run-1"}, agent.AgentProfile{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot == nil || len(snapshot.Working) != 0 || len(snapshot.Long) != 0 || len(snapshot.Shared) != 0 {
		t.Fatalf("snapshot = %#v, want empty snapshot", snapshot)
	}
	if err := provider.AppendEvent(context.Background(), agent.AgentEvent{RunID: "run-1"}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	if err := provider.WriteRun(context.Background(), agent.AgentContext{}, nil, nil); err != nil {
		t.Fatalf("WriteRun() error = %v", err)
	}
}

func TestStoreBackedMemoryProviderLoadUsesTaskIDWhenRunIDEmpty(t *testing.T) {
	store := newFakeStore()
	store.seed(NamespaceWorking, "task-1", map[string]any{"summary": "from task"})
	provider := NewStoreBackedMemoryProvider(store)

	snapshot, err := provider.Load(context.Background(), agent.RunRequest{TaskID: "task-1"}, agent.AgentProfile{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Working) != 1 || snapshot.Working[0].Key != "task-1" || snapshot.Working[0].Content != "from task" {
		t.Fatalf("Working snapshot = %#v, want task fallback item", snapshot.Working)
	}
}

func TestStoreBackedMemoryProviderMemoryTopicPriority(t *testing.T) {
	tests := []struct {
		name string
		req  agent.RunRequest
		want string
	}{
		{
			name: "goal wins",
			req: agent.RunRequest{
				Goal:   "goal-topic",
				Action: "action-topic",
				Params: map[string]any{
					"topic": "param-topic",
					"query": "query-topic",
				},
			},
			want: "goal-topic",
		},
		{
			name: "topic param wins over query",
			req: agent.RunRequest{
				Action: "action-topic",
				Params: map[string]any{
					"topic": "param-topic",
					"query": "query-topic",
				},
			},
			want: "param-topic",
		},
		{
			name: "query wins over action",
			req: agent.RunRequest{
				Action: "action-topic",
				Params: map[string]any{
					"query": "query-topic",
				},
			},
			want: "query-topic",
		},
		{
			name: "action fallback",
			req:  agent.RunRequest{Action: "action-topic"},
			want: "action-topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := memoryTopic(tt.req); got != tt.want {
				t.Fatalf("memoryTopic() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStoreBackedMemoryProviderSearchDefaultsToWorkingNamespace(t *testing.T) {
	store := newFakeStore()
	store.seed(NamespaceWorking, "run-1", map[string]any{"summary": "working"})
	provider := NewStoreBackedMemoryProvider(store)

	items, err := provider.Search(context.Background(), MemoryQuery{Key: "run-1"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if store.readNamespace != NamespaceWorking || len(items) != 1 || items[0].Namespace != NamespaceWorking {
		t.Fatalf("Search() namespace/items = %s %#v, want working", store.readNamespace, items)
	}
}

func TestStoreBackedMemoryProviderWriteRunDoesNotWriteLongOnFailedEvaluation(t *testing.T) {
	store := newFakeStore()
	provider := NewStoreBackedMemoryProvider(store)

	err := provider.WriteRun(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID:     "run-1",
			TaskID:    "task-1",
			SkillName: "writer",
			Goal:      "topic-a",
		},
		Agent: agent.AgentProfile{ID: "agent-1"},
	}, &agent.ActionResult{Status: "failed", Output: "bad"}, &agent.Evaluation{Success: false})
	if err != nil {
		t.Fatalf("WriteRun() error = %v", err)
	}
	if store.updatedNamespace != NamespaceWorking || store.updatedKey != "run-1" {
		t.Fatalf("working write = %s/%s, want working/run-1", store.updatedNamespace, store.updatedKey)
	}
	if store.writtenNamespace != "" || store.writtenKey != "" {
		t.Fatalf("long write = %s/%s, want none", store.writtenNamespace, store.writtenKey)
	}
}

func TestStoreBackedMemoryProviderWritesFieldsConsumedByBaseMemory(t *testing.T) {
	store := basememory.NewManager(nil, nil)
	provider := NewStoreBackedMemoryProvider(store)
	req := agent.RunRequest{
		RunID:     "run-1",
		TaskID:    "task-1",
		UserID:    "user-1",
		SkillName: "writer",
		Goal:      "topic-a",
	}

	err := provider.WriteRun(context.Background(), agent.AgentContext{
		Request: req,
		Agent:   agent.AgentProfile{ID: "agent-1"},
	}, &agent.ActionResult{Status: "completed", Output: "final output"}, &agent.Evaluation{Success: true})
	if err != nil {
		t.Fatalf("WriteRun() error = %v", err)
	}

	working, ok, err := store.ReadMemory(context.Background(), NamespaceWorking, "run-1")
	if err != nil || !ok {
		t.Fatalf("working ReadMemory() = %#v, %v, %v; want item", working, ok, err)
	}
	if working["summary"] != "final output" {
		t.Fatalf("working summary = %#v, want final output", working["summary"])
	}

	long, ok, err := store.ReadMemory(context.Background(), NamespaceLong, "agent-1:writer:topic-a")
	if err != nil || !ok {
		t.Fatalf("long ReadMemory() = %#v, %v, %v; want item", long, ok, err)
	}
	if long["content"] != "final output" {
		t.Fatalf("long content = %#v, want final output", long["content"])
	}
}
