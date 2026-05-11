package prompt

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jiujuan/wukong/pkg/llm"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type Engine struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

type Template struct {
	Key         string
	Description string
	Version     string
	Messages    []MessageTemplate
}

type MessageTemplate struct {
	Role    string
	Content string
}

type RenderInput struct {
	Variables map[string]any
	Context   any
}

const (
	TemplateWorkerActionDefault = "worker.action.default"
	TemplateWorkerActionSearch  = "worker.action.web_search"
	TemplateWorkerActionReport  = "worker.action.report_gen"
	TemplateWorkerReactDefault  = "worker.react.default"
	TemplatePlannerTaskDefault  = "planner.task.default"
	TemplateChatSessionDefault  = "chat.session.default"
)

type MissingVariablesError struct {
	TemplateKey string
	Keys        []string
}

func (e *MissingVariablesError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("render prompt %q missing variables: %s", e.TemplateKey, strings.Join(e.Keys, ", "))
}

func New() *Engine {
	return &Engine{
		templates: make(map[string]*Template),
	}
}

func NewDefaultEngine() *Engine {
	e := New()
	RegisterBuiltins(e)
	return e
}

func (e *Engine) Register(t *Template) error {
	if e == nil {
		return fmt.Errorf("prompt engine is nil")
	}
	if t == nil {
		return fmt.Errorf("template is nil")
	}
	key := strings.TrimSpace(t.Key)
	if key == "" {
		return fmt.Errorf("template key is empty")
	}
	if len(t.Messages) == 0 {
		return fmt.Errorf("template %q has no messages", key)
	}
	for i, msg := range t.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return fmt.Errorf("template %q message[%d] role is empty", key, i)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.templates[key]; exists {
		return fmt.Errorf("template %q already registered", key)
	}
	e.templates[key] = cloneTemplate(t)
	return nil
}

func (e *Engine) MustRegister(t *Template) {
	if err := e.Register(t); err != nil {
		panic(err)
	}
}

func (e *Engine) Get(key string) (*Template, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.templates[strings.TrimSpace(key)]
	if !ok {
		return nil, false
	}
	return cloneTemplate(t), true
}

func (e *Engine) Render(key string, input RenderInput) ([]llm.Message, error) {
	t, ok := e.Get(key)
	if !ok {
		return nil, fmt.Errorf("template %q not found", strings.TrimSpace(key))
	}
	vars := buildRenderVars(input)
	out := make([]llm.Message, 0, len(t.Messages))
	missingSet := make(map[string]struct{})
	for _, item := range t.Messages {
		content, missing := renderText(item.Content, vars)
		for _, name := range missing {
			missingSet[name] = struct{}{}
		}
		out = append(out, llm.Message{
			Role:    strings.TrimSpace(item.Role),
			Content: content,
		})
	}
	if len(missingSet) > 0 {
		keys := make([]string, 0, len(missingSet))
		for name := range missingSet {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		return nil, &MissingVariablesError{
			TemplateKey: t.Key,
			Keys:        keys,
		}
	}
	return out, nil
}

func cloneTemplate(src *Template) *Template {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Messages != nil {
		dst.Messages = append([]MessageTemplate(nil), src.Messages...)
	}
	return &dst
}

func buildRenderVars(input RenderInput) map[string]string {
	vars := make(map[string]string)
	for k, v := range input.Variables {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		vars[key] = stringify(v)
	}
	addContextVars(vars, input.Context)
	return vars
}

func addContextVars(vars map[string]string, contextValue any) {
	if contextValue == nil {
		return
	}
	switch v := contextValue.(type) {
	case string:
		vars["context"] = v
		vars["context_text"] = v
	case map[string]string:
		for key, value := range v {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			vars["context."+key] = value
		}
	case map[string]any:
		for key, value := range v {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			vars["context."+key] = stringify(value)
		}
	default:
		rv := reflect.ValueOf(contextValue)
		if rv.Kind() == reflect.Map {
			iter := rv.MapRange()
			for iter.Next() {
				key := strings.TrimSpace(fmt.Sprint(iter.Key().Interface()))
				if key == "" {
					continue
				}
				vars["context."+key] = stringify(iter.Value().Interface())
			}
			return
		}
		vars["context"] = stringify(contextValue)
		vars["context_text"] = stringify(contextValue)
	}
}

func renderText(raw string, vars map[string]string) (string, []string) {
	matches := placeholderPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return raw, nil
	}
	missingSet := make(map[string]struct{})
	rendered := placeholderPattern.ReplaceAllStringFunc(raw, func(match string) string {
		sub := placeholderPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := strings.TrimSpace(sub[1])
		value, ok := vars[key]
		if !ok {
			missingSet[key] = struct{}{}
			return match
		}
		return value
	})
	if len(missingSet) == 0 {
		return rendered, nil
	}
	missing := make([]string, 0, len(missingSet))
	for key := range missingSet {
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return rendered, missing
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprint(v)
	}
}

