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

type openAPIProvider struct{}

func (p *openAPIProvider) Name() ProviderType { return ProviderTypeOpenAPI }

func (p *openAPIProvider) Chat(ctx context.Context, provider *Provider, messages []Message) (*ChatResponse, error) {
	return doOpenAPIChat(ctx, provider, messages)
}

func (p *openAPIProvider) StreamChat(ctx context.Context, provider *Provider, messages []Message, handler func(chunk string)) error {
	return doOpenAPIStreamChat(ctx, provider, messages, handler)
}

func doOpenAPIChat(ctx context.Context, provider *Provider, messages []Message) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    provider.model,
		Messages: messages,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}
	url := strings.TrimRight(provider.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(provider.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
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

	var result ChatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}
	return &result, nil
}

func doOpenAPIStreamChat(ctx context.Context, provider *Provider, messages []Message, handler func(chunk string)) error {
	req := ChatRequest{
		Model:    provider.model,
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}
	url := strings.TrimRight(provider.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(provider.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
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
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "[DONE]" {
			break
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		choices, ok := data["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}
		chunk, ok := delta["content"].(string)
		if ok && chunk != "" && handler != nil {
			handler(chunk)
		}
	}
	return nil
}
