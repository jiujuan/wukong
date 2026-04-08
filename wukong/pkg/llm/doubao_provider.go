package llm

import "context"

type doubaoProvider struct{}

func (p *doubaoProvider) Name() ProviderType { return ProviderTypeDoubao }

func (p *doubaoProvider) Chat(ctx context.Context, provider *Provider, messages []Message) (*ChatResponse, error) {
	return doOpenAPIChat(ctx, provider, messages)
}

func (p *doubaoProvider) StreamChat(ctx context.Context, provider *Provider, messages []Message, handler func(chunk string)) error {
	return doOpenAPIStreamChat(ctx, provider, messages, handler)
}
