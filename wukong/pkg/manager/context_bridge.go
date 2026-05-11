package manager

import (
	"context"
	"encoding/json"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
)

const plannerSceneName = "planner"

func newPlannerContextEngine(loader ctxengine.SkillSpecLoader) *ctxengine.Engine {
	engine := ctxengine.New()
	_ = engine.RegisterSource(ctxengine.NewTaskStateSource())
	_ = engine.RegisterSource(ctxengine.NewSkillSpecSource(loader))
	_ = engine.RegisterScene(ctxengine.SceneConfig{
		Name:    plannerSceneName,
		Sources: []string{"task_state", "skill_spec"},
	})
	return engine
}

type plannerSceneAssembler struct{}

func newPlannerPromptBuilder(contextEngine *ctxengine.Engine, promptEngine *prompt.Engine) *promptbuilder.Builder {
	builder := promptbuilder.New(contextEngine, promptEngine)
	builder.BindSceneTemplate(plannerSceneName, prompt.TemplatePlannerTaskDefault)
	builder.RegisterAssembler(plannerSceneName, plannerSceneAssembler{})
	return builder
}

func (a plannerSceneAssembler) BuildPromptInput(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle) prompt.RenderInput {
	return prompt.RenderInput{
		Variables: req.Variables,
		Context:   plannerPromptContext(bundle),
	}
}

func (a plannerSceneAssembler) Assemble(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	return append([]llm.Message(nil), promptMessages...), nil
}

func buildPlannerContextBundle(ctx context.Context, engine *ctxengine.Engine, task *Task) *ctxengine.ContextBundle {
	if engine == nil || task == nil {
		return nil
	}
	req := buildPlannerContextRequest(task)
	bundle, err := engine.Build(ctx, req)
	if err != nil {
		return nil
	}
	return bundle
}

func buildPlannerContextRequest(task *Task) ctxengine.BuildRequest {
	if task == nil {
		return ctxengine.BuildRequest{Scene: plannerSceneName}
	}
	paramsJSON, _ := json.Marshal(task.Params)
	taskResultJSON := ""
	if task.Result != nil {
		data, _ := json.Marshal(task.Result)
		taskResultJSON = string(data)
	}
	query := extractPlannerQuery(task)
	return ctxengine.BuildRequest{
		Scene:     plannerSceneName,
		UserID:    task.UserID,
		SessionID: task.SessionID,
		TaskID:    task.TaskID,
		SkillName: task.SkillName,
		Query:     query,
		Variables: map[string]any{
			"task_status":      task.Status,
			"params_json":      string(paramsJSON),
			"task_params_json": string(paramsJSON),
			"task_result_json": taskResultJSON,
		},
	}
}

func plannerPromptContext(bundle *ctxengine.ContextBundle) map[string]any {
	if bundle == nil {
		return nil
	}
	return map[string]any{
		"task_state_text": bundle.Named[ctxengine.BlockTaskStateText],
		"skill_spec_text": bundle.Named[ctxengine.BlockSkillSpecText],
		"text":            bundle.Text,
	}
}

func extractPlannerQuery(task *Task) string {
	if task == nil {
		return ""
	}
	for _, key := range []string{"query", "topic", "prompt", "title", "goal"} {
		if raw, ok := task.Params[key]; ok && raw != nil {
			if text := stringifyPlannerValue(raw); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringifyPlannerValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}
