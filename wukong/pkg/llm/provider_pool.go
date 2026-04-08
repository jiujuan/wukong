package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type PoolMember struct {
	Name     string
	Priority int
	Provider *Provider
}

type PoolOption func(*ProviderPool)

type providerNode struct {
	name         string
	priority     int
	provider     *Provider
	failureCount int
	openUntil    time.Time
}

type ProviderPool struct {
	mu               sync.Mutex
	nodes            []*providerNode
	maxRetries       int
	failureThreshold int
	cooldown         time.Duration
	retryBackoff     time.Duration
}

func WithPoolMaxRetries(maxRetries int) PoolOption {
	return func(p *ProviderPool) {
		if maxRetries >= 0 {
			p.maxRetries = maxRetries
		}
	}
}

func WithPoolFailureThreshold(threshold int) PoolOption {
	return func(p *ProviderPool) {
		if threshold > 0 {
			p.failureThreshold = threshold
		}
	}
}

func WithPoolCooldown(cooldown time.Duration) PoolOption {
	return func(p *ProviderPool) {
		if cooldown > 0 {
			p.cooldown = cooldown
		}
	}
}

func WithPoolRetryBackoff(backoff time.Duration) PoolOption {
	return func(p *ProviderPool) {
		if backoff > 0 {
			p.retryBackoff = backoff
		}
	}
}

func NewProviderPool(members []PoolMember, opts ...PoolOption) *ProviderPool {
	pool := &ProviderPool{
		nodes:            make([]*providerNode, 0, len(members)),
		maxRetries:       1,
		failureThreshold: 3,
		cooldown:         15 * time.Second,
		retryBackoff:     200 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(pool)
	}
	for idx, member := range members {
		if member.Provider == nil {
			continue
		}
		name := strings.TrimSpace(member.Name)
		if name == "" {
			name = fmt.Sprintf("%s#%d", member.Provider.providerType, idx+1)
		}
		pool.nodes = append(pool.nodes, &providerNode{
			name:     name,
			priority: member.Priority,
			provider: member.Provider,
		})
	}
	sort.Slice(pool.nodes, func(i, j int) bool {
		if pool.nodes[i].priority == pool.nodes[j].priority {
			return pool.nodes[i].name < pool.nodes[j].name
		}
		return pool.nodes[i].priority < pool.nodes[j].priority
	})
	return pool
}

func (p *ProviderPool) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		nodes := p.availableNodes()
		if len(nodes) == 0 {
			lastErr = fmt.Errorf("no available llm providers")
			if !p.waitRetry(ctx, attempt) {
				break
			}
			continue
		}
		for _, node := range nodes {
			resp, err := node.provider.chatDirect(ctx, messages)
			if err == nil {
				p.markSuccess(node)
				return resp, nil
			}
			lastErr = fmt.Errorf("provider %s failed: %w", node.name, err)
			p.markFailure(node)
		}
		if !p.waitRetry(ctx, attempt) {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("provider pool request failed")
	}
	return nil, lastErr
}

func (p *ProviderPool) StreamChat(ctx context.Context, messages []Message, handler func(chunk string)) error {
	var lastErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		nodes := p.availableNodes()
		if len(nodes) == 0 {
			lastErr = fmt.Errorf("no available llm providers")
			if !p.waitRetry(ctx, attempt) {
				break
			}
			continue
		}
		for _, node := range nodes {
			err := node.provider.streamDirect(ctx, messages, handler)
			if err == nil {
				p.markSuccess(node)
				return nil
			}
			lastErr = fmt.Errorf("provider %s failed: %w", node.name, err)
			p.markFailure(node)
		}
		if !p.waitRetry(ctx, attempt) {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("provider pool stream request failed")
	}
	return lastErr
}

func (p *ProviderPool) availableNodes() []*providerNode {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	available := make([]*providerNode, 0, len(p.nodes))
	for _, node := range p.nodes {
		if node.openUntil.After(now) {
			continue
		}
		available = append(available, node)
	}
	return available
}

func (p *ProviderPool) markSuccess(node *providerNode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	node.failureCount = 0
	node.openUntil = time.Time{}
}

func (p *ProviderPool) markFailure(node *providerNode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	node.failureCount++
	if node.failureCount >= p.failureThreshold {
		node.openUntil = time.Now().Add(p.cooldown)
	}
}

func (p *ProviderPool) waitRetry(ctx context.Context, attempt int) bool {
	if attempt >= p.maxRetries {
		return false
	}
	backoff := p.retryBackoff
	if backoff <= 0 {
		return true
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
