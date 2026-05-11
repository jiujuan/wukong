package worker

import (
	"context"
	"encoding/json"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/messagebuilder"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/skills"
)

const workerSceneName = "worker"

type workerSkillSpecLoader struct {
	registry *skills.Registry
}

func (l *workerSkillSpecLoader) LoadSkillSpec(ctx context.Context, skillName string) (*ctxengine.SkillSpec, error) {
	if l == nil || l.registry == nil {
		return nil, nil
	}
	item, ok := l.registry.Get(skillName)
	if !ok || item == nil {
		return nil, nil
	}
	params := make([]ctxengine.SkillParam, 0, len(item.Params))
	for _, param := range item.Params {
		params = append(params, ctxengine.SkillParam{
			Name:       param.Name,
			Type:       param.Type,
			Required:   param.Required,
			DefaultVal: param.DefaultVal,
		})
	}
	return &ctxengine.SkillSpec{
		SkillName:      item.SkillName,
		Description:    item.Description,
		Version:        item.Version,
		Enabled:        item.Enabled,
		Tools:          append([]string(nil), item.Tools...),
		Params:         params,
		MemoryType:     item.Memory.MemoryType,
		MemoryWindow:   item.Memory.WindowSize,
		MemoryCompress: item.Memory.CompressSwitch,
	}, nil
}

func newWorkerContextEngine(registry *skills.Registry) *ctxengine.Engine {
	engine := ctxengine.New()
	_ = engine.RegisterSource(ctxengine.NewTaskStateSource())
	_ = engine.RegisterSource(ctxengine.NewSkillSpecSource(&workerSkillSpecLoader{registry: registry}))
	_ = engine.RegisterScene(ctxengine.SceneConfig{
		Name:    workerSceneName,
		Sources: []string{"task_state", "skill_spec"},
	})
	return engine
}

type workerSceneAssembler struct{}

func newWorkerMessageBuilder(contextEngine *ctxengine.Engine, promptEngine *prompt.Engine) *messagebuilder.Builder {
	builder := messagebuilder.New(contextEngine, promptEngine)
	builder.BindSceneTemplate(workerSceneName, prompt.TemplateWorkerActionDefault)
	builder.RegisterAssembler(workerSceneName, workerSceneAssembler{})
	return builder
}

func (a workerSceneAssembler) BuildPromptInput(req messagebuilder.BuildRequest, bundle *ctxengine.ContextBundle) prompt.RenderInput {
	return prompt.RenderInput{
		Variables: req.Variables,
		Context:   workerPromptContext(bundle),
	}
}

func (a workerSceneAssembler) Assemble(req messagebuilder.BuildRequest, bundle *ctxengine.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	return append([]llm.Message(nil), promptMessages...), nil
}

func buildWorkerContextBundle(ctx context.Context, engine *ctxengine.Engine, subTask executableSubTask) *ctxengine.ContextBundle {
	if engine == nil || subTask == nil {
		return nil
	}
	req := buildWorkerContextRequest(subTask)
	bundle, err := engine.Build(ctx, req)
	if err != nil {
		return nil
	}
	return bundle
}

func buildWorkerContextRequest(subTask executableSubTask) ctxengine.BuildRequest {
	if subTask == nil {
		return ctxengine.BuildRequest{Scene: workerSceneName}
	}
	params := cloneParams(subTask.GetParams())
	paramsJSON, _ := json.Marshal(params)
	taskResultJSON := ""
	if raw, ok := params["task_result"]; ok && raw != nil {
		data, _ := json.Marshal(raw)
		taskResultJSON = string(data)
	}
	return ctxengine.BuildRequest{
		Scene:     workerSceneName,
		TaskID:    subTask.GetTaskID(),
		SkillName: resolveSkillName(subTask.GetAction(), params),
		Query:     extractStringParam(params, "query", "q", "keyword", "topic", "prompt"),
		Variables: map[string]any{
			"sub_task_id":      subTask.GetSubTaskID(),
			"action":           subTask.GetAction(),
			"params_json":      string(paramsJSON),
			"task_params_json": string(paramsJSON),
			"plan_thought":     extractStringParam(params, "plan_thought"),
			"task_status":      strings.TrimSpace(extractStringParam(params, "task_status")),
			"subtask_status":   strings.TrimSpace(extractStringParam(params, "subtask_status")),
			"task_result_json": taskResultJSON,
		},
	}
}

func workerPromptContext(bundle *ctxengine.ContextBundle) map[string]any {
	if bundle == nil {
		return nil
	}
	return map[string]any{
		"task_state_text": bundle.Named[ctxengine.BlockTaskStateText],
		"skill_spec_text": bundle.Named[ctxengine.BlockSkillSpecText],
		"text":            bundle.Text,
	}
}
