package model

import "time"

// User 用户模型
type User struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Email     string    `json:"email,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatSession 会话模型
type ChatSession struct {
	ID        int64      `json:"id"`
	SessionID string     `json:"session_id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title,omitempty"`
	Scene     string     `json:"scene"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpireAt  *time.Time `json:"expire_at,omitempty"`
}

// ChatMessage 消息模型
type ChatMessage struct {
	ID          int64     `json:"id"`
	MsgID       string    `json:"msg_id"`
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	TaskID      string    `json:"task_id,omitempty"`
	Thought     string    `json:"thought,omitempty"`
	ToolCall    string    `json:"tool_call,omitempty"`
	ToolResult  string    `json:"tool_result,omitempty"`
	Seq         int       `json:"seq"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaskInfo 任务模型
type TaskInfo struct {
	ID          int64      `json:"id"`
	TaskID      string     `json:"task_id"`
	UserID      string     `json:"user_id"`
	SessionID   string     `json:"session_id,omitempty"`
	SkillName   string     `json:"skill_name"`
	Params      string     `json:"params"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	RetryCount  int        `json:"retry_count"`
	MaxRetry    int        `json:"max_retry"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	IsDeleted   bool       `json:"is_deleted"`
}

// TaskSub 子任务模型
type TaskSub struct {
	ID        int64     `json:"id"`
	SubTaskID string    `json:"sub_task_id"`
	TaskID    string    `json:"task_id"`
	DependsOn string    `json:"depends_on"`
	Action    string    `json:"action"`
	Params    string    `json:"params"`
	Status    string    `json:"status"`
	WorkerID  string    `json:"worker_id,omitempty"`
	Result    string    `json:"result,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkillMeta 技能模型
type SkillMeta struct {
	ID             int64     `json:"id"`
	SkillName      string    `json:"skill_name"`
	Description    string    `json:"description,omitempty"`
	Version        string    `json:"version"`
	Enabled        bool      `json:"enabled"`
	MemoryType     string    `json:"memory_type"`
	MemoryWindow   int       `json:"memory_window"`
	MemoryCompress bool      `json:"memory_compress"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type StreamMessage struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	MsgType   string    `json:"msg_type"`
	Content   string    `json:"content"`
	Seq       int       `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
}
