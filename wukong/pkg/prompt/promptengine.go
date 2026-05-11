package prompt

import (
	"fmt"
	"strings"
	"sync"
)

type Engine struct {
	mu        sync.RWMutex
	templates map[string]*Template
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
				{Role: "user", Content: "你是任务执行 Worker，请严格执行子任务并输出可直接使用的结果。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\nAction: {{action}}\nParams(JSON): {{params_json}}\nTaskState:\n{{context.task_state_text}}\nSkillSpec:\n{{context.skill_spec_text}}\n要求:\n1. 结果要与 Action 对应\n2. 输出使用中文\n3. 内容尽量结构化\n4. 不要解释系统实现细节"},
			},
		},
		{
			Key:         TemplateWorkerActionSearch,
			Description: "web search worker action prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是网络检索执行引擎，优先提取高可信信息并给出结构化结论。"},
				{Role: "user", Content: "请以 web_search 执行器模式处理该任务。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\n查询: {{query}}\n参数: {{params_json}}\nTaskState:\n{{context.task_state_text}}\nSkillSpec:\n{{context.skill_spec_text}}\n输出要求:\n1. 返回 3-5 条关键信息\n2. 每条包含标题、要点、可信度评估\n3. 最后给出综合结论"},
			},
		},
		{
			Key:         TemplateWorkerActionReport,
			Description: "report generation worker action prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是报告生成执行引擎，按结构输出可直接交付的报告内容。"},
				{Role: "user", Content: "请以 report_gen 执行器模式处理该任务。\n子任务ID: {{sub_task_id}}\n主任务ID: {{task_id}}\n报告主题: {{topic}}\n参数: {{params_json}}\nTaskState:\n{{context.task_state_text}}\nSkillSpec:\n{{context.skill_spec_text}}\n输出要求:\n1. 包含摘要、背景、分析、建议、结论\n2. 结构化分节输出\n3. 适合直接交付"},
			},
		},
		{
			Key:         TemplateWorkerReactDefault,
			Description: "react executor prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是 ReAct 执行引擎。必须只输出 JSON，不要输出其他文本。JSON 格式: {\"thought\":\"...\",\"action\":\"tool|final\",\"tool_name\":\"...\",\"tool_params\":{},\"final_answer\":\"...\"}。当 action=tool 时必须给出 tool_name 和 tool_params；当 action=final 时给出 final_answer。当前 skill={{skill_name}}，允许工具白名单={{allowed_tools_json}}"},
				{Role: "user", Content: "sub_task_id={{sub_task_id}}\ntask_id={{task_id}}\naction={{action}}\nparams={{params_json}}\ntool_hint={{tool_name_hint}}\ntask_state={{context.task_state_text}}\nskill_spec={{context.skill_spec_text}}"},
			},
		},
		{
			Key:         TemplatePlannerTaskDefault,
			Description: "llm planner prompt",
			Version:     "v1",
			Messages: []MessageTemplate{
				{Role: "system", Content: "你是任务规划器。把用户任务拆解为可执行 DAG 子任务。只输出 JSON，不要输出任何解释。\nJSON 格式: {\"thought\":\"一句整体规划思路\",\"steps\":[{\"id\":\"s1\",\"action\":\"web_search\",\"params\":{},\"depends_on\":[],\"thought\":\"该步骤思路\"}]}\n要求:\n1. action 必须是简短可执行动作名\n2. depends_on 引用步骤 id\n3. steps 至少 1 个，最多 8 个\n4. 保证 DAG 无环"},
				{Role: "user", Content: "task_id={{task_id}}\nskill={{skill_name}}\nparams={{params_json}}\ntask_state={{context.task_state_text}}\nskill_spec={{context.skill_spec_text}}"},
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
