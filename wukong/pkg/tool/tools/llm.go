package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jiujuan/wukong/pkg/llm"
	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type LLMTool struct {
	provider *llm.Provider
	logger   *pkglogger.Logger
}

func NewLLMTool(provider *llm.Provider, logger *pkglogger.Logger) *LLMTool {
	return &LLMTool{provider: provider, logger: logger}
}

func (t *LLMTool) Name() string { return "llm_chat" }

func (t *LLMTool) Description() string { return "调用 LLM 进行对话推理" }

func (t *LLMTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("system", "string", false, "system prompt", nil, "You are a helpful assistant"),
		schemaItem("messages", "array<object>", false, "prebuilt message list", nil, []map[string]any{
			{"role": "user", "content": "hello"},
		}),
		schemaItem("prompt", "string", false, "fallback prompt content", nil, "hello"),
	}
}

func (t *LLMTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	if t.provider == nil {
		t.logger.Error("[Tool] llm_chat failed: provider is nil")
		return nil, fmt.Errorf("llm provider is nil")
	}
	t.logger.Info("[Tool] llm_chat start", "params_keys", mapKeys(params))
	messages := make([]llm.Message, 0, 4)
	if system := readString(params, "system"); strings.TrimSpace(system) != "" {
		messages = append(messages, llm.Message{Role: "system", Content: system})
	}
	if rawMessages, ok := params["messages"].([]map[string]any); ok && len(rawMessages) > 0 {
		for _, item := range rawMessages {
			role := readString(item, "role")
			content := readString(item, "content")
			if strings.TrimSpace(role) == "" || strings.TrimSpace(content) == "" {
				continue
			}
			messages = append(messages, llm.Message{Role: role, Content: content})
		}
	}
	if len(messages) == 0 {
		prompt := readString(params, "prompt", "query", "input")
		if strings.TrimSpace(prompt) == "" {
			t.logger.Warn("[Tool] llm_chat invalid params: prompt is empty")
			return nil, fmt.Errorf("prompt is required")
		}
		messages = append(messages, llm.Message{Role: "user", Content: prompt})
	}
	t.logger.Debug("[Tool] llm_chat request prepared", "message_count", len(messages))
	resp, err := t.provider.Chat(ctx, messages)
	if err != nil {
		t.logger.Error("[Tool] llm_chat provider call failed", "error", err)
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.logger.Error("[Tool] llm_chat empty response")
		return nil, fmt.Errorf("llm response is empty")
	}
	result := map[string]any{
		"output":            strings.TrimSpace(resp.Choices[0].Message.Content),
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
	}
	t.logger.Info("[Tool] llm_chat success",
		"model", resp.Model,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens,
	)
	return result, nil
}
