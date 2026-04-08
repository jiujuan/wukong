package llm

import "context"

type deepseekProvider struct{}

func (p *deepseekProvider) Name() ProviderType { return ProviderTypeDeepSeek }

func (p *deepseekProvider) Chat(ctx context.Context, provider *Provider, messages []Message) (*ChatResponse, error) {
	return doOpenAPIChat(ctx, provider, messages)
}

func (p *deepseekProvider) StreamChat(ctx context.Context, provider *Provider, messages []Message, handler func(chunk string)) error {
	return doOpenAPIStreamChat(ctx, provider, messages, handler)
}
