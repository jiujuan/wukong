package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/tool"
)

// executableSubTask 可执行的子任务, 定义了子任务的基本属性和操作
type executableSubTask interface {
	GetSubTaskID() string
	GetTaskID() string
	GetAction() string
	GetParams() map[string]any
	SetResult(map[string]any)
	SetError(string)
	SetUpdatedAt(time.Time)
}

// PromptBuilder 构建子任务的Prompt消息, 定义了根据子任务的Action执行类型动态选择系统提示模板的方法
// 实现了PromptBuilder接口的类型, 可以根据子任务的Action执行类型动态选择系统提示模板
// 并构建出符合要求的Prompt消息
type PromptBuilder interface {
	BuildMessages(ctx context.Context, subTask executableSubTask) ([]llm.Message, error)
}

type ActionPromptBuilder struct {
	actionSystem  map[string]string
	defaultSystem string
}

// NewActionPromptBuilder 创建一个新的ActionPromptBuilder实例, Prompt 构建策略, 根据Action执行类型动态选择系统提示模板
func NewActionPromptBuilder() *ActionPromptBuilder {
	return &ActionPromptBuilder{
		actionSystem: map[string]string{
			"web_search": "你是网络检索执行引擎，优先提取高可信信息并给出结构化结论。",
			"report_gen": "你是报告生成执行引擎，按结构输出可直接交付的报告内容。",
		},
		defaultSystem: "你是可靠的多智能体任务执行引擎。",
	}
}

// BuildMessages 构建子任务的Prompt消息, 返回一个包含系统提示和用户提示的消息列表
func (b *ActionPromptBuilder) BuildMessages(_ context.Context, subTask executableSubTask) ([]llm.Message, error) {
	paramsJSON, err := json.Marshal(subTask.GetParams())
	if err != nil {
		paramsJSON = []byte("{}")
	}
	systemPrompt := b.defaultSystem
	action := subTask.GetAction()
	if custom, ok := b.actionSystem[action]; ok {
		systemPrompt = custom
	}

	prompt := fmt.Sprintf(
		"你是任务执行Worker，请严格执行子任务并输出可直接使用的结果。\n子任务ID: %s\n主任务ID: %s\nAction: %s\nParams(JSON): %s\n要求：\n1. 结果要与Action对应\n2. 输出使用中文\n3. 内容尽量结构化\n4. 不要解释系统实现细节",
		subTask.GetSubTaskID(),
		subTask.GetTaskID(),
		subTask.GetAction(),
		string(paramsJSON),
	)

	return []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}, nil
}

type WebSearchPromptBuilder struct{}

func (b *WebSearchPromptBuilder) BuildMessages(_ context.Context, subTask executableSubTask) ([]llm.Message, error) {
	paramsJSON, err := json.Marshal(subTask.GetParams())
	if err != nil {
		paramsJSON = []byte("{}")
	}
	query := extractStringParam(subTask.GetParams(), "query", "keyword", "q", "topic")
	if strings.TrimSpace(query) == "" {
		query = string(paramsJSON)
	}
	userPrompt := fmt.Sprintf(
		"请以web_search执行器模式处理该任务。\n子任务ID: %s\n主任务ID: %s\n查询: %s\n参数: %s\n输出要求：\n1. 返回3-5条关键信息\n2. 每条包含标题、要点、可信度评估\n3. 最后给出综合结论",
		subTask.GetSubTaskID(),
		subTask.GetTaskID(),
		query,
		string(paramsJSON),
	)
	return []llm.Message{
		{Role: "system", Content: "你是网络检索执行引擎，优先提取高可信信息并给出结构化结论。"},
		{Role: "user", Content: userPrompt},
	}, nil
}

type ReportGenPromptBuilder struct{}

func (b *ReportGenPromptBuilder) BuildMessages(_ context.Context, subTask executableSubTask) ([]llm.Message, error) {
	paramsJSON, err := json.Marshal(subTask.GetParams())
	if err != nil {
		paramsJSON = []byte("{}")
	}
	topic := extractStringParam(subTask.GetParams(), "topic", "title", "subject", "query")
	if strings.TrimSpace(topic) == "" {
		topic = "未指定主题"
	}
	userPrompt := fmt.Sprintf(
		"请以report_gen执行器模式处理该任务。\n子任务ID: %s\n主任务ID: %s\n报告主题: %s\n参数: %s\n输出要求：\n1. 包含摘要、背景、分析、建议、结论\n2. 结构化分节输出\n3. 适合直接交付",
		subTask.GetSubTaskID(),
		subTask.GetTaskID(),
		topic,
		string(paramsJSON),
	)
	return []llm.Message{
		{Role: "system", Content: "你是报告生成执行引擎，按结构输出可直接交付的报告内容。"},
		{Role: "user", Content: userPrompt},
	}, nil
}

// ActionExecutor 执行子任务的接口, 定义了执行子任务的方法
type ActionExecutor interface {
	Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error)
}

