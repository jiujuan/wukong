package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jiujuan/wukong/internal/model"
	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
)

type recordingChatLLM struct {
	messages []llm.Message
	calls    [][]llm.Message
	reply    string
	replies  []string
}

func (r *recordingChatLLM) Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error) {
	r.messages = append([]llm.Message(nil), messages...)
	r.calls = append(r.calls, append([]llm.Message(nil), messages...))
	reply := r.reply
	if len(r.replies) > 0 {
		reply = r.replies[0]
		r.replies = r.replies[1:]
	}
	return &llm.ChatResponse{
		Choices: []llm.Choice{{
			Message: llm.Message{Role: "assistant", Content: reply},
		}},
	}, nil
}

type fakeChatRepo struct {
	sessions   map[string]*model.ChatSession
	messages   map[string][]*model.ChatMessage
	memories   map[string]*model.ChatMemory
	memoryErr  error
	recentErr  error
	upsertErr  error
	upsertCall int
}

func newFakeChatRepo() *fakeChatRepo {
	return &fakeChatRepo{
		sessions: make(map[string]*model.ChatSession),
		messages: make(map[string][]*model.ChatMessage),
		memories: make(map[string]*model.ChatMemory),
	}
}

func (r *fakeChatRepo) CreateSession(ctx context.Context, item *model.ChatSession) error {
	r.sessions[item.SessionID] = item
	return nil
}

func (r *fakeChatRepo) ListSessions(ctx context.Context, userID string, page, size int) ([]*model.ChatSession, int64, error) {
	return nil, 0, nil
}

