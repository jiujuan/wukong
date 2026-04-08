package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/statemachine"
)

type LLMPlanner struct {
	provider *llm.Provider
	fallback TaskPlanner
}

func NewLLMPlanner(provider *llm.Provider, fallback TaskPlanner) *LLMPlanner {
	if fallback == nil {
		fallback = NewTplPlanner()
	}
	return &LLMPlanner{
		provider: provider,
		fallback: fallback,
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
	systemPrompt := `你是任务规划器。把用户任务拆解为可执行DAG子任务。只输出JSON，不要输出任何解释。
JSON格式：
{"thought":"一句整体规划思路","steps":[{"id":"s1","action":"web_search","params":{},"depends_on":[],"thought":"该步骤思路"}]}
要求：
1) action 必须是简短可执行动作名
2) depends_on 引用步骤id
3) steps 至少1个，最多8个
4) 保证DAG无环`
	userPrompt := fmt.Sprintf("task_id=%s\nskill=%s\nparams=%s", task.TaskID, task.SkillName, string(paramsJSON))
	resp, err := p.provider.Chat(ctx, []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm response")
	}
	content := sanitizeJSON(resp.Choices[0].Message.Content)
	result := &llmPlanPayload{}
	if err := json.Unmarshal([]byte(content), result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Thought) != "" {
		reportPlan(ctx, "THINK", result.Thought)
	}
	return result, nil
}

func (p *LLMPlanner) convert(ctx context.Context, task *Task, payload *llmPlanPayload) ([]SubTaskDef, error) {
	if payload == nil || len(payload.Steps) == 0 {
		return nil, fmt.Errorf("empty plan steps")
	}
	idToSubID := make(map[string]string, len(payload.Steps))
	for i, step := range payload.Steps {
		rawID := strings.TrimSpace(step.ID)
		if rawID == "" {
			rawID = fmt.Sprintf("s%d", i+1)
		}
		idToSubID[rawID] = fmt.Sprintf("%s_step_%d", task.TaskID, i+1)
	}

	defs := make([]SubTaskDef, 0, len(payload.Steps))
	for i, step := range payload.Steps {
		rawID := strings.TrimSpace(step.ID)
		if rawID == "" {
			rawID = fmt.Sprintf("s%d", i+1)
		}
		action := strings.ToLower(strings.TrimSpace(step.Action))
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
		if strings.TrimSpace(step.Thought) != "" {
			params["plan_thought"] = strings.TrimSpace(step.Thought)
			reportPlan(ctx, "THINK", fmt.Sprintf("步骤%s：%s", rawID, strings.TrimSpace(step.Thought)))
		}

		dependsOn := make([]string, 0, len(step.DependsOn))
		for _, dep := range step.DependsOn {
			d := strings.TrimSpace(dep)
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
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}
