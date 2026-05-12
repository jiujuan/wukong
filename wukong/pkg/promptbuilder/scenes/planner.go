package scenes

import (
	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
)

const PlannerSceneName = "planner"

func NewPlannerPreset(templateKey string, setup func(*promptbuilder.Builder) error) promptbuilder.ScenePreset {
	return Preset{
		SceneName:   PlannerSceneName,
		TemplateKey: templateKey,
		SceneConfig: wkcontext.SceneConfig{
			Name:    PlannerSceneName,
			Sources: []string{"task_state", "skill_spec"},
		},
		Setup: setup,
	}
}
