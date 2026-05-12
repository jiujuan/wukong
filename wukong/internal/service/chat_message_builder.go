package service

import (
	"fmt"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/prompt"
	"github.com/jiujuan/wukong/pkg/promptbuilder"
	"github.com/jiujuan/wukong/pkg/promptbuilder/scenes"
)

type chatSceneAssembler struct{}

func newChatPromptBuilder(contextEngine *ctxengine.Engine, promptEngine *prompt.Engine) *promptbuilder.Builder {
	factory := promptbuilder.NewFactory(
		promptbuilder.WithContextEngineFactory(func() *ctxengine.Engine { return contextEngine }),
		promptbuilder.WithPromptEngineFactory(func() *prompt.Engine { return promptEngine }),
	)
	factory.RegisterPreset(scenes.Preset{
		SceneName: chatSceneName,
		SceneConfig: ctxengine.SceneConfig{
			Name:    chatSceneName,
			Sources: []string{chatMemorySourceName, chatHistorySourceName},
		},
		TemplateKey: prompt.TemplateChatSessionDefault,
		Setup: func(b *promptbuilder.Builder) error {
			b.RegisterAssembler(chatSceneName, chatSceneAssembler{})
			return nil
		},
	})
	return factory.MustForScene(chatSceneName)
}

func (a chatSceneAssembler) BuildPromptInput(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle) prompt.RenderInput {
	currentUserMessage := strings.TrimSpace(stringifyChatValue(req.Variables["current_user_message"]))
	if currentUserMessage == "" {
		currentUserMessage = strings.TrimSpace(req.Context.Query)
	}
	return prompt.RenderInput{
		Variables: map[string]any{
			"memory_text":          memoryTextFromBundle(bundle),
			"current_user_message": currentUserMessage,
		},
	}
}

func (a chatSceneAssembler) Assemble(req promptbuilder.BuildRequest, bundle *ctxengine.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
	if len(promptMessages) == 0 {
		return nil, nil
	}

	blockCount := 0
	if bundle != nil {
		blockCount = len(bundle.Blocks)
	}
	messages := make([]llm.Message, 0, len(promptMessages)+blockCount)
	if len(promptMessages) > 1 {
		for _, item := range filterPromptMessages(promptMessages[:len(promptMessages)-1]) {
			messages = append(messages, item)
		}
	}
	for _, item := range historyMessagesFromBundle(bundle) {
		messages = append(messages, llm.Message{Role: item.Role, Content: item.Content})
	}
	last := promptMessages[len(promptMessages)-1]
	if strings.TrimSpace(last.Role) != "" && strings.TrimSpace(last.Content) != "" {
		messages = append(messages, last)
	}
	return messages, nil
}

func stringifyChatValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
