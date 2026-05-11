package service

import (
	"fmt"
	"strings"

	ctxengine "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/messagebuilder"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type chatSceneAssembler struct{}

func newChatMessageBuilder(contextEngine *ctxengine.Engine, promptEngine *prompt.Engine) *messagebuilder.Builder {
	builder := messagebuilder.New(contextEngine, promptEngine)
	builder.BindSceneTemplate(chatSceneName, prompt.TemplateChatSessionDefault)
	builder.RegisterAssembler(chatSceneName, chatSceneAssembler{})
	return builder
}

func (a chatSceneAssembler) BuildPromptInput(req messagebuilder.BuildRequest, bundle *ctxengine.ContextBundle) prompt.RenderInput {
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

func (a chatSceneAssembler) Assemble(req messagebuilder.BuildRequest, bundle *ctxengine.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error) {
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
