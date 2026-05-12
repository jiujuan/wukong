package scenes

import (
	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
)

const WorkerSceneName = "worker"

func NewWorkerPreset(templateKey string, setup func(*promptbuilder.Builder) error) promptbuilder.ScenePreset {
	return Preset{
		SceneName:   WorkerSceneName,
		TemplateKey: templateKey,
		SceneConfig: wkcontext.SceneConfig{
			Name:    WorkerSceneName,
			Sources: []string{"task_state", "skill_spec"},
		},
		Setup: setup,
	}
}
