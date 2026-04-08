package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
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
	provider      *llm.Provider
	toolManager   *tool.Manager
	skillRegistry *skills.Registry
	logger        *slog.Logger
	maxIterations int
}

func NewReActExecutor(provider *llm.Provider, toolManager *tool.Manager, skillRegistry *skills.Registry, logger *slog.Logger) *ReActExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReActExecutor{
		provider:      provider,
		toolManager:   toolManager,
		skillRegistry: skillRegistry,
		logger:        logger,
		maxIterations: 6,
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

	systemPrompt := buildReActSystemPrompt(skillName, allowedTools)
	paramsJSON, _ := json.Marshal(params)
	userPrompt := fmt.Sprintf("sub_task_id=%s\ntask_id=%s\naction=%s\nparams=%s\ntool_hint=%s",
		subTask.GetSubTaskID(), subTask.GetTaskID(), subTask.GetAction(), string(paramsJSON), toolNameHint)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
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
	toolsJSON, _ := json.Marshal(allowedTools)
	return "你是ReAct执行引擎。必须只输出JSON，不要输出其他文本。JSON格式为: " +
		`{"thought":"...","action":"tool|final","tool_name":"...","tool_params":{},"final_answer":"..."}` +
		"。当 action=tool 时必须给出 tool_name 和 tool_params；当 action=final 时给出 final_answer。当前 skill=" +
		skillName + "，允许工具白名单=" + string(toolsJSON)
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
