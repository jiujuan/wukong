package promptbuilder

import wkcontext "github.com/jiujuan/wukong/pkg/context"

type ScenePreset interface {
	Name() string
	Apply(*Builder) error
}

type FuncPreset struct {
	SceneName   string
	SceneConfig wkcontext.SceneConfig
	TemplateKey string
	Setup       func(*Builder) error
}

func (p FuncPreset) Name() string {
	return p.SceneName
}

func (p FuncPreset) Apply(b *Builder) error {
	sceneConfig := p.SceneConfig
	if sceneConfig.Name == "" {
		sceneConfig.Name = p.SceneName
	}
	if sceneConfig.Name != "" {
		if err := b.RegisterScene(sceneConfig); err != nil {
			return err
		}
	}
	if p.TemplateKey != "" {
		b.BindSceneTemplate(p.SceneName, p.TemplateKey)
	}
	if p.Setup != nil {
		return p.Setup(b)
	}
	return nil
}