func (r *fakeChatRepo) CreateMessage(ctx context.Context, item *model.ChatMessage) error {
	list := append(r.messages[item.SessionID], item)
	if item.Seq == 0 {
		item.Seq = len(list)
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	r.messages[item.SessionID] = list
	return nil
}

func (r *fakeChatRepo) ListMessages(ctx context.Context, userID, sessionID string, page, size int) ([]*model.ChatMessage, int64, error) {
	list := append([]*model.ChatMessage(nil), r.messages[sessionID]...)
	return list, int64(len(list)), nil
}

func (r *fakeChatRepo) ListRecentMessages(ctx context.Context, userID, sessionID string, limit int) ([]*model.ChatMessage, error) {
	if r.recentErr != nil {
		return nil, r.recentErr
	}
	list := append([]*model.ChatMessage(nil), r.messages[sessionID]...)
	if limit > 0 && len(list) > limit {
		list = list[len(list)-limit:]
	}
	return list, nil
}

func (r *fakeChatRepo) GetMemory(ctx context.Context, userID, sessionID string) (*model.ChatMemory, error) {
	if r.memoryErr != nil {
		return nil, r.memoryErr
	}
	item, ok := r.memories[sessionID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return item, nil
}

func (r *fakeChatRepo) UpsertMemory(ctx context.Context, item *model.ChatMemory) error {
	r.upsertCall++
	if r.upsertErr != nil {
		return r.upsertErr
	}
	copyItem := *item
	copyItem.RecentMessages = append([]byte(nil), item.RecentMessages...)
	copyItem.UserProfile = append([]byte(nil), item.UserProfile...)
	copyItem.Preference = append([]byte(nil), item.Preference...)
	r.memories[item.SessionID] = &copyItem
	return nil
}

func (r *fakeChatRepo) SessionExists(ctx context.Context, userID, sessionID string) (bool, error) {
	_, ok := r.sessions[sessionID]
	return ok, nil
}

func (r *fakeChatRepo) DeleteSession(ctx context.Context, userID, sessionID string) (bool, error) {
	delete(r.sessions, sessionID)
	delete(r.messages, sessionID)
	delete(r.memories, sessionID)
	return true, nil
}

func addChatMessage(r *fakeChatRepo, sessionID, userID, role, content string) {
	_ = r.CreateMessage(context.Background(), &model.ChatMessage{
		MsgID:       fmt.Sprintf("%s-%s-%d", role, content, len(r.messages[sessionID])+1),
		SessionID:   sessionID,
		UserID:      userID,
		Role:        role,
		Content:     content,
		ContentType: "TEXT",
	})
}

func TestChatHistorySourceLoadsOrderedMessages(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-source-history"
	userID := "user-source-history"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	addChatMessage(repo, sessionID, userID, "user", "first")
	addChatMessage(repo, sessionID, userID, "assistant", "second")
	addChatMessage(repo, sessionID, userID, "user", "current")

	source := &ChatHistorySource{repo: repo}
	blocks, err := source.Load(context.Background(), ctxengine.BuildRequest{
		Scene:     chatSceneName,
		UserID:    userID,
		SessionID: sessionID,
		Variables: map[string]any{
			"current_msg_id": repo.messages[sessionID][2].MsgID,
			"history_limit":  5,
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].Type != "user" || blocks[0].Content != "first" {
		t.Fatalf("unexpected first block: %+v", blocks[0])
	}
	if blocks[1].Type != "assistant" || blocks[1].Content != "second" {
		t.Fatalf("unexpected second block: %+v", blocks[1])
	}
}

func TestChatMemorySourceLoadsMemoryBlocks(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-source-memory"
	userID := "user-source-memory"
	recent, err := json.Marshal([]chatMemoryMessage{
		{Role: "user", Content: "memory-user"},
		{Role: "assistant", Content: "memory-assistant"},
	})
	if err != nil {
		t.Fatalf("marshal recent memory failed: %v", err)
	}
	repo.memories[sessionID] = &model.ChatMemory{
		SessionID:      sessionID,
		UserID:         userID,
		Summary:        "summary text",
		UserProfile:    []byte(`{"name":"Ada"}`),
		Preference:     []byte(`{"tone":"direct"}`),
		RecentMessages: recent,
	}

	source := &ChatMemorySource{repo: repo}
	blocks, err := source.Load(context.Background(), ctxengine.BuildRequest{
		Scene:     chatSceneName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(blocks) < 4 {
		t.Fatalf("expected multiple memory blocks, got %d", len(blocks))
	}
	text := blockContent(blocks, chatContextBlockMemoryText)
	if !strings.Contains(text, "会话记忆") || !strings.Contains(text, "summary text") {
		t.Fatalf("memory text block missing content: %q", text)
	}
	if blockContent(blocks, chatContextBlockRecentHistory) == "" {
		t.Fatalf("expected recent history block")
	}
}

func TestChatServiceSendMessageBuildsMultiturnContext(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-1"
	userID := "user-1"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	repo.memories[sessionID] = &model.ChatMemory{
		SessionID:   sessionID,
		UserID:      userID,
		Summary:     "we discussed the product roadmap",
		UserProfile: []byte(`{"name":"Ada","role":"builder"}`),
		Preference:  []byte(`{"tone":"direct"}`),
	}
	addChatMessage(repo, sessionID, userID, "user", "hello")
	addChatMessage(repo, sessionID, userID, "assistant", "hi there")
	addChatMessage(repo, sessionID, userID, "user", "what's new")
	addChatMessage(repo, sessionID, userID, "assistant", "not much")

	llmClient := &recordingChatLLM{reply: "context aware reply"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	msg, err := svc.SendMessage(context.Background(), userID, sessionID, "tell me more", "")
	if err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if msg == nil || msg.Content != "context aware reply" {
		t.Fatalf("unexpected assistant message: %+v", msg)
	}
	if len(llmClient.messages) != 7 {
		t.Fatalf("unexpected llm message count: %d", len(llmClient.messages))
	}
	if llmClient.messages[0].Role != "system" {
		t.Fatalf("system prompt missing: %+v", llmClient.messages[0])
	}
	if llmClient.messages[1].Role != "system" || !strings.Contains(llmClient.messages[1].Content, "we discussed the product roadmap") {
		t.Fatalf("memory prompt missing: %+v", llmClient.messages[1])
	}
	if llmClient.messages[2].Content != "hello" || llmClient.messages[5].Content != "not much" {
		t.Fatalf("history order broken: %+v", llmClient.messages)
	}
	if llmClient.messages[6].Content != "tell me more" {
		t.Fatalf("current message should be last: %+v", llmClient.messages[6])
	}
}

func TestChatServiceKeepsSameSessionContextAcrossTurns(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-same-context"
	userID := "user-same-context"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}

	llmClient := &recordingChatLLM{
		replies: []string{"好的，我记住了。", "你叫悟空范。"},
	}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	if _, err := svc.SendMessage(context.Background(), userID, sessionID, "我叫悟空范", ""); err != nil {
		t.Fatalf("first turn failed: %v", err)
	}
	if _, err := svc.SendMessage(context.Background(), userID, sessionID, "我叫什么", ""); err != nil {
		t.Fatalf("second turn failed: %v", err)
	}
	if len(llmClient.calls) != 2 {
		t.Fatalf("expected two llm calls, got %d", len(llmClient.calls))
	}

	secondCall := llmClient.calls[1]
	if !containsLLMMessage(secondCall, "user", "我叫悟空范") {
		t.Fatalf("second turn should include previous user message: %+v", secondCall)
	}
	if !containsLLMMessage(secondCall, "assistant", "好的，我记住了。") {
		t.Fatalf("second turn should include previous assistant message: %+v", secondCall)
	}
	last := secondCall[len(secondCall)-1]
	if last.Role != "user" || last.Content != "我叫什么" {
		t.Fatalf("current user message should be last, got %+v", last)
	}
}

func TestChatServiceFallsBackToMemoryRecentMessages(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-memory-fallback"
	userID := "user-memory-fallback"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	repo.recentErr = errors.New("recent history unavailable")
	recent, err := json.Marshal([]chatMemoryMessage{
		{Role: "user", Content: "我叫悟空范"},
		{Role: "assistant", Content: "好的，我记住了。"},
	})
	if err != nil {
		t.Fatalf("marshal recent memory failed: %v", err)
	}
	repo.memories[sessionID] = &model.ChatMemory{
		SessionID:      sessionID,
		UserID:         userID,
		RecentMessages: recent,
	}

	llmClient := &recordingChatLLM{reply: "你叫悟空范。"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	if _, err := svc.SendMessage(context.Background(), userID, sessionID, "我叫什么", ""); err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if !containsLLMMessage(llmClient.messages, "user", "我叫悟空范") {
		t.Fatalf("memory fallback should include previous user message: %+v", llmClient.messages)
	}
	if !containsLLMMessage(llmClient.messages, "assistant", "好的，我记住了。") {
		t.Fatalf("memory fallback should include previous assistant message: %+v", llmClient.messages)
	}
}

func TestChatServiceFallsBackWhenMemoryMissing(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-2"
	userID := "user-2"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	repo.memoryErr = errors.New("memory lookup failed")
	addChatMessage(repo, sessionID, userID, "user", "first turn")

	llmClient := &recordingChatLLM{reply: "fallback ok"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	_, err := svc.SendMessage(context.Background(), userID, sessionID, "second turn", "")
	if err != nil {
		t.Fatalf("send message should fallback, got err: %v", err)
	}
	if len(llmClient.messages) != 3 {
		t.Fatalf("unexpected llm message count: %d", len(llmClient.messages))
	}
	if llmClient.messages[0].Role != "system" {
		t.Fatalf("system prompt missing: %+v", llmClient.messages)
	}
	if llmClient.messages[1].Content != "first turn" || llmClient.messages[2].Content != "second turn" {
		t.Fatalf("fallback context broken: %+v", llmClient.messages)
	}
}

func TestChatServiceTruncatesOldHistory(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-3"
	userID := "user-3"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	for i := 1; i <= 14; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		addChatMessage(repo, sessionID, userID, role, fmt.Sprintf("msg-%d", i))
	}

	llmClient := &recordingChatLLM{reply: "trimmed"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	_, err := svc.SendMessage(context.Background(), userID, sessionID, "current", "")
	if err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if len(llmClient.messages) != 14 {
		t.Fatalf("unexpected llm message count: %d", len(llmClient.messages))
	}
	if llmClient.messages[1].Content != "msg-3" {
		t.Fatalf("old history should be trimmed, got first history: %s", llmClient.messages[1].Content)
	}
	if llmClient.messages[len(llmClient.messages)-1].Content != "current" {
		t.Fatalf("current message should remain last")
	}
}

func TestChatServicePersistsMemoryWindowAndSummary(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-4"
	userID := "user-4"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	repo.memories[sessionID] = &model.ChatMemory{
		SessionID:   sessionID,
		UserID:      userID,
		Summary:     "earlier summary",
		UserProfile: []byte(`{"tier":"pro"}`),
		Preference:  []byte(`{"style":"concise"}`),
	}
	for i := 1; i <= 14; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		addChatMessage(repo, sessionID, userID, role, fmt.Sprintf("turn-%d", i))
	}

	llmClient := &recordingChatLLM{reply: "final answer"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	_, err := svc.SendMessage(context.Background(), userID, sessionID, "turn-15", "")
	if err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if repo.upsertCall == 0 {
		t.Fatalf("expected chat memory to be written back")
	}

	memory := repo.memories[sessionID]
	if memory == nil {
		t.Fatalf("expected persisted memory")
	}
	if !strings.Contains(memory.Summary, "earlier summary") || !strings.Contains(memory.Summary, "turn-1") {
		t.Fatalf("unexpected summary: %s", memory.Summary)
	}
	if string(memory.UserProfile) != `{"tier":"pro"}` || string(memory.Preference) != `{"style":"concise"}` {
		t.Fatalf("expected existing memory metadata preserved: %+v", memory)
	}

	var recent []chatMemoryMessage
	if err := json.Unmarshal(memory.RecentMessages, &recent); err != nil {
		t.Fatalf("recent_messages should be valid json: %v", err)
	}
	if len(recent) != chatMemoryWindowLimit {
		t.Fatalf("unexpected recent window size: %d", len(recent))
	}
	if recent[0].Content != "turn-5" {
		t.Fatalf("recent window should retain newest messages, got first=%s", recent[0].Content)
	}
	if recent[len(recent)-1].Content != "final answer" {
		t.Fatalf("assistant reply should be included in recent window, got last=%s", recent[len(recent)-1].Content)
	}
}

func TestChatServiceIgnoresMemoryWriteFailures(t *testing.T) {
	repo := newFakeChatRepo()
	sessionID := "session-5"
	userID := "user-5"
	repo.sessions[sessionID] = &model.ChatSession{SessionID: sessionID, UserID: userID}
	repo.upsertErr = errors.New("write failed")

	llmClient := &recordingChatLLM{reply: "still works"}
	svc := &ChatService{repo: repo, llmProvider: llmClient}

	msg, err := svc.SendMessage(context.Background(), userID, sessionID, "hello", "")
	if err != nil {
		t.Fatalf("memory write failure should not break chat: %v", err)
	}
	if msg == nil || msg.Content != "still works" {
		t.Fatalf("unexpected assistant message: %+v", msg)
	}
}

func containsLLMMessage(messages []llm.Message, role, content string) bool {
	for _, message := range messages {
		if message.Role == role && message.Content == content {
			return true
		}
	}
	return false
}

func blockContent(blocks []ctxengine.ContextBlock, name string) string {
	for _, block := range blocks {
		if block.Name == name {
			return block.Content
		}
	}
	return ""
}
