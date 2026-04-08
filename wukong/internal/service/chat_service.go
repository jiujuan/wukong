package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/internal/repository"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/uuid"
)

type ChatService struct {
	repo          *repository.ChatRepository
	llmProvider   *llm.Provider
	streamService *StreamService
}

func NewChatService(repo *repository.ChatRepository, llmProvider *llm.Provider, streamService *StreamService) *ChatService {
	return &ChatService{
		repo:          repo,
		llmProvider:   llmProvider,
		streamService: streamService,
	}
}

func (s *ChatService) CreateSession(ctx context.Context, userID, title, scene string) (*model.ChatSession, error) {
	if s == nil || s.repo == nil {
		return nil, errors.ErrServerError
	}
	if strings.TrimSpace(userID) == "" {
		return nil, errors.ErrUnauthorized
	}
	if strings.TrimSpace(scene) == "" {
		scene = "CHAT"
	}
	if strings.TrimSpace(title) == "" {
		title = "新会话"
	}
	item := &model.ChatSession{
		SessionID: uuid.NewSessionID(),
		UserID:    userID,
		Title:     title,
		Scene:     scene,
		Status:    "OPEN",
	}
	if err := s.repo.CreateSession(ctx, item); err != nil {
		return nil, fmt.Errorf("chat service create session failed: %w", err)
	}
	return item, nil
}

func (s *ChatService) ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.ErrServerError
	}
	if strings.TrimSpace(userID) == "" {
		return nil, 0, errors.ErrUnauthorized
	}
	return s.repo.ListSessions(ctx, userID, page, size)
}

func (s *ChatService) SendMessage(ctx context.Context, userID, sessionID, content, skillName string) (*model.ChatMessage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.ErrServerError
	}
	if strings.TrimSpace(userID) == "" {
		return nil, errors.ErrUnauthorized
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.ErrBadRequest
	}
	ok, err := s.repo.SessionExists(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.ErrSessionNotFound
	}

	userMsg := &model.ChatMessage{
		MsgID:       uuid.NewMsgID(),
		SessionID:   sessionID,
		UserID:      userID,
		Role:        "user",
		Content:     content,
		ContentType: "TEXT",
	}
	if err := s.repo.CreateMessage(ctx, userMsg); err != nil {
		return nil, err
	}

	reply := "收到您的消息: " + content
	if s.llmProvider != nil && strings.TrimSpace(skillName) == "" {
		resp, chatErr := s.llmProvider.Chat(ctx, []llm.Message{{Role: "user", Content: content}})
		if chatErr != nil {
			return nil, chatErr
		}
		if len(resp.Choices) > 0 {
			reply = resp.Choices[0].Message.Content
		}
	}

	assistantMsg := &model.ChatMessage{
		MsgID:       uuid.NewMsgID(),
		SessionID:   sessionID,
		UserID:      userID,
		Role:        "assistant",
		Content:     reply,
		ContentType: "TEXT",
	}
	if err := s.repo.CreateMessage(ctx, assistantMsg); err != nil {
		return nil, err
	}

	if s.streamService != nil {
		_, _ = s.streamService.PublishChat(ctx, sessionID, StreamTypeChunk, reply)
		_, _ = s.streamService.PublishChat(ctx, sessionID, StreamTypeFinish, "chat finished")
	}
	return assistantMsg, nil
}

func (s *ChatService) ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.ErrServerError
	}
	if strings.TrimSpace(userID) == "" {
		return nil, 0, errors.ErrUnauthorized
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, 0, errors.ErrBadRequest
	}
	return s.repo.ListMessages(ctx, userID, sessionID, page, size)
}

func (s *ChatService) DeleteSession(ctx context.Context, userID, sessionID string) (bool, error) {
	if s == nil || s.repo == nil {
		return false, errors.ErrServerError
	}
	if strings.TrimSpace(userID) == "" {
		return false, errors.ErrUnauthorized
	}
	if strings.TrimSpace(sessionID) == "" {
		return false, errors.ErrBadRequest
	}
	return s.repo.DeleteSession(ctx, userID, sessionID)
}
