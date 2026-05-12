package scenes

import "github.com/jiujuan/wukong/pkg/promptbuilder"

const WorkerSceneName = "worker"

func NewWorkerPreset(templateKey string, setup func(*promptbuilder.Builder) error) promptbuilder.ScenePreset {
	return Preset{
		SceneName:   WorkerSceneName,
		TemplateKey: templateKey,
		Setup:       setup,
	}
}