// LLMActionExecutor 基于LLM的子任务执行器, 实现了ActionExecutor接口
type LLMActionExecutor struct {
	provider      *llm.Provider
	promptBuilder PromptBuilder
}

func NewLLMActionExecutor(provider *llm.Provider, promptBuilder PromptBuilder) *LLMActionExecutor {
	if promptBuilder == nil {
		promptBuilder = NewActionPromptBuilder()
	}
	return &LLMActionExecutor{
		provider:      provider,
		promptBuilder: promptBuilder,
	}
}

// Execute 执行子任务, 返回执行结果
// 根据子任务的Action执行类型动态选择系统提示模板
// 并构建出符合要求的Prompt消息
func (e *LLMActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("llm provider is nil")
	}
	messages, err := e.promptBuilder.BuildMessages(ctx, subTask)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}
	resp, err := e.provider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm execute subtask failed: %w", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	return map[string]any{
		"output":            content,
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
		"total_tokens":      resp.Usage.TotalTokens,
		"completed_at":      time.Now().Format(time.RFC3339),
	}, nil
}

type SubTaskExecutor struct {
	logger          *slog.Logger
	defaultExecutor ActionExecutor
	actionExecutors map[string]ActionExecutor
}

func NewSubTaskExecutor(provider *llm.Provider, logger *slog.Logger, promptBuilder PromptBuilder) *SubTaskExecutor {
	defaultExecutor := NewLLMActionExecutor(provider, promptBuilder)
	actionExecutors := defaultActionExecutors(provider, nil)
	return &SubTaskExecutor{
		logger:          logger,
		defaultExecutor: defaultExecutor,
		actionExecutors: actionExecutors,
	}
}

func NewSubTaskExecutorWithTools(provider *llm.Provider, logger *slog.Logger, promptBuilder PromptBuilder, toolManager *tool.Manager, skillRegistry *skills.Registry) *SubTaskExecutor {
	reactExecutor := NewReActExecutor(provider, toolManager, skillRegistry, logger)
	defaultExecutor := reactExecutor
	actionExecutors := defaultActionExecutors(provider, toolManager)
	actionExecutors["web_search"] = reactExecutor
	actionExecutors["report_gen"] = reactExecutor
	return &SubTaskExecutor{
		logger:          logger,
		defaultExecutor: defaultExecutor,
		actionExecutors: actionExecutors,
	}
}

// Handle 处理子任务
// 根据Action执行类型动态选择系统提示模板
func (e *SubTaskExecutor) Handle(ctx context.Context, task *queue.Task) error {
	subTask, ok := task.Data.(executableSubTask)
	if !ok || subTask == nil {
		return fmt.Errorf("invalid subtask payload for task_id=%s", task.TaskID)
	}
	action := strings.ToLower(strings.TrimSpace(subTask.GetAction()))
	executor := e.defaultExecutor
	if routed, ok := e.actionExecutors[action]; ok {
		executor = routed
	}
	if executor == nil {
		return fmt.Errorf("no action executor for action=%s", action)
	}
	result, err := executor.Execute(ctx, subTask)
	if err != nil {
		return err
	}
	if result == nil {
		result = map[string]any{}
	}
	result["sub_task_id"] = subTask.GetSubTaskID()
	result["action"] = subTask.GetAction()
	subTask.SetResult(result)
	subTask.SetError("")
	subTask.SetUpdatedAt(time.Now())
	if e.logger != nil {
		e.logger.Info("subtask executed by router",
			"task_id", task.TaskID,
			"sub_task_id", subTask.GetSubTaskID(),
			"action", subTask.GetAction())
	}
	return nil
}

func (e *SubTaskExecutor) RegisterActionExecutor(action string, executor ActionExecutor) {
	key := strings.ToLower(strings.TrimSpace(action))
	if key == "" || executor == nil {
		return
	}
	if e.actionExecutors == nil {
		e.actionExecutors = make(map[string]ActionExecutor)
	}
	e.actionExecutors[key] = executor
}

func NewRoutedSubTaskHandler(provider *llm.Provider, logger *slog.Logger) TaskHandler {
	executor := NewSubTaskExecutor(provider, logger, NewActionPromptBuilder())
	return executor.Handle
}

func NewRoutedSubTaskHandlerWithTools(provider *llm.Provider, logger *slog.Logger, toolManager *tool.Manager, skillRegistry *skills.Registry) TaskHandler {
	executor := NewSubTaskExecutorWithTools(provider, logger, NewActionPromptBuilder(), toolManager, skillRegistry)
	return executor.Handle
}

func NewLLMSubTaskHandler(provider *llm.Provider, logger *slog.Logger) TaskHandler {
	return NewRoutedSubTaskHandler(provider, logger)
}

