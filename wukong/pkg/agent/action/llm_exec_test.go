package action

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestLLMExecutorFakeReturnsOutput(t *testing.T) {
	provider := &fakeLLMProvider{output: "hello from llm"}
	executor := NewLLMExecutor(provider)
	step := agent.PlanStep{
		StepID:   "step-1",
		Type:     agent.StepTypeLLM,
		Thought:  "Say hello",
		Expected: "short greeting",
	}

	if !executor.CanExecute(context.Background(), agent.AgentContext{}, step) {
		t.Fatal("CanExecute() = false, want true")
	}
	result, err := executor.Execute(context.Background(), agent.AgentContext{}, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != "completed" || result.Output != "hello from llm" {
		t.Fatalf("StepResult = %#v, want completed output", result)
	}
	if len(provider.messages) != 1 || provider.messages[0].Content != "short greeting" {
		t.Fatalf("messages = %#v, want expected prompt", provider.messages)
	}
}

type fakeLLMProvider struct {
	output   string
	err      error
	messages []LLMMessage
}

func (p *fakeLLMProvider) Chat(ctx context.Context, messages []LLMMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	p.messages = append([]LLMMessage(nil), messages...)
	return p.output, p.err
}
