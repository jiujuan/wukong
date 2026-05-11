package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/tool"
)

// executableSubTask 定义子任务进入执行层后的最小协议。
//
// 这层故意不直接依赖 manager.SubTask 具体类型，而只依赖几个读写方法，
// 这样 worker 目录可以把“编排数据模型”和“执行运行时”弱耦合起来：
// 只要一个对象能提供 action / params / result 这些最小字段，就能被执行。
type executableSubTask interface {
	GetSubTaskID() string
	GetTaskID() string
	GetAction() string
	GetParams() map[string]any
	SetResult(map[string]any)
	SetError(string)
	SetUpdatedAt(time.Time)
}

// PromptBuilder 负责把子任务转换成 LLM 可消费的消息列表。
//
// 它是“执行逻辑”和“提示词构造”之间的边界：
// - 执行器只关心拿到 messages 后去调 LLM
// - prompt 怎么选模板、怎么拼变量，由 PromptBuilder 处理
type PromptBuilder interface {
	BuildMessages(ctx context.Context, subTask executableSubTask) ([]llm.Message, error)
}

// ActionPromptBuilder 是默认的子任务提示词构造器。
//
// 第一层按 action 选择模板 key：
// - web_search -> 检索模板
// - report_gen -> 报告模板
// - 其他 action -> 默认执行模板
//
// 第二层再把 subtask 基础字段和 params 派生字段灌入 PromptEngine。
type ActionPromptBuilder struct {
	engine      *prompt.Engine
	templateKey map[string]string
}

func NewActionPromptBuilder() *ActionPromptBuilder {
	return &ActionPromptBuilder{
		engine: prompt.NewDefaultEngine(),
		templateKey: map[string]string{
			"web_search": prompt.TemplateWorkerActionSearch,
			"report_gen": prompt.TemplateWorkerActionReport,
		},
	}
}

func (b *ActionPromptBuilder) BuildMessages(_ context.Context, subTask executableSubTask) ([]llm.Message, error) {
	if b == nil || b.engine == nil {
		return nil, fmt.Errorf("prompt engine is nil")
	}

	// 统一把 params 先序列化成 JSON 字符串，便于模板直接使用。
	// 如果 params 中有复杂对象，也至少能给模型一个稳定的文本表示。
	paramsJSON, err := json.Marshal(subTask.GetParams())
	if err != nil {
		paramsJSON = []byte("{}")
	}

	// action 模板选择是“特例覆盖默认”的结构：
	// 低风险 action 直接走专用模板，其余 action 仍可复用统一执行模板。
	action := strings.ToLower(strings.TrimSpace(subTask.GetAction()))
	templateKey := prompt.TemplateWorkerActionDefault
	if custom, ok := b.templateKey[action]; ok {
		templateKey = custom
	}

	// query / topic 是为了兼容不同 action 的常见参数命名。
	// 例如 search 可能叫 query/q/topic，report 可能叫 topic/title/subject。
	// 找不到时回退到 params JSON 或默认主题，避免模板渲染缺参。
	query := extractStringParam(subTask.GetParams(), "query", "keyword", "q", "topic")
	if strings.TrimSpace(query) == "" {
		query = string(paramsJSON)
	}
	topic := extractStringParam(subTask.GetParams(), "topic", "title", "subject", "query")
	if strings.TrimSpace(topic) == "" {
		topic = "未指定主题"
	}
	return b.engine.Render(templateKey, prompt.RenderInput{
		Variables: map[string]any{
			"sub_task_id": subTask.GetSubTaskID(),
			"task_id":     subTask.GetTaskID(),
			"action":      subTask.GetAction(),
			"params_json": string(paramsJSON),
			"query":       query,
			"topic":       topic,
		},
	})
}

// ActionExecutor 表示真正执行某类 action 的策略对象。
//
// 当前实现里至少有三类：
// - 纯 LLM 执行
// - 工具执行
// - ReAct / 组合执行
//
// SubTaskExecutor 只做路由，不关心底层到底如何完成 action。
type ActionExecutor interface {
	Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error)
}

// LLMActionExecutor 是最基础的 action 执行器：
// 1. 用 PromptBuilder 构造消息
// 2. 调用 Provider.Chat
// 3. 把 LLM 输出包装成统一结果
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

func (e *LLMActionExecutor) Execute(ctx context.Context, subTask executableSubTask) (map[string]any, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("llm provider is nil")
	}
	messages, err := e.promptBuilder.BuildMessages(ctx, subTask)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}
	// 执行层返回的不只是 output，还会带上 model 和 usage，
	// 这样上层聚合或调试时能看到一次子任务实际消耗了什么。
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