func RegisterBuiltins(e *Engine) {
	if e == nil {
		return
	}
	for _, t := range builtinTemplates() {
		if _, exists := e.Get(t.Key); exists {
			continue
		}
		e.MustRegister(t)
	}
}

func builtinTemplates() []*Template {
	return []*Template{
		{
			Key:         TemplateWorkerActionDefault,
			Description: "default worker action prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是可靠的多智能体任务执行引擎。"},
				{Role: "user", Content: "你是任务执行 Worker，请严格执行子任务并输出可直接使用的结果。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\nAction: {{action}}\nParams(JSON): {{params_json}}\n要求:\n1. 结果要与 Action 对应\n2. 输出使用中文\n3. 内容尽量结构化\n4. 不要解释系统实现细节"},
			},
		},
		{
			Key:         TemplateWorkerActionSearch,
			Description: "web search worker action prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是网络检索执行引擎，优先提取高可信信息并给出结构化结论。"},
				{Role: "user", Content: "请以 web_search 执行器模式处理该任务。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\n查询: {{query}}\n参数: {{params_json}}\n输出要求:\n1. 返回 3-5 条关键信息\n2. 每条包含标题、要点、可信度评估\n3. 最后给出综合结论"},
			},
		},
		{
			Key:         TemplateWorkerActionReport,
			Description: "report generation worker action prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是报告生成执行引擎，按结构输出可直接交付的报告内容。"},
				{Role: "user", Content: "请以 report_gen 执行器模式处理该任务。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\n报告主题: {{topic}}\n参数: {{params_json}}\n输出要求:\n1. 包含摘要、背景、分析、建议、结论\n2. 结构化分节输出\n3. 适合直接交付"},
			},
		},
		{
			Key:         TemplateWorkerReactDefault,
			Description: "react executor prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是 ReAct 执行引擎。必须只输出 JSON，不要输出其他文本。JSON 格式: {\"thought\":\"...\",\"action\":\"tool|final\",\"tool_name\":\"...\",\"tool_params\":{},\"final_answer\":\"...\"}。当 action=tool 时必须给出 tool_name 和 tool_params；当 action=final 时给出 final_answer。当前 skill={{skill_name}}，允许工具白名单={{allowed_tools_json}}"},
				{Role: "user", Content: "sub_task_id={{sub_task_id}}\ntask_id={{task_id}}\naction={{action}}\nparams={{params_json}}\ntool_hint={{tool_name_hint}}"},
			},
		},
		{
			Key:         TemplatePlannerTaskDefault,
			Description: "llm planner prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是任务规划器。把用户任务拆解为可执行 DAG 子任务。只输出 JSON，不要输出任何解释。\nJSON 格式: {\"thought\":\"一句整体规划思路\",\"steps\":[{\"id\":\"s1\",\"action\":\"web_search\",\"params\":{},\"depends_on\":[],\"thought\":\"该步骤思路\"}]}\n要求:\n1. action 必须是简短可执行动作名\n2. depends_on 引用步骤 id\n3. steps 至少 1 个，最多 8 个\n4. 保证 DAG 无环"},
				{Role: "user", Content: "task_id={{task_id}}\nskill={{skill_name}}\nparams={{params_json}}"},
			},
		},
		{
			Key:         TemplateChatSessionDefault,
			Description: "chat session prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是 Wukong 的对话助手。请结合会话记忆和历史对话回答。若记忆与历史冲突，以更近的消息为准。"},
				{Role: "system", Content: "{{memory_text}}"},
				{Role: "user", Content: "{{current_user_message}}"},
			},
		},
	}
}
