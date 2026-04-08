package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ollamaProvider struct{}

type ollamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ollamaChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

func (p *ollamaProvider) Name() ProviderType { return ProviderTypeOllama }

func (p *ollamaProvider) Chat(ctx context.Context, provider *Provider, messages []Message) (*ChatResponse, error) {
	req := ollamaChatRequest{
		Model:    provider.model,
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}
	url := strings.TrimRight(provider.baseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := provider.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}
	var parsed ollamaChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}
	return &ChatResponse{
		Model: parsed.Model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      parsed.Message,
				FinishReason: "stop",
			},
		},
	}, nil
}

func (p *ollamaProvider) StreamChat(ctx context.Context, provider *Provider, messages []Message, handler func(chunk string)) error {
	req := ollamaChatRequest{
		Model:    provider.model,
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}
	url := strings.TrimRight(provider.baseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := provider.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var parsed ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			continue
		}
		if parsed.Message.Content != "" && handler != nil {
			handler(parsed.Message.Content)
		}
		if parsed.Done {
			break
		}
	}
	return nil
}
