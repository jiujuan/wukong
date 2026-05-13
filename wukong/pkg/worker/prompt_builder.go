package worker

import (
	"context"
	"encoding/json"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
	"github.com/jiujuan/wukong/pkg/promptbuilder/scenes"
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
	canon := item.Canonical()
	return &ctxengine.SkillSpec{
		SkillName:      item.SkillName,
		Description:    item.Description,
		Version:        item.Version,
		Enabled:        item.Enabled,
		SourceType:     canon.Source.Type,
		RootDir:        canon.Source.RootDir,
		Runtime:        canon.Runtime.Runtime,
		Entry:          canon.Runtime.Entry,
		Tools:          append([]string(nil), item.Tools...),
		Params:         params,
		MemoryType:     item.Memory.MemoryType,
		MemoryWindow:   item.Memory.WindowSize,
		MemoryCompress: item.Memory.CompressSwitch,
		References:     append([]string(nil), item.References...),
		Assets:         append([]string(nil), item.Assets...),
		Metadata:       cloneAnyMap(item.Metadata),
	}, nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
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

func newWorkerPromptBuilder(contextEngine *ctxengine.Engine, promptEngine *prompt.Engine) *promptbuilder.Builder {
	factory := promptbuilder.NewFactory(
		promptbuilder.WithContextEngineFactory(func() *ctxengine.Engine { return contextEngine }),
		promptbuilder.WithPromptEngineFactory(func() *prompt.Engine { return promptEngine }),
	)
	factory.RegisterPreset(scenes.NewWorkerPreset(
		prompt.TemplateWorkerActionDefault,
		func(b *promptbuilder.Builder) error {
			b.RegisterAssembler(workerSceneName, workerSceneAssembler{})
			return nil
		},
	))
	return factory.MustForScene(workerSceneName)
}

func (a workerSceneAssembler) BuildPromptInput(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle) prompt.RenderInput {
	return prompt.RenderInput{
		Variables: req.Variables,
		Context:   workerPromptContext(bundle),
	}
}

func (a workerSceneAssembler) Assemble(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
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
