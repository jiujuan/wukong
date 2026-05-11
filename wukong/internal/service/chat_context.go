package service

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
)

const (
	chatSceneName                 = "chat"
	chatHistorySourceName         = "chat_history"
	chatMemorySourceName          = "chat_memory"
	chatContextBlockMemoryText    = "memory_text"
	chatContextBlockRecentHistory = "memory_recent_messages"
)

type ChatHistorySource struct {
	repo chatRepository
}

func (s *ChatHistorySource) Name() string { return chatHistorySourceName }

func (s *ChatHistorySource) Load(ctx stdctx.Context, req ctxengine.BuildRequest) ([]ctxengine.ContextBlock, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	limit := requestInt(req, "history_limit", chatRecentHistoryLimit+1)
	list, err := s.repo.ListRecentMessages(ctx, req.UserID, req.SessionID, limit)
	if err != nil {
		return nil, nil
	}

	excludeMsgID := requestString(req, "current_msg_id")
	blocks := make([]ctxengine.ContextBlock, 0, len(list))
	for _, item := range list {
		if item == nil || item.MsgID == excludeMsgID {
			continue
		}
		role := normalizeChatRole(item.Role)
		content := strings.TrimSpace(item.Content)
		if role == "" || content == "" {
			continue
		}
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:      fmt.Sprintf("history_%06d", item.Seq),
			Type:      role,
			Source:    s.Name(),
			Content:   content,
			Priority:  50,
			Timestamp: int64(item.Seq),
		})
	}
	return blocks, nil
}

type ChatMemorySource struct {
	repo chatRepository
}

func (s *ChatMemorySource) Name() string { return chatMemorySourceName }

func (s *ChatMemorySource) Load(ctx stdctx.Context, req ctxengine.BuildRequest) ([]ctxengine.ContextBlock, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}

	memory, err := s.repo.GetMemory(ctx, req.UserID, req.SessionID)
	if err != nil || memory == nil {
		return nil, nil
	}

	blocks := make([]ctxengine.ContextBlock, 0, 4)
	if text := renderChatMemory(memory); text != "" {
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:     chatContextBlockMemoryText,
			Type:     "memory",
			Source:   s.Name(),
			Content:  text,
			Priority: 90,
		})
	}
	if summary := strings.TrimSpace(memory.Summary); summary != "" {
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:     "memory_summary",
			Type:     "memory_summary",
			Source:   s.Name(),
			Content:  summary,
			Priority: 95,
		})
	}
	if profile := strings.TrimSpace(string(memory.UserProfile)); profile != "" {
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:     "user_profile",
			Type:     "memory_profile",
			Source:   s.Name(),
			Content:  profile,
			Priority: 80,
		})
	}
	if preference := strings.TrimSpace(string(memory.Preference)); preference != "" {
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:     "user_preference",
			Type:     "memory_preference",
			Source:   s.Name(),
			Content:  preference,
			Priority: 80,
		})
	}
	if len(memory.RecentMessages) > 0 {
		blocks = append(blocks, ctxengine.ContextBlock{
			Name:     chatContextBlockRecentHistory,
			Type:     "memory_recent_messages",
			Source:   s.Name(),
			Content:  string(memory.RecentMessages),
			Priority: 30,
		})
	}
	return blocks, nil
}

func newChatContextEngine(repo chatRepository) *ctxengine.Engine {
	engine := ctxengine.New()
	if engine == nil {
		return nil
	}
	_ = engine.RegisterSource(&ChatHistorySource{repo: repo})
	_ = engine.RegisterSource(&ChatMemorySource{repo: repo})
	_ = engine.RegisterScene(ctxengine.SceneConfig{
		Name:    chatSceneName,
		Sources: []string{chatMemorySourceName, chatHistorySourceName},
	})
	return engine
}

func requestString(req ctxengine.BuildRequest, key string) string {
	if req.Variables == nil {
		return ""
	}
	raw, ok := req.Variables[key]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func requestInt(req ctxengine.BuildRequest, key string, fallback int) int {
	if req.Variables == nil {
		return fallback
	}
	raw, ok := req.Variables[key]
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case int:
		if v > 0 {
			return v
		}
	case int32:
		if v > 0 {
			return int(v)
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	case json.Number:
		if parsed, err := v.Int64(); err == nil && parsed > 0 {
			return int(parsed)
		}
	}
	return fallback
}

func historyMessagesFromBundle(bundle *ctxengine.ContextBundle) []llmMessageLike {
	if bundle == nil {
		return nil
	}

	history := make([]llmMessageLike, 0, len(bundle.Blocks))
	for _, block := range bundle.Blocks {
		if block.Source != chatHistorySourceName {
			continue
		}
		role := normalizeChatRole(block.Type)
		content := strings.TrimSpace(block.Content)
		if role == "" || content == "" {
			continue
		}
		history = append(history, llmMessageLike{Role: role, Content: content})
	}
	if len(history) > 0 {
		return history
	}

	raw := strings.TrimSpace(bundle.Named[chatContextBlockRecentHistory])
	if raw == "" {
		return nil
	}
	var recent []chatMemoryMessage
	if err := json.Unmarshal([]byte(raw), &recent); err != nil {
		return nil
	}
	fallback := make([]llmMessageLike, 0, len(recent))
	for _, item := range tailMemoryMessages(recent, chatRecentHistoryLimit) {
		role := normalizeChatRole(item.Role)
		content := strings.TrimSpace(item.Content)
		if role == "" || content == "" {
			continue
		}
		fallback = append(fallback, llmMessageLike{Role: role, Content: content})
	}
	return fallback
}

func memoryTextFromBundle(bundle *ctxengine.ContextBundle) string {
	if bundle == nil {
		return ""
	}
	return strings.TrimSpace(bundle.Named[chatContextBlockMemoryText])
}

type llmMessageLike struct {
	Role    string
	Content string
}
