package handler

import (
	"context"

	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/pkg/manager"
	"github.com/jiujuan/wukong/pkg/memory"
)

type ChatService interface {
	CreateSession(ctx context.Context, userID, title, scene string) (*model.ChatSession, error)
	ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error)
	SendMessage(ctx context.Context, userID, sessionID, content, skillName string) (*model.ChatMessage, error)
	ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error)
	DeleteSession(ctx context.Context, userID, sessionID string) (bool, error)
}

type TaskService interface {
	CreateTask(ctx context.Context, userID, sessionID, skillName string, params map[string]any, priority int) (*manager.Task, error)
	ListTasks(ctx context.Context, userID, status string, page, size int) ([]*manager.Task, int64, error)
	GetTask(ctx context.Context, taskID string) (*manager.Task, error)
	CancelTask(ctx context.Context, taskID string) error
	GetSubTasks(ctx context.Context, taskID string) ([]*manager.SubTask, error)
}

type SkillService interface {
	ListSkills(ctx context.Context) ([]*model.SkillMeta, error)
}

type ToolService interface {
	ListTools(ctx context.Context) []map[string]string
}

type MemoryService interface {
	ListWorking(ctx context.Context, userID string, taskID string, limit int) []*memory.WorkingMemory
	ListLong(ctx context.Context, userID string, skillName string, keyword string, limit int) []*memory.LongTermMemory
}

type StreamAppService interface {
	SubscribeChat(ctx context.Context, sessionID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func())
	SubscribeTask(ctx context.Context, taskID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func())
	HandleTaskCommand(ctx context.Context, defaultTaskID string, action string, taskID string, content string)
}

type StreamMessage = model.StreamMessage
