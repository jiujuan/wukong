package scenes

import (
	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
)

type Preset struct {
	SceneName   string
	TemplateKey string
	SceneConfig wkcontext.SceneConfig
	Setup       func(*promptbuilder.Builder) error
}

func (p Preset) Name() string {
	return p.SceneName
}

func (p Preset) Apply(builder *promptbuilder.Builder) error {
	if builder == nil {
		return promptbuilder.ErrBuilderNil
	}
	sceneConfig := p.SceneConfig
	if sceneConfig.Name == "" {
		sceneConfig.Name = p.SceneName
	}
	if sceneConfig.Name != "" {
		if err := builder.RegisterScene(sceneConfig); err != nil {
			return err
		}
	}
	if p.TemplateKey != "" {
		builder.BindSceneTemplate(p.SceneName, p.TemplateKey)
	}
	if p.Setup != nil {
		return p.Setup(builder)
	}
	return nil
}
