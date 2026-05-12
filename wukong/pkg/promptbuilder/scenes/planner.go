package scenes

import "github.com/jiujuan/wukong/pkg/promptbuilder"

const PlannerSceneName = "planner"

func NewPlannerPreset(templateKey string, setup func(*promptbuilder.Builder) error) promptbuilder.ScenePreset {
	return Preset{
		SceneName:   PlannerSceneName,
		TemplateKey: templateKey,
		Setup:       setup,
	}
}
