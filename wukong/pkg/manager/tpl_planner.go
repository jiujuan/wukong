package manager

import (
	"context"
	"fmt"

	"github.com/jiujuan/wukong/pkg/statemachine"
)

type TplPlannerOption func(*TplPlanner)

type TplPlanner struct {
	templates map[string]*TaskTemplate
}

func NewTplPlanner(opts ...TplPlannerOption) *TplPlanner {
	p := &TplPlanner{
		templates: make(map[string]*TaskTemplate),
	}
	p.registerDefaultTemplates()
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func NewPlanner(opts ...TplPlannerOption) TaskPlanner {
	return NewTplPlanner(opts...)
}

func WithTemplate(name string, steps []TemplateStep) TplPlannerOption {
	return func(p *TplPlanner) {
		p.templates[name] = &TaskTemplate{
			Name:  name,
			Steps: steps,
		}
	}
}

func (p *TplPlanner) Name() string { return "tpl" }

func (p *TplPlanner) registerDefaultTemplates() {
	p.templates["default"] = &TaskTemplate{
		Name: "default",
		Steps: []TemplateStep{
			{Action: "execute", Params: nil, DependsOn: []int{}},
		},
	}
	p.templates["search"] = &TaskTemplate{
		Name: "search",
		Steps: []TemplateStep{
			{Action: "search_prepare", Params: nil, DependsOn: []int{}},
			{Action: "search_execute", Params: nil, DependsOn: []int{0}},
			{Action: "search_aggregate", Params: nil, DependsOn: []int{1}},
		},
	}
	p.templates["data_process"] = &TaskTemplate{
		Name: "data_process",
		Steps: []TemplateStep{
			{Action: "data_validate", Params: nil, DependsOn: []int{}},
			{Action: "data_extract", Params: nil, DependsOn: []int{0}},
			{Action: "data_transform", Params: nil, DependsOn: []int{1}},
			{Action: "data_load", Params: nil, DependsOn: []int{2}},
		},
	}
	p.templates["content_generate"] = &TaskTemplate{
		Name: "content_generate",
		Steps: []TemplateStep{
			{Action: "content_plan", Params: nil, DependsOn: []int{}},
			{Action: "content_draft", Params: nil, DependsOn: []int{0}},
			{Action: "content_review", Params: nil, DependsOn: []int{1}},
			{Action: "content_finalize", Params: nil, DependsOn: []int{2}},
		},
	}
	p.templates["analysis"] = &TaskTemplate{
		Name: "analysis",
		Steps: []TemplateStep{
			{Action: "data_collection", Params: nil, DependsOn: []int{}},
			{Action: "data_cleaning", Params: nil, DependsOn: []int{0}},
			{Action: "data_analysis", Params: nil, DependsOn: []int{1}},
			{Action: "report_generate", Params: nil, DependsOn: []int{2}},
		},
	}
	p.templates["chat"] = &TaskTemplate{
		Name: "chat",
		Steps: []TemplateStep{
			{Action: "understand_intent", Params: nil, DependsOn: []int{}},
			{Action: "process_request", Params: nil, DependsOn: []int{0}},
			{Action: "generate_response", Params: nil, DependsOn: []int{1}},
		},
	}
	p.templates["code_generate"] = &TaskTemplate{
		Name: "code_generate",
		Steps: []TemplateStep{
			{Action: "analyze_requirement", Params: nil, DependsOn: []int{}},
			{Action: "design_solution", Params: nil, DependsOn: []int{0}},
			{Action: "implement_code", Params: nil, DependsOn: []int{1}},
			{Action: "test_code", Params: nil, DependsOn: []int{2}},
		},
	}
}

func (p *TplPlanner) PlanSubTasks(ctx context.Context, task *Task) ([]SubTaskDef, error) {
	reportPlan(ctx, "THINK", "开始模板规划")
	template, ok := p.templates[task.SkillName]
	if !ok {
		template = p.templates["default"]
	}
	subTasks := make([]SubTaskDef, 0, len(template.Steps))
	for i, step := range template.Steps {
		subTaskID := fmt.Sprintf("%s_step_%d", task.TaskID, i+1)
		dependsOn := make([]string, 0, len(step.DependsOn))
		for _, depIdx := range step.DependsOn {
			depSubTaskID := fmt.Sprintf("%s_step_%d", task.TaskID, depIdx+1)
			dependsOn = append(dependsOn, depSubTaskID)
		}
		mergedParams := make(map[string]any)
		for k, v := range task.Params {
			mergedParams[k] = v
		}
		for k, v := range step.Params {
			mergedParams[k] = v
		}
		mergedParams["skill_name"] = task.SkillName
		mergedParams["action"] = step.Action
		subTasks = append(subTasks, SubTaskDef{
			SubTaskID: subTaskID,
			TaskID:    task.TaskID,
			Action:    step.Action,
			Params:    mergedParams,
			DependsOn: dependsOn,
			Status:    statemachine.SubStatusPending,
		})
	}
	reportPlan(ctx, "STATUS", fmt.Sprintf("模板规划完成，子任务数=%d", len(subTasks)))
	return subTasks, nil
}

func (p *TplPlanner) GetTemplate(name string) *TaskTemplate {
	return p.templates[name]
}

func (p *TplPlanner) ListTemplates() []string {
	templates := make([]string, 0, len(p.templates))
	for name := range p.templates {
		templates = append(templates, name)
	}
	return templates
}
