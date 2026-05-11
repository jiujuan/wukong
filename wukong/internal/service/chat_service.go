package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jiujuan/wukong/internal/model"
	"github.com/jiujuan/wukong/pkg/errors"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/uuid"
)

type chatRepository interface {
	CreateSession(ctx context.Context, item *model.ChatSession) error
	ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error)
	CreateMessage(ctx context.Context, item *model.ChatMessage) error
	ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error)
	ListRecentMessages(ctx context.Context, userID, sessionID string, limit int) ([]*model.ChatMessage, error)
	GetMemory(ctx context.Context, userID, sessionID string) (*model.ChatMemory, error)
	UpsertMemory(ctx context.Context, item *model.ChatMemory) error
	SessionExists(ctx context.Context, userID, sessionID string) (bool, error)
	DeleteSession(ctx context.Context, userID, sessionID string) (bool, error)
}

type chatLLM interface {
	Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error)
}

type chatStreamer interface {
	StreamChat(ctx context.Context, messages []llm.Message, handler func(chunk string)) error
}

type ChatService struct {
	repo          chatRepository
	llmProvider   chatLLM
	streamService *StreamService
}

func NewChatService(repo chatRepository, llmProvider *llm.Provider, streamService *StreamService) *ChatService {
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
	streamed := false
	if s.llmProvider != nil && strings.TrimSpace(skillName) == "" {
		messages := s.buildLLMMessages(ctx, userID, sessionID, userMsg.MsgID, content)
		if streamer, ok := s.llmProvider.(chatStreamer); ok {
			streamedReply, chatErr := s.streamChatReply(ctx, sessionID, messages, streamer)
			if chatErr != nil {
				return nil, chatErr
			}
			if strings.TrimSpace(streamedReply) != "" {
				reply = streamedReply
				streamed = true
			}
		} else {
			resp, chatErr := s.llmProvider.Chat(ctx, messages)
			if chatErr != nil {
				return nil, chatErr
			}
			if len(resp.Choices) > 0 {
				reply = resp.Choices[0].Message.Content
			}
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

	s.persistChatMemory(ctx, userID, sessionID)

	if s.streamService != nil {
		if !streamed {
			_, _ = s.streamService.PublishChat(ctx, sessionID, StreamTypeChunk, reply)
		}
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

const (
	chatRecentHistoryLimit = 12
	chatMemoryWindowLimit  = 12
	chatSummarySourceLimit = 20
	chatSystemPrompt       = "你是 Wukong 的对话助手。请结合会话记忆和历史对话回答。若记忆与历史冲突，以更近的消息为准。"
)

func (s *ChatService) buildLLMMessages(ctx context.Context, userID, sessionID, currentMsgID, content string) []llm.Message {
	messages := make([]llm.Message, 0, 16)
	messages = append(messages, llm.Message{Role: "system", Content: chatSystemPrompt})

	if memory := s.loadChatMemory(ctx, userID, sessionID); memory != nil {
		if memoryText := renderChatMemory(memory); memoryText != "" {
			messages = append(messages, llm.Message{Role: "system", Content: memoryText})
		}
	}

	if history := s.loadRecentMessages(ctx, userID, sessionID, chatRecentHistoryLimit+1); len(history) > 0 {
		for _, item := range history {
			if item == nil || item.MsgID == currentMsgID {
				continue
			}
			role := normalizeChatRole(item.Role)
			if role == "" {
				continue
			}
			msgContent := strings.TrimSpace(item.Content)
			if msgContent == "" {
				continue
			}
			messages = append(messages, llm.Message{Role: role, Content: msgContent})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: strings.TrimSpace(content)})
	return messages
}

func (s *ChatService) streamChatReply(ctx context.Context, sessionID string, messages []llm.Message, streamer chatStreamer) (string, error) {
	if s == nil || streamer == nil {
		return "", nil
	}
	var builder strings.Builder
	err := streamer.StreamChat(ctx, messages, func(chunk string) {
		if chunk == "" {
			return
		}
		builder.WriteString(chunk)
		if s.streamService != nil {
			_, _ = s.streamService.PublishChat(ctx, sessionID, StreamTypeChunk, chunk)
		}
	})
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

func (s *ChatService) loadRecentMessages(ctx context.Context, userID, sessionID string, limit int) []*model.ChatMessage {
	if s == nil || s.repo == nil {
		return nil
	}
	messages, err := s.repo.ListRecentMessages(ctx, userID, sessionID, limit)
	if err != nil {
		return nil
	}
	return messages
}

func (s *ChatService) loadChatMemory(ctx context.Context, userID, sessionID string) *model.ChatMemory {
	if s == nil || s.repo == nil {
		return nil
	}
	memory, err := s.repo.GetMemory(ctx, userID, sessionID)
	if err != nil {
		return nil
	}
	return memory
}

func renderChatMemory(memory *model.ChatMemory) string {
	if memory == nil {
		return ""
	}
	sections := make([]string, 0, 4)
	if summary := strings.TrimSpace(memory.Summary); summary != "" {
		sections = append(sections, "摘要:\n"+summary)
	}
	if profile := strings.TrimSpace(string(memory.UserProfile)); profile != "" {
		sections = append(sections, "用户画像:\n"+profile)
	}
	if preference := strings.TrimSpace(string(memory.Preference)); preference != "" {
		sections = append(sections, "用户偏好:\n"+preference)
	}
	if len(sections) == 0 {
		return ""
	}
	return "会话记忆:\n" + strings.Join(sections, "\n\n")
}

func normalizeChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "user", "system":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return ""
	}
}

type chatMemoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Seq       int    `json:"seq,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func (s *ChatService) persistChatMemory(ctx context.Context, userID, sessionID string) {
	if s == nil || s.repo == nil {
		return
	}

	existing := s.loadChatMemory(ctx, userID, sessionID)
	history := s.loadRecentMessages(ctx, userID, sessionID, chatSummarySourceLimit)
	if len(history) == 0 {
		return
	}

	recentWindow := tailMessages(history, chatMemoryWindowLimit)
	recentJSON, err := json.Marshal(toChatMemoryMessages(recentWindow))
	if err != nil {
		return
	}

	summary := ""
	if existing != nil {
		summary = strings.TrimSpace(existing.Summary)
	}
	if len(history) > chatMemoryWindowLimit {
		olderMessages := history[:len(history)-len(recentWindow)]
		summary = mergeChatSummary(summary, olderMessages)
	}

	memory := &model.ChatMemory{
		SessionID:      sessionID,
		UserID:         userID,
		RecentMessages: recentJSON,
		Summary:        summary,
	}
	if existing != nil {
		memory.UserProfile = append(json.RawMessage(nil), existing.UserProfile...)
		memory.Preference = append(json.RawMessage(nil), existing.Preference...)
	}
	_ = s.repo.UpsertMemory(ctx, memory)
}

func tailMessages(list []*model.ChatMessage, limit int) []*model.ChatMessage {
	if limit < 1 || len(list) <= limit {
		return append([]*model.ChatMessage(nil), list...)
	}
	return append([]*model.ChatMessage(nil), list[len(list)-limit:]...)
}

func toChatMemoryMessages(list []*model.ChatMessage) []chatMemoryMessage {
	result := make([]chatMemoryMessage, 0, len(list))
	for _, item := range list {
		if item == nil {
			continue
		}
		role := normalizeChatRole(item.Role)
		content := strings.TrimSpace(item.Content)
		if role == "" || content == "" {
			continue
		}
		msg := chatMemoryMessage{
			Role:    role,
			Content: content,
			Seq:     item.Seq,
		}
		if !item.CreatedAt.IsZero() {
			msg.CreatedAt = item.CreatedAt.Format(time.RFC3339)
		}
		result = append(result, msg)
	}
	return result
}

func mergeChatSummary(existing string, messages []*model.ChatMessage) string {
	existing = strings.TrimSpace(existing)
	lines := make([]string, 0, len(messages)+1)
	if existing != "" {
		lines = append(lines, existing)
	}
	for _, item := range messages {
		if item == nil {
			continue
		}
		role := normalizeChatRole(item.Role)
		content := compactChatContent(item.Content, 120)
		if role == "" || content == "" {
			continue
		}
		lines = append(lines, role+": "+content)
	}
	if len(lines) == 0 {
		return ""
	}
	return compactChatContent(strings.Join(lines, "\n"), 4000)
}

func compactChatContent(content string, limit int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if limit < 1 || len(normalized) <= limit {
		return normalized
	}
	if limit <= 3 {
		return normalized[:limit]
	}
	return normalized[:limit-3] + "..."
}
