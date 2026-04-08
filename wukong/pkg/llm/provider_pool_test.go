package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockAdapter struct {
	name      ProviderType
	failFirst int
	chatCalls int
}

func (m *mockAdapter) Name() ProviderType {
	return m.name
}

func (m *mockAdapter) Chat(ctx context.Context, p *Provider, messages []Message) (*ChatResponse, error) {
	m.chatCalls++
	if m.chatCalls <= m.failFirst {
		return nil, errors.New("mock chat failed")
	}
	return &ChatResponse{
		Model: string(m.name),
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "ok"},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (m *mockAdapter) StreamChat(ctx context.Context, p *Provider, messages []Message, handler func(chunk string)) error {
	if handler != nil {
		handler("ok")
	}
	return nil
}

func TestProviderPoolFallback(t *testing.T) {
	primaryAdapter := &mockAdapter{name: ProviderTypeDeepSeek, failFirst: 100}
	backupAdapter := &mockAdapter{name: ProviderTypeOllama, failFirst: 0}
	primary := &Provider{providerType: ProviderTypeDeepSeek, adapter: primaryAdapter}
	backup := &Provider{providerType: ProviderTypeOllama, adapter: backupAdapter}

	pool := NewProviderPool([]PoolMember{
		{Name: "primary", Priority: 1, Provider: primary},
		{Name: "backup", Priority: 2, Provider: backup},
	}, WithPoolMaxRetries(0), WithPoolFailureThreshold(1), WithPoolCooldown(time.Minute))
	primary.SetProviderPool(pool)

	resp, err := primary.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("chat should fallback success, got err: %v", err)
	}
	if resp == nil || resp.Model != string(ProviderTypeOllama) {
		t.Fatalf("unexpected fallback model: %+v", resp)
	}
	if primaryAdapter.chatCalls != 1 {
		t.Fatalf("primary should be called once, got %d", primaryAdapter.chatCalls)
	}
	if backupAdapter.chatCalls != 1 {
		t.Fatalf("backup should be called once, got %d", backupAdapter.chatCalls)
	}
}

func TestProviderPoolCircuitBreaker(t *testing.T) {
	primaryAdapter := &mockAdapter{name: ProviderTypeDeepSeek, failFirst: 100}
	backupAdapter := &mockAdapter{name: ProviderTypeOllama, failFirst: 0}
	primary := &Provider{providerType: ProviderTypeDeepSeek, adapter: primaryAdapter}
	backup := &Provider{providerType: ProviderTypeOllama, adapter: backupAdapter}

	pool := NewProviderPool([]PoolMember{
		{Name: "primary", Priority: 1, Provider: primary},
		{Name: "backup", Priority: 2, Provider: backup},
	}, WithPoolMaxRetries(0), WithPoolFailureThreshold(1), WithPoolCooldown(time.Minute))
	primary.SetProviderPool(pool)

	_, _ = primary.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}})
	_, _ = primary.Chat(context.Background(), []Message{{Role: "user", Content: "hello again"}})

	if primaryAdapter.chatCalls != 1 {
		t.Fatalf("primary should be short-circuited after first failure, got calls=%d", primaryAdapter.chatCalls)
	}
	if backupAdapter.chatCalls != 2 {
		t.Fatalf("backup should handle both calls, got calls=%d", backupAdapter.chatCalls)
	}
}

func TestProviderPoolRetry(t *testing.T) {
	primaryAdapter := &mockAdapter{name: ProviderTypeDeepSeek, failFirst: 1}
	primary := &Provider{providerType: ProviderTypeDeepSeek, adapter: primaryAdapter}
	pool := NewProviderPool([]PoolMember{
		{Name: "primary", Priority: 1, Provider: primary},
	}, WithPoolMaxRetries(1), WithPoolFailureThreshold(3), WithPoolRetryBackoff(1*time.Millisecond))
	primary.SetProviderPool(pool)

	resp, err := primary.Chat(context.Background(), []Message{{Role: "user", Content: "retry"}})
	if err != nil {
		t.Fatalf("retry should recover, got err: %v", err)
	}
	if resp == nil {
		t.Fatalf("response should not be nil")
	}
	if primaryAdapter.chatCalls != 2 {
		t.Fatalf("primary should be called twice with one retry, got %d", primaryAdapter.chatCalls)
	}
}
