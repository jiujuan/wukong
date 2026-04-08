package uuid

import (
	"github.com/google/uuid"
)

// New 生成新的UUID
func New() string {
	return uuid.New().String()
}

// NewShort 生成短UUID (8位)
func NewShort() string {
	id := uuid.New()
	return id.String()[:8]
}

// NewTaskID 生成任务ID
func NewTaskID() string {
	return "task_" + uuid.New().String()[:12]
}

// NewSessionID 生成会话ID
func NewSessionID() string {
	return "sess_" + uuid.New().String()[:12]
}

// NewMsgID 生成消息ID
func NewMsgID() string {
	return "msg_" + uuid.New().String()[:12]
}

// NewSubTaskID 生成子任务ID
func NewSubTaskID() string {
	return "sub_" + uuid.New().String()[:12]
}

// NewUserID 生成用户ID
func NewUserID() string {
	return "user_" + uuid.New().String()[:12]
}

// Parse 解析UUID
func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// IsValid 验证UUID格式
func IsValid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
