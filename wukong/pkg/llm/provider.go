package llm

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type ProviderType string

const (
	ProviderTypeDeepSeek ProviderType = "deepseek"
	ProviderTypeOllama   ProviderType = "ollama"
	ProviderTypeOpenAPI  ProviderType = "openapi"
	ProviderTypeDoubao   ProviderType = "doubao"
)

type Option func(*Provider)

type Provider struct {
	providerType ProviderType
	baseURL      string
	apiKey       string
	model        string
	timeout      time.Duration
	httpClient   *http.Client
	adapter      providerAdapter
	pool         *ProviderPool
}

type providerAdapter interface {
	Name() ProviderType
	Chat(ctx context.Context, p *Provider, messages []Message) (*ChatResponse, error)
	StreamChat(ctx context.Context, p *Provider, messages []Message, handler func(chunk string)) error
}

func New(opts ...Option) *Provider {
	p := &Provider{
		providerType: ProviderTypeDeepSeek,
		timeout:      60 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: p.timeout}
	} else if p.timeout > 0 {
		p.httpClient.Timeout = p.timeout
	}
	if p.timeout <= 0 {
		p.timeout = 60 * time.Second
		p.httpClient.Timeout = p.timeout
	}
	defaultBaseURL, defaultModel := defaultsForProviderType(p.providerType)
	if strings.TrimSpace(p.baseURL) == "" {
		p.baseURL = defaultBaseURL
	}
	if strings.TrimSpace(p.model) == "" {
		p.model = defaultModel
	}
	p.adapter = buildAdapter(p.providerType)
	return p
}

func WithProviderType(providerType ProviderType) Option {
	return func(p *Provider) {
		if strings.TrimSpace(string(providerType)) == "" {
			return
		}
		p.providerType = providerType
	}
}

func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = strings.TrimSpace(url)
	}
}

func WithAPIKey(key string) Option {
	return func(p *Provider) {
		p.apiKey = strings.TrimSpace(key)
	}
}

func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = strings.TrimSpace(model)
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(p *Provider) {
		if timeout <= 0 {
			return
		}
		p.timeout = timeout
		if p.httpClient != nil {
			p.httpClient.Timeout = timeout
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

func WithProviderPool(pool *ProviderPool) Option {
	return func(p *Provider) {
		p.pool = pool
	}
}

func (p *Provider) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if p.pool != nil {
		return p.pool.Chat(ctx, messages)
	}
	return p.chatDirect(ctx, messages)
}

func (p *Provider) StreamChat(ctx context.Context, messages []Message, handler func(chunk string)) error {
	if p.pool != nil {
		return p.pool.StreamChat(ctx, messages, handler)
	}
	return p.streamDirect(ctx, messages, handler)
}

func (p *Provider) SetProviderPool(pool *ProviderPool) {
	p.pool = pool
}

func (p *Provider) chatDirect(ctx context.Context, messages []Message) (*ChatResponse, error) {
	if p.adapter == nil {
		p.adapter = buildAdapter(p.providerType)
	}
	return p.adapter.Chat(ctx, p, messages)
}

func (p *Provider) streamDirect(ctx context.Context, messages []Message, handler func(chunk string)) error {
	if p.adapter == nil {
		p.adapter = buildAdapter(p.providerType)
	}
	return p.adapter.StreamChat(ctx, p, messages, handler)
}

func defaultsForProviderType(providerType ProviderType) (string, string) {
	switch providerType {
	case ProviderTypeOllama:
		return "http://127.0.0.1:11434", "qwen2.5:7b"
	case ProviderTypeOpenAPI:
		return "https://api.openai.com/v1", "gpt-4o-mini"
	case ProviderTypeDoubao:
		return "https://ark.cn-beijing.volces.com/api/v3", "doubao-pro-32k"
	default:
		return "https://api.deepseek.com/v1", "deepseek-chat"
	}
}

func buildAdapter(providerType ProviderType) providerAdapter {
	switch providerType {
	case ProviderTypeOllama:
		return &ollamaProvider{}
	case ProviderTypeOpenAPI:
		return &openAPIProvider{}
	case ProviderTypeDoubao:
		return &doubaoProvider{}
	default:
		return &deepseekProvider{}
	}
}
