package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
	basememory "github.com/jiujuan/wukong/pkg/memory"
)

// Store is the minimal base memory dependency used by StoreBackedMemoryProvider.
type Store interface {
	WriteMemory(ctx context.Context, namespace, key string, value map[string]any) error
	ReadMemory(ctx context.Context, namespace, key string) (map[string]any, bool, error)
	UpdateMemory(ctx context.Context, namespace, key string, value map[string]any) error
}

// StoreBackedMemoryProvider adapts the base memory store to Agent memory.
type StoreBackedMemoryProvider struct {
	store Store
}

// NewStoreBackedMemoryProvider creates a provider backed by a base memory store.
func NewStoreBackedMemoryProvider(store Store) *StoreBackedMemoryProvider {
	return &StoreBackedMemoryProvider{store: store}
}

// Load reads working, long, and shared memory using the default key mapping.
func (p *StoreBackedMemoryProvider) Load(ctx context.Context, req agent.RunRequest, profile agent.AgentProfile) (*agent.MemorySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.store == nil {
		return &agent.MemorySnapshot{}, nil
	}

	snapshot := &agent.MemorySnapshot{}
	workingKey := workingKey(req)
	if workingKey != "" {
		item, ok, err := p.readItem(ctx, NamespaceWorking, workingKey)
		if err != nil {
			return nil, err
		}
		if ok {
			snapshot.Working = append(snapshot.Working, item)
		}
	}

	longKey := longKey(req, profile)
	if longKey != "" {
		item, ok, err := p.readItem(ctx, NamespaceLong, longKey)
		if err != nil {
			return nil, err
		}
		if ok {
			snapshot.Long = append(snapshot.Long, item)
		}
	}

	sharedKey := sharedKey(req)
	if sharedKey != "" {
		value, ok, err := p.store.ReadMemory(ctx, NamespaceShared, sharedKey)
		if err != nil {
			return nil, err
		}
		if ok {
			snapshot.Shared = cloneMap(value)
		}
	}
	return snapshot, nil
}

// AppendEvent writes a trace event to working memory.
func (p *StoreBackedMemoryProvider) AppendEvent(ctx context.Context, event agent.AgentEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.store == nil {
		return nil
	}
	key := event.RunID
	if key == "" {
		key = event.Type
	}
	return p.store.UpdateMemory(ctx, NamespaceWorking, key, map[string]any{
		"event_type": event.Type,
		"message":    event.Message,
		"role":       "system",
		"content":    eventContent(event),
		"metadata":   cloneMap(event.Metadata),
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

// WriteRun writes a run summary to working memory and successful lessons to long memory.
func (p *StoreBackedMemoryProvider) WriteRun(ctx context.Context, agentCtx agent.AgentContext, result *agent.ActionResult, eval *agent.Evaluation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.store == nil {
		return nil
	}
	req := agentCtx.Request
	runKey := workingKey(req)
	if runKey != "" {
		if err := p.store.UpdateMemory(ctx, NamespaceWorking, runKey, map[string]any{
			"task_id":    req.TaskID,
			"run_id":     req.RunID,
			"status":     actionStatus(result),
			"output":     actionOutput(result),
			"summary":    actionOutput(result),
			"role":       "assistant",
			"content":    actionOutput(result),
			"updated_at": time.Now().Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	if eval != nil && eval.Success {
		key := longKey(req, agentCtx.Agent)
		if key != "" {
			return p.store.WriteMemory(ctx, NamespaceLong, key, map[string]any{
				"user_id":        req.UserID,
				"skill_name":     req.SkillName,
				"topic":          memoryTopic(req),
				"content":        actionOutput(result),
				"source_task_id": req.TaskID,
				"created_at":     time.Now().Format(time.RFC3339),
			})
		}
	}
	return nil
}

// Search reads one memory item by namespace/key.
func (p *StoreBackedMemoryProvider) Search(ctx context.Context, query MemoryQuery) ([]MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.store == nil || query.Key == "" {
		return nil, nil
	}
	namespace := query.Namespace
	if namespace == "" {
		namespace = NamespaceWorking
	}
	item, ok, err := p.readItem(ctx, namespace, query.Key)
	if err != nil || !ok {
		return nil, err
	}
	return []MemoryItem{item}, nil
}

func (p *StoreBackedMemoryProvider) readItem(ctx context.Context, namespace, key string) (MemoryItem, bool, error) {
	value, ok, err := p.store.ReadMemory(ctx, namespace, key)
	if err != nil || !ok {
		return MemoryItem{}, false, err
	}
	return mapToMemoryItem(namespace, key, value), true, nil
}

func mapToMemoryItem(namespace, key string, value map[string]any) MemoryItem {
	return MemoryItem{
		ID:        stringValue(value, "memory_id", "id", "task_id", "share_key"),
		Namespace: namespace,
		Key:       key,
		Content:   stringValue(value, "content", "summary", "output"),
		Metadata:  cloneMap(value),
	}
}

func workingKey(req agent.RunRequest) string {
	if req.RunID != "" {
		return req.RunID
	}
	return req.TaskID
}

func longKey(req agent.RunRequest, profile agent.AgentProfile) string {
	topic := memoryTopic(req)
	if topic == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", profile.ID, req.SkillName, topic)
}

func sharedKey(req agent.RunRequest) string {
	if req.ParentRunID != "" {
		return req.ParentRunID
	}
	return req.TaskID
}

func memoryTopic(req agent.RunRequest) string {
	if req.Goal != "" {
		return req.Goal
	}
	if topic, ok := req.Params["topic"].(string); ok {
		return topic
	}
	if query, ok := req.Params["query"].(string); ok {
		return query
	}
	return req.Action
}

func actionStatus(result *agent.ActionResult) string {
	if result == nil {
		return ""
	}
	return result.Status
}

func actionOutput(result *agent.ActionResult) string {
	if result == nil {
		return ""
	}
	return result.Output
}

func eventContent(event agent.AgentEvent) string {
	if event.Message != "" {
		return event.Message
	}
	return event.Type
}

func stringValue(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := value[key].(string); ok && raw != "" {
			return raw
		}
	}
	return ""
}

var _ Store = (basememory.Memory)(nil)
var _ agent.LoopMemoryProvider = (*StoreBackedMemoryProvider)(nil)
