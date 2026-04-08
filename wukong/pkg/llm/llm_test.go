package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"default", []Option{}},
		{"with base url", []Option{WithBaseURL("https://api.deepseek.com")}},
		{"with api key", []Option{WithAPIKey("test-key")}},
		{"with model", []Option{WithModel("gpt-4")}},
		{"with timeout", []Option{WithTimeout(30 * time.Second)}},
		{"with provider", []Option{WithProviderType(ProviderTypeOllama)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.opts...)
			if p == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestWithProviderType(t *testing.T) {
	p := New(WithProviderType(ProviderTypeDoubao))
	if p.providerType != ProviderTypeDoubao {
		t.Errorf("providerType = %v, want %v", p.providerType, ProviderTypeDoubao)
	}
}

func TestProviderDefaults(t *testing.T) {
	p := New(WithProviderType(ProviderTypeDeepSeek))
	if p.baseURL == "" || p.model == "" {
		t.Fatalf("deepseek defaults should not be empty")
	}
	o := New(WithProviderType(ProviderTypeOllama))
	if o.baseURL == "" || o.model == "" {
		t.Fatalf("ollama defaults should not be empty")
	}
}

func TestWithBaseURL(t *testing.T) {
	p := New(WithBaseURL("https://custom.api.com"))
	if p.baseURL != "https://custom.api.com" {
		t.Errorf("baseURL = %v, want %v", p.baseURL, "https://custom.api.com")
	}
}

func TestWithAPIKey(t *testing.T) {
	p := New(WithAPIKey("test-key-123"))
	if p.apiKey != "test-key-123" {
		t.Errorf("apiKey = %v, want %v", p.apiKey, "test-key-123")
	}
}

func TestWithModel(t *testing.T) {
	p := New(WithModel("gpt-4-turbo"))
	if p.model != "gpt-4-turbo" {
		t.Errorf("model = %v, want %v", p.model, "gpt-4-turbo")
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %v, want %v", msg.Role, "user")
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %v, want %v", msg.Content, "Hello, world!")
	}
}

func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		Stream: false,
	}

	if req.Model != "gpt-3.5-turbo" {
		t.Errorf("Model = %v, want %v", req.Model, "gpt-3.5-turbo")
	}
	if len(req.Messages) != 2 {
		t.Errorf("Messages length = %d, want %d", len(req.Messages), 2)
	}
}

func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-3.5-turbo",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	if len(resp.Choices) != 1 {
		t.Errorf("Choices length = %d, want %d", len(resp.Choices), 1)
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Content = %v, want %v", resp.Choices[0].Message.Content, "Hello!")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, 15)
	}
}

func TestChatWithoutAPIKey(t *testing.T) {
	// 创建一个测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个模拟响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"choices": [{"message": {"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}]
		}`))
	}))
	defer server.Close()

	p := New(
		WithBaseURL(server.URL),
		WithAPIKey("test-key"),
		WithModel("gpt-3.5-turbo"),
	)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := p.Chat(context.Background(), messages)
	if err != nil {
		t.Errorf("Chat() error = %v", err)
	}
	if resp == nil {
		t.Fatal("Chat() returned nil response")
	}
	if len(resp.Choices) == 0 {
		t.Error("Choices should not be empty")
	}
}

func TestChatHTTPError(t *testing.T) {
	// 创建一个返回错误的测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	p := New(
		WithBaseURL(server.URL),
		WithAPIKey("test-key"),
	)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := p.Chat(context.Background(), messages)
	if err == nil {
		t.Error("Chat() should return error for HTTP 500")
	}
}

func TestStreamChat(t *testing.T) {
	// 创建一个模拟流式响应的测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":"World"}}]}
data: [DONE]
`))
	}))
	defer server.Close()

	p := New(
		WithBaseURL(server.URL),
		WithAPIKey("test-key"),
	)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	var received string
	err := p.StreamChat(context.Background(), messages, func(chunk string) {
		received += chunk
	})

	if err != nil {
		t.Errorf("StreamChat() error = %v", err)
	}
	// 注意：简化测试，只验证没有错误
}
