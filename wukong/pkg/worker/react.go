package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/messagebuilder"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/tool"
)

type ReActStep struct {
	Iteration   int            `json:"iteration"`
	Thought     string         `json:"thought,omitempty"`
	Action      string         `json:"action,omitempty"`
	ToolName    string         `json:"tool_name,omitempty"`
	ToolParams  map[string]any `json:"tool_params,omitempty"`
	Observation map[string]any `json:"observation,omitempty"`
	Output      string         `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type reactLLMReply struct {
	Thought     string         `json:"thought"`
	Action      string         `json:"action"`
	ToolName    string         `json:"tool_name"`
	ToolParams  map[string]any `json:"tool_params"`
	FinalAnswer string         `json:"final_answer"`
}

type ReActExecutor struct {
	provider       *llm.Provider
	toolManager    *tool.Manager
	skillRegistry  *skills.Registry
	logger         *slog.Logger
	maxIterations  int
	messageBuilder *messagebuilder.Builder
}

func NewReActExecutor(provider *llm.Provider, toolManager *tool.Manager, skillRegistry *skills.Registry, logger *slog.Logger) *ReActExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReActExecutor{
		provider:       provider,
		toolManager:    toolManager,
		skillRegistry:  skillRegistry,
		logger:         logger,
		maxIterations:  6,
		messageBuilder: newWorkerMessageBuilder(newWorkerContextEngine(skillRegistry), prompt.NewDefaultEngine()),
	}
}

func (e *ReActExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("llm provider is nil")
	}
	params := cloneParams(subTask.GetParams())
	skillName := resolveSkillName(subTask.GetAction(), params)
	toolNameHint := resolveToolName(subTask.GetAction(), params)
	allowedTools := e.allowedTools(skillName)
	if len(allowedTools) == 0 && e.toolManager != nil && toolNameHint != "" {
		if _, ok := e.toolManager.Get(toolNameHint); ok {
			allowedTools = []string{toolNameHint}
		}
	}

	paramsJSON, _ := json.Marshal(params)
	result, err := e.messageBuilder.BuildMessages(ctx, messagebuilder.BuildRequest{
		Scene:       workerSceneName,
		TemplateKey: prompt.TemplateWorkerReactDefault,
		Context:     buildWorkerContextRequest(subTask),
		Variables: map[string]any{
			"skill_name":         skillName,
			"allowed_tools_json": mustJSON(allowedTools),
			"sub_task_id":        subTask.GetSubTaskID(),
			"task_id":            subTask.GetTaskID(),
			"action":             subTask.GetAction(),
			"params_json":        string(paramsJSON),
			"tool_name_hint":     toolNameHint,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("render react prompt failed: %w", err)
	}
	messages := result.Messages
	steps := make([]ReActStep, 0, e.maxIterations)

	for i := 1; i <= e.maxIterations; i++ {
		resp, err := e.provider.Chat(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("react chat failed: %w", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			return nil, fmt.Errorf("react returned empty choices")
		}
		content := strings.TrimSpace(resp.Choices[0].Message.Content)
		messages = append(messages, llm.Message{Role: "assistant", Content: content})

		reply, parseErr := parseReActReply(content)
		if parseErr != nil {
			final := map[string]any{
				"output":            content,
				"skill_name":        skillName,
				"react_steps":       steps,
				"completed_at":      time.Now().Format(time.RFC3339),
				"prompt_tokens":     resp.Usage.PromptTokens,
				"completion_tokens": resp.Usage.CompletionTokens,
				"total_tokens":      resp.Usage.TotalTokens,
			}
			return final, nil
		}

		step := ReActStep{
			Iteration:  i,
			Thought:    strings.TrimSpace(reply.Thought),
			Action:     strings.ToLower(strings.TrimSpace(reply.Action)),
			ToolName:   strings.ToLower(strings.TrimSpace(reply.ToolName)),
			ToolParams: cloneParams(reply.ToolParams),
		}

		if step.Action == "final" || strings.TrimSpace(reply.FinalAnswer) != "" {
			output := strings.TrimSpace(reply.FinalAnswer)
			if output == "" {
				output = content
			}
			step.Output = output
			steps = append(steps, step)
			return map[string]any{
				"output":            output,
				"skill_name":        skillName,
				"react_steps":       steps,
				"completed_at":      time.Now().Format(time.RFC3339),
				"prompt_tokens":     resp.Usage.PromptTokens,
				"completion_tokens": resp.Usage.CompletionTokens,
				"total_tokens":      resp.Usage.TotalTokens,
			}, nil
		}

		if step.Action != "tool" {
			step.Output = content
			steps = append(steps, step)
			return map[string]any{
				"output":       content,
				"skill_name":   skillName,
				"react_steps":  steps,
				"completed_at": time.Now().Format(time.RFC3339),
			}, nil
		}

		callTool := step.ToolName
		if callTool == "" {
			callTool = toolNameHint
			step.ToolName = callTool
		}
		if callTool == "" {
			step.Error = "tool_name empty"
			steps = append(steps, step)
			messages = append(messages, llm.Message{Role: "user", Content: `{"observation":"tool_name empty, please provide tool_name"}`})
			continue
		}

		if step.ToolParams == nil {
			step.ToolParams = map[string]any{}
		}
		if _, ok := step.ToolParams["query"]; !ok && callTool == "web_search" {
			if q := extractStringParam(params, "query", "q", "keyword", "topic", "prompt"); strings.TrimSpace(q) != "" {
				step.ToolParams["query"] = q
			}
		}

		var observation map[string]any
		var toolErr error
		if e.toolManager == nil {
			toolErr = fmt.Errorf("tool manager is nil")
		} else {
			observation, toolErr = e.toolManager.ExecuteForSkill(ctx, skillName, callTool, step.ToolParams)
		}
		if toolErr != nil {
			step.Error = toolErr.Error()
			observation = map[string]any{"error": toolErr.Error()}
		}
		step.Observation = observation
		steps = append(steps, step)

		observationJSON, _ := json.Marshal(observation)
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: fmt.Sprintf("Observation: %s", string(observationJSON)),
		})
	}

	return nil, fmt.Errorf("react exceeded max iterations: %d", e.maxIterations)
}

func (e *ReActExecutor) allowedTools(skillName string) []string {
	if e.skillRegistry == nil {
		return []string{}
	}
	item, ok := e.skillRegistry.Get(skillName)
	if !ok || item == nil {
		return []string{}
	}
	return append([]string(nil), item.Tools...)
}

func buildReActSystemPrompt(skillName string, allowedTools []string) string {
	engine := prompt.NewDefaultEngine()
	msgs, err := engine.Render(prompt.TemplateWorkerReactDefault, prompt.RenderInput{
		Variables: map[string]any{
			"skill_name":         skillName,
			"allowed_tools_json": mustJSON(allowedTools),
			"sub_task_id":        "sub_task_id",
			"task_id":            "task_id",
			"action":             "action",
			"params_json":        "{}",
			"tool_name_hint":     "",
		},
		Context: map[string]any{
			"task_state_text": "task_state",
			"skill_spec_text": "skill_spec",
		},
	})
	if err != nil || len(msgs) == 0 {
		return ""
	}
	return msgs[0].Content
}

func parseReActReply(raw string) (*reactLLMReply, error) {
	content := strings.TrimSpace(raw)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	reply := &reactLLMReply{}
	if err := json.Unmarshal([]byte(content), reply); err != nil {
		return nil, err
	}
	return reply, nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}