// NewSubTaskExecutor 创建“纯本地路由”的子任务执行器。
//
// 路由策略：
// - 如果 action 有显式注册执行器，则用专用执行器
// - 否则走 defaultExecutor
func NewSubTaskExecutor(provider *llm.Provider, logger *slog.Logger, promptBuilder PromptBuilder) *SubTaskExecutor {
	defaultExecutor := NewLLMActionExecutor(provider, promptBuilder)
	actionExecutors := defaultActionExecutors(provider, nil)
	return &SubTaskExecutor{
		logger:          logger,
		defaultExecutor: defaultExecutor,
		actionExecutors: actionExecutors,
	}
}

// NewSubTaskExecutorWithTools 在默认路由基础上，把 ReAct 和工具能力接进来。
//
// 这里让 web_search / report_gen 默认走 reactExecutor，
// 是因为这两类 action 天然受益于“先思考，再决定是否调工具，再输出”的链路。
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

// Handle 是 Pool.taskHandler 最常落到的业务入口。
//
// 它做三件事：
// 1. 从 queue.Task 中取出 executableSubTask
// 2. 根据 action 路由到具体执行器
// 3. 把结果写回 subTask，供 Manager 回调时读取
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

	// 结果写回 subtask 是这里非常关键的一步：
	// WorkerPool 本身不理解业务结果，只会在 executeTask 结束后通过 extractResult
	// 从 task.Data 里把结果拿出来，再回调给 Manager。
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

// RegisterActionExecutor 允许在默认路由表之外追加或覆盖某个 action 的执行策略。
// 这使得 worker 层可以在不改 Handle 主流程的情况下逐步扩展新的 action。
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

// NewRoutedSubTaskHandler / NewRoutedSubTaskHandlerWithTools / NewLLMSubTaskHandler
// 是给 Pool 注入 taskHandler 的便捷工厂。
//
// 调用方通常不直接手写 TaskHandler，而是通过这些 helper 把：
// - provider
// - logger
// - toolManager
// - skillRegistry
// 组装成一个完整的“子任务执行入口”。
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

// extractStringParam 是执行层的一个小兼容工具。
//
// 原因是不同 planner / tool / caller 传入的 params 命名不总是稳定，
// 这里通过一组候选 key 兜底，减少上层参数命名抖动对执行链路的影响。
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

// defaultActionExecutors 返回默认 action -> executor 路由表。
//
// 设计上这里用了“按能力增强”的两层逻辑：
// - 没有 toolManager：web_search / report_gen 走 LLM 执行
// - 有 toolManager：优先走工具，必要时回退到 LLM
//
// 这样同一批 action 可以在不同运行环境下自然获得不同执行能力。
func defaultActionExecutors(provider *llm.Provider, toolManager *tool.Manager) map[string]ActionExecutor {
	buildPrompt := NewActionPromptBuilder()
	executors := map[string]ActionExecutor{
		"report_gen": NewLLMActionExecutor(provider, buildPrompt),
	}
	if toolManager != nil {
		executors["web_search"] = NewToolActionExecutor(toolManager, "web_search", "web_search")
		executors["report_gen"] = NewCompositeActionExecutor(
			NewToolActionExecutor(toolManager, "report_gen", "llm_chat"),
			NewLLMActionExecutor(provider, buildPrompt),
		)
		return executors
	}
	executors["web_search"] = NewLLMActionExecutor(provider, buildPrompt)
	return executors
}

// ToolActionExecutor 代表“直接调用某个工具完成 action”。
//
// 它适合 action 和 tool 几乎一一对应的场景，例如：
// - web_search -> web_search tool
// - report_gen -> llm_chat tool
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

	// search 类 action 在很多地方只传 prompt/topic/title，不一定显式给 query。
	// 这里执行前补齐 query，避免工具层因为缺少标准参数而失败。
	params := subTask.GetParams()
	if params == nil {
		params = map[string]any{}
	}
	if _, ok := params["query"]; !ok && strings.EqualFold(e.toolName, "web_search") {
		promptValue := extractStringParam(params, "prompt", "topic", "title")
		if strings.TrimSpace(promptValue) != "" {
			params["query"] = promptValue
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

// CompositeActionExecutor 表示“先试 primary，失败后走 fallback”。
//
// 目前主要用于：
// - 优先调用工具
// - 工具失败时退回到 LLM
//
// 这样比单一路径更稳，也更方便渐进增强执行能力。
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

// SkillAwareActionExecutor 是更高一层的策略执行器：
// 它会优先根据 skill/tool 信息选择更具体的执行路径，
// 如果都不满足，再回退到通用执行器。
//
// 优先级顺序：
// 1. toolManager 中存在显式 tool
// 2. skillRegistry 中存在可执行 skill
// 3. fallback
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

// resolveSkillName / resolveToolName 是执行路由层的关键归一化函数。
//
// 它们的目标不是“绝对正确地理解业务语义”，而是给执行器一个稳定的默认路由依据，
// 让不同来源的 action / params 能收敛到较少的 skill/tool 名称上。
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

// cloneParams 避免执行器在补参数、改参数时直接污染原始 subtask params。
// 这在工具执行和 ReAct 多轮执行里尤其重要。
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
