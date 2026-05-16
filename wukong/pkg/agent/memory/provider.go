package memory

import (
	"context"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

const (
	// NamespaceWorking stores per-run working memory.
	NamespaceWorking = "working"
	// NamespaceLong stores long-term lessons and reusable memories.
	NamespaceLong = "long_term"
	// NamespaceShared stores shared task/run context.
	NamespaceShared = "shared"
)

// MemoryProvider loads and writes memory for Agent Loop runs.
type MemoryProvider interface {
	Load(ctx context.Context, req agent.RunRequest, profile agent.AgentProfile) (*MemorySnapshot, error)
	AppendEvent(ctx context.Context, event agent.AgentEvent) error
	WriteRun(ctx context.Context, agentCtx agent.AgentContext, result *agent.ActionResult, eval *agent.Evaluation) error
	Search(ctx context.Context, query MemoryQuery) ([]MemoryItem, error)
}

// MemorySnapshot groups memory loaded before planning.
type MemorySnapshot struct {
	Working []MemoryItem   `json:"working,omitempty"`
	Long    []MemoryItem   `json:"long,omitempty"`
	Shared  map[string]any `json:"shared,omitempty"`
}

// MemoryItem is an agent-layer memory record independent of storage details.
type MemoryItem struct {
	ID        string         `json:"id,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Key       string         `json:"key,omitempty"`
	Content   string         `json:"content,omitempty"`
	Score     float64        `json:"score,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

// MemoryQuery describes a memory lookup request.
type MemoryQuery struct {
	Namespace string         `json:"namespace,omitempty"`
	Key       string         `json:"key,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	AgentID   agent.AgentID  `json:"agent_id,omitempty"`
	SkillName string         `json:"skill_name,omitempty"`
	Query     string         `json:"query,omitempty"`
	Limit     int            `json:"limit,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// MemoryPolicy controls how Agent requests map to memory keys and writes.
type MemoryPolicy interface {
	BuildQuery(req agent.RunRequest, profile agent.AgentProfile) MemoryQuery
	ShouldWrite(event agent.AgentEvent, result *agent.ActionResult, eval *agent.Evaluation) bool
	BuildLessons(agentCtx agent.AgentContext, result *agent.ActionResult, eval *agent.Evaluation) []MemoryItem
}

// NoopMemoryProvider is a safe default memory provider.
type NoopMemoryProvider struct{}

// Load returns an empty snapshot.
func (NoopMemoryProvider) Load(ctx context.Context, _ agent.RunRequest, _ agent.AgentProfile) (*MemorySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &MemorySnapshot{}, nil
}

// AppendEvent ignores the event.
func (NoopMemoryProvider) AppendEvent(ctx context.Context, _ agent.AgentEvent) error {
	return ctx.Err()
}

// WriteRun ignores run writes.
func (NoopMemoryProvider) WriteRun(ctx context.Context, _ agent.AgentContext, _ *agent.ActionResult, _ *agent.Evaluation) error {
	return ctx.Err()
}

// Search returns no memories.
func (NoopMemoryProvider) Search(ctx context.Context, _ MemoryQuery) ([]MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
