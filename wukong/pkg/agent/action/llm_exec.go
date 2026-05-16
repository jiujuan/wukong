package action

import (
	"context"
	"fmt"
	"time"

	"github.com/jiujuan/wukong/pkg/agent"
)

// LLMMessage is the minimal chat message shape required by LLMExecutor.
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMProvider is the minimal LLM dependency used by LLMExecutor.
type LLMProvider interface {
	Chat(ctx context.Context, messages []LLMMessage) (string, error)
}

// LLMExecutor executes llm plan steps through an injected provider.
type LLMExecutor struct {
	provider LLMProvider
}

// NewLLMExecutor creates an LLM action executor skeleton.
func NewLLMExecutor(provider LLMProvider) *LLMExecutor {
	return &LLMExecutor{provider: provider}
}

// Name returns the executor name.
func (e *LLMExecutor) Name() string {
	return "llm"
}

// CanExecute reports whether this executor can handle the step.
func (e *LLMExecutor) CanExecute(ctx context.Context, _ agent.AgentContext, step agent.PlanStep) bool {
	return ctx.Err() == nil && step.Type == agent.StepTypeLLM
}

// Execute calls the injected LLM provider and records the output.
func (e *LLMExecutor) Execute(ctx context.Context, agentCtx agent.AgentContext, step agent.PlanStep) (*agent.StepResult, error) {
	startedAt := time.Now()
	result := baseStepResult(step, "running", startedAt)
	if err := ctx.Err(); err != nil {
		return failStepResult(result, err), err
	}
	if e.provider == nil {
		err := fmt.Errorf("llm provider is not configured")
		return failStepResult(result, err), err
	}

	output, err := e.provider.Chat(ctx, buildLLMMessages(agentCtx, step))
	if err != nil {
		return failStepResult(result, err), err
	}
	result.Status = "completed"
	result.Output = output
	result.CompletedAt = time.Now()
	return result, nil
}

func buildLLMMessages(agentCtx agent.AgentContext, step agent.PlanStep) []LLMMessage {
	content := step.Expected
	if content == "" {
		content = step.Thought
	}
	if content == "" {
		content = agentCtx.Request.Goal
	}
	if content == "" {
		content = "Complete the requested step."
	}
	return []LLMMessage{
		{Role: "user", Content: content},
	}
}
