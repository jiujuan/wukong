package scenes

import "github.com/jiujuan/wukong/pkg/promptbuilder"

const ChatSceneName = "chat"

func NewChatPreset(templateKey string, setup func(*promptbuilder.Builder) error) promptbuilder.ScenePreset {
	return Preset{
		SceneName:   ChatSceneName,
		TemplateKey: templateKey,
		Setup:       setup,
	}
}
