package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/statemachine"
	wkstr "github.com/jiujuan/wukong/pkg/str"
)

type LLMPlanner struct {
	provider      *llm.Provider
	fallback      TaskPlanner
	promptEngine  *prompt.Engine
	contextEngine *ctxengine.Engine
	promptBuilder *promptbuilder.Builder
}

func NewLLMPlanner(provider *llm.Provider, fallback TaskPlanner) *LLMPlanner {
	return NewLLMPlannerWithRegistry(provider, fallback, nil)
}

func NewLLMPlannerWithRegistry(provider *llm.Provider, fallback TaskPlanner, registry *skills.Registry) *LLMPlanner {
	if fallback == nil {
		fallback = NewTplPlanner()
	}
	promptEngine := prompt.NewDefaultEngine()
	contextEngine := newPlannerContextEngine(&registrySkillSpecLoader{registry: registry})
	return &LLMPlanner{
		provider:      provider,
		fallback:      fallback,
		promptEngine:  promptEngine,
		contextEngine: contextEngine,
		promptBuilder: newPlannerPromptBuilder(contextEngine, promptEngine),
	}
}

func (p *LLMPlanner) Name() string { return "llm" }

type llmPlanPayload struct {
	Thought string        `json:"thought"`
	Steps   []llmPlanStep `json:"steps"`
}

type llmPlanStep struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params"`
	DependsOn []string       `json:"depends_on"`
	Thought   string         `json:"thought"`
}

func (p *LLMPlanner) PlanSubTasks(ctx context.Context, task *Task) ([]SubTaskDef, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}
	if p.provider == nil {
		reportPlan(ctx, "STATUS", "LLM规划器不可用，降级模板规划")
		return p.fallback.PlanSubTasks(ctx, task)
	}

	reportPlan(ctx, "THINK", "开始基于LLM分析意图并规划DAG")
	plan, err := p.planByLLM(ctx, task)
	if err != nil {
		reportPlan(ctx, "TOOL", fmt.Sprintf("LLM规划失败，降级模板规划: %v", err))
		return p.fallback.PlanSubTasks(ctx, task)
	}
	defs, convErr := p.convert(ctx, task, plan)
	if convErr != nil || len(defs) == 0 {
		if convErr != nil {
			reportPlan(ctx, "TOOL", fmt.Sprintf("LLM规划结果不合法，降级模板规划: %v", convErr))
		} else {
			reportPlan(ctx, "TOOL", "LLM规划结果为空，降级模板规划")
		}
		return p.fallback.PlanSubTasks(ctx, task)
	}
	reportPlan(ctx, "STATUS", fmt.Sprintf("LLM规划完成，子任务数=%d", len(defs)))
	return defs, nil
}

func (p *LLMPlanner) planByLLM(ctx context.Context, task *Task) (*llmPlanPayload, error) {
	paramsJSON, _ := json.Marshal(task.Params)
	buildResult, err := p.promptBuilder.BuildMessages(ctx, promptbuilder.BuildRequest{
		Scene:       plannerSceneName,
		TemplateKey: prompt.TemplatePlannerTaskDefault,
		Context:     buildPlannerContextRequest(task),
		Variables: map[string]any{
			"task_id":     task.TaskID,
			"skill_name":  task.SkillName,
			"params_json": string(paramsJSON),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("render planner prompt failed: %w", err)
	}
	resp, err := p.provider.Chat(ctx, buildResult.Messages)
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm response")
	}
	content := sanitizeJSON(resp.Choices[0].Message.Content)
	plan := &llmPlanPayload{}
	if err := json.Unmarshal([]byte(content), plan); err != nil {
		return nil, err
	}
	if wkstr.NotEmpty(plan.Thought) {
		reportPlan(ctx, "THINK", plan.Thought)
	}
	return plan, nil
}

func (p *LLMPlanner) convert(ctx context.Context, task *Task, payload *llmPlanPayload) ([]SubTaskDef, error) {
	if payload == nil || len(payload.Steps) == 0 {
		return nil, fmt.Errorf("empty plan steps")
	}
	idToSubID := make(map[string]string, len(payload.Steps))
	for i, step := range payload.Steps {
		rawID := wkstr.Trim(step.ID)
		if rawID == "" {
			rawID = fmt.Sprintf("s%d", i+1)
		}
		idToSubID[rawID] = fmt.Sprintf("%s_step_%d", task.TaskID, i+1)
	}

	defs := make([]SubTaskDef, 0, len(payload.Steps))
	for i, step := range payload.Steps {
		rawID := wkstr.Trim(step.ID)
		if rawID == "" {
			rawID = fmt.Sprintf("s%d", i+1)
		}
		action := wkstr.TrimLower(step.Action)
		if action == "" {
			return nil, fmt.Errorf("step[%d] action empty", i)
		}
		params := map[string]any{}
		for k, v := range task.Params {
			params[k] = v
		}
		for k, v := range step.Params {
			params[k] = v
		}
		params["skill_name"] = task.SkillName
		params["action"] = action
		if wkstr.NotEmpty(step.Thought) {
			params["plan_thought"] = wkstr.Trim(step.Thought)
			reportPlan(ctx, "THINK", fmt.Sprintf("步骤%s：%s", rawID, wkstr.Trim(step.Thought)))
		}

		dependsOn := make([]string, 0, len(step.DependsOn))
		for _, dep := range step.DependsOn {
			d := wkstr.Trim(dep)
			if d == "" {
				continue
			}
			subID, ok := idToSubID[d]
			if !ok {
				return nil, fmt.Errorf("step[%d] depends_on not found: %s", i, d)
			}
			dependsOn = append(dependsOn, subID)
		}
		defs = append(defs, SubTaskDef{
			SubTaskID: idToSubID[rawID],
			TaskID:    task.TaskID,
			Action:    action,
			Params:    params,
			DependsOn: dependsOn,
			Status:    statemachine.SubStatusPending,
		})
	}
	return defs, nil
}

func sanitizeJSON(raw string) string {
	text := wkstr.Trim(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return wkstr.Trim(text)
}
