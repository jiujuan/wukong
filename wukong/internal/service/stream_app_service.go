package service

import (
	"context"
	"strings"
)

type StreamAppService struct {
	streamService *StreamService
	taskService   *TaskService
}

func NewStreamAppService(streamService *StreamService, taskService *TaskService) *StreamAppService {
	return &StreamAppService{
		streamService: streamService,
		taskService:   taskService,
	}
}

func (s *StreamAppService) SubscribeChat(ctx context.Context, sessionID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func()) {
	if s == nil || s.streamService == nil {
		ch := make(chan *StreamMessage)
		close(ch)
		return nil, ch, func() {}
	}
	return s.streamService.SubscribeChat(ctx, sessionID, lastSeq)
}

func (s *StreamAppService) SubscribeTask(ctx context.Context, taskID string, lastSeq int) ([]*StreamMessage, <-chan *StreamMessage, func()) {
	if s == nil || s.streamService == nil {
		ch := make(chan *StreamMessage)
		close(ch)
		return nil, ch, func() {}
	}
	return s.streamService.SubscribeTask(ctx, taskID, lastSeq)
}

func (s *StreamAppService) HandleTaskCommand(ctx context.Context, defaultTaskID string, action string, taskID string, content string) {
	if s == nil {
		return
	}
	action = strings.ToLower(strings.TrimSpace(action))
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = defaultTaskID
	}
	switch action {
	case "interrupt":
		if s.taskService != nil {
			_ = s.taskService.CancelTask(ctx, taskID)
		}
	case "inject":
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			return
		}
		if s.taskService != nil {
			_ = s.taskService.InjectTaskInstruction(ctx, taskID, trimmed)
		}
		if s.streamService != nil {
			_, _ = s.streamService.PublishTask(ctx, taskID, StreamTypeThink, trimmed)
		}
	}
}
