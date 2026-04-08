package memory

import (
	"context"
	"time"
)

const (
	NamespaceWorking = "working"
	NamespaceLong    = "long_term"
	NamespaceShared  = "shared"
)

type MemoryMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkingMemory struct {
	TaskID       string          `json:"task_id"`
	UserID       string          `json:"user_id"`
	FullHistory  []MemoryMessage `json:"full_history"`
	Summary      string          `json:"summary"`
	WindowSize   int             `json:"window_size"`
	CompressFlag bool            `json:"compress_flag"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	ExpireAt     *time.Time      `json:"expire_at,omitempty"`
}

type LongTermMemory struct {
	MemoryID     string    `json:"memory_id"`
	UserID       string    `json:"user_id"`
	SkillName    string    `json:"skill_name"`
	Topic        string    `json:"topic"`
	Content      string    `json:"content"`
	SourceTaskID string    `json:"source_task_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type SharedMemory struct {
	ShareKey    string         `json:"share_key"`
	Data        map[string]any `json:"data"`
	OwnerTaskID string         `json:"owner_task_id"`
	ReadOnly    bool           `json:"read_only"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ExpireAt    *time.Time     `json:"expire_at,omitempty"`
}

type Memory interface {
	WriteMemory(ctx context.Context, namespace, key string, value map[string]any) error
	ReadMemory(ctx context.Context, namespace, key string) (map[string]any, bool, error)
	UpdateMemory(ctx context.Context, namespace, key string, value map[string]any) error
	DeleteMemory(ctx context.Context, namespace, key string) error
	CompressMemory(ctx context.Context, taskID string) (string, error)
	SessionArchive(ctx context.Context, taskID string, skillName string, topic string) (*LongTermMemory, error)
	SharedMemorySync(ctx context.Context, shareKey string, patch map[string]any) error
	MemoryExpire(ctx context.Context, now time.Time) (int, error)
}