func extractStringParam(params map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := params[k]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		default:
			s := fmt.Sprintf("%v", value)
			if strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func defaultActionExecutors(provider *llm.Provider, toolManager *tool.Manager) map[string]ActionExecutor {
	executors := map[string]ActionExecutor{
		"report_gen": NewLLMActionExecutor(provider, &ReportGenPromptBuilder{}),
	}
	if toolManager != nil {
		executors["web_search"] = NewToolActionExecutor(toolManager, "web_search", "web_search")
		executors["report_gen"] = NewCompositeActionExecutor(
			NewToolActionExecutor(toolManager, "report_gen", "llm_chat"),
			NewLLMActionExecutor(provider, &ReportGenPromptBuilder{}),
		)
		return executors
	}
	executors["web_search"] = NewLLMActionExecutor(provider, &WebSearchPromptBuilder{})
	return executors
}

type ToolActionExecutor struct {
	toolManager *tool.Manager
	skillName   string
	toolName    string
}

func NewToolActionExecutor(toolManager *tool.Manager, skillName, toolName string) *ToolActionExecutor {
	return &ToolActionExecutor{
		toolManager: toolManager,
		skillName:   strings.ToLower(strings.TrimSpace(skillName)),
		toolName:    strings.ToLower(strings.TrimSpace(toolName)),
	}
}

func (e *ToolActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	if e.toolManager == nil {
		return nil, fmt.Errorf("tool manager is nil")
	}
	params := subTask.GetParams()
	if params == nil {
		params = map[string]any{}
	}
	if _, ok := params["query"]; !ok && strings.EqualFold(e.toolName, "web_search") {
		prompt := extractStringParam(params, "prompt", "topic", "title")
		if strings.TrimSpace(prompt) != "" {
			params["query"] = prompt
		}
	}
	result, err := e.toolManager.ExecuteForSkill(ctx, e.skillName, e.toolName, params)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	result["tool"] = e.toolName
	return result, nil
}

type CompositeActionExecutor struct {
	primary  ActionExecutor
	fallback ActionExecutor
}

func NewCompositeActionExecutor(primary ActionExecutor, fallback ActionExecutor) *CompositeActionExecutor {
	return &CompositeActionExecutor{primary: primary, fallback: fallback}
}

func (e *CompositeActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	if e.primary != nil {
		result, err := e.primary.Execute(ctx, subTask)
		if err == nil {
			return result, nil
		}
	}
	if e.fallback == nil {
		return nil, fmt.Errorf("composite executor has no fallback")
	}
	return e.fallback.Execute(ctx, subTask)
}

type SkillAwareActionExecutor struct {
	toolManager   *tool.Manager
	skillRegistry *skills.Registry
	fallback      ActionExecutor
}

func NewSkillAwareActionExecutor(toolManager *tool.Manager, skillRegistry *skills.Registry, fallback ActionExecutor) *SkillAwareActionExecutor {
	return &SkillAwareActionExecutor{
		toolManager:   toolManager,
		skillRegistry: skillRegistry,
		fallback:      fallback,
	}
}

func (e *SkillAwareActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	params := cloneParams(subTask.GetParams())
	skillName := resolveSkillName(subTask.GetAction(), params)
	toolName := resolveToolName(subTask.GetAction(), params)
	if e.toolManager != nil && skillName != "" && toolName != "" {
		if _, ok := e.toolManager.Get(toolName); ok {
			result, err := e.toolManager.ExecuteForSkill(ctx, skillName, toolName, params)
			if err == nil {
				result["skill_name"] = skillName
				result["tool"] = toolName
				return result, nil
			}
		}
	}
	if e.skillRegistry != nil && skillName != "" {
		if item, ok := e.skillRegistry.Get(skillName); ok && strings.TrimSpace(item.Execute) != "" {
			result, err := e.skillRegistry.ExecuteWithParams(ctx, skillName, params)
			if err == nil {
				result["skill_name"] = skillName
				return result, nil
			}
		}
	}
	if e.fallback == nil {
		return nil, fmt.Errorf("skill aware executor fallback is nil")
	}
	return e.fallback.Execute(ctx, subTask)
}

func resolveSkillName(action string, params map[string]any) string {
	if name := extractStringParam(params, "skill_name", "skill", "skillName"); strings.TrimSpace(name) != "" {
		return strings.ToLower(strings.TrimSpace(name))
	}
	return strings.ToLower(strings.TrimSpace(action))
}

func resolveToolName(action string, params map[string]any) string {
	if name := extractStringParam(params, "tool_name", "tool", "toolName"); strings.TrimSpace(name) != "" {
		return strings.ToLower(strings.TrimSpace(name))
	}
	a := strings.ToLower(strings.TrimSpace(action))
	switch a {
	case "report_gen":
		return "llm_chat"
	case "execute":
		return "llm_chat"
	case "search_execute", "search", "search_query":
		return "web_search"
	default:
		if strings.Contains(a, "search") {
			return "web_search"
		}
		return a
	}
}

func cloneParams(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
