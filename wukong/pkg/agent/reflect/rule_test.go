package reflect

import (
	"context"
	"errors"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestNoopReflectorSuccessScorePositive(t *testing.T) {
	eval, err := NoopReflector{}.Reflect(context.Background(), agent.AgentContext{}, nil, &agent.ActionResult{
		Status: "completed",
		Output: "done",
	}, nil)
	if err != nil {
		t.Fatalf("Reflect() error = %v", err)
	}
	if !eval.Success || eval.Score <= 0 {
		t.Fatalf("Evaluation = %#v, want successful positive score", eval)
	}
}

func TestRuleReflectorSuccessResultScorePositive(t *testing.T) {
	eval, err := NewRuleReflector().Reflect(context.Background(), agent.AgentContext{}, nil, &agent.ActionResult{
		Status: "completed",
		Output: "done",
	}, nil)
	if err != nil {
		t.Fatalf("Reflect() error = %v", err)
	}
	if !eval.Success || eval.Score <= 0 {
		t.Fatalf("Evaluation = %#v, want successful positive score", eval)
	}
}

func TestRuleReflectorEmptyOutputFails(t *testing.T) {
	eval, err := NewRuleReflector().Reflect(context.Background(), agent.AgentContext{}, nil, &agent.ActionResult{
		Status: "completed",
		Output: " ",
	}, nil)
	if err != nil {
		t.Fatalf("Reflect() error = %v", err)
	}
	if eval.Success || !eval.Retry || eval.Reason != "empty action output" {
		t.Fatalf("Evaluation = %#v, want failed retry for empty output", eval)
	}
}

func TestRuleReflectorErrorTriggersRetryDecision(t *testing.T) {
	wantErr := errors.New("action failed")
	eval, err := NewRuleReflector().Reflect(context.Background(), agent.AgentContext{}, nil, &agent.ActionResult{
		Status: "failed",
		Error:  "result failed",
	}, wantErr)
	if err != nil {
		t.Fatalf("Reflect() error = %v", err)
	}
	if eval.Success || !eval.Retry {
		t.Fatalf("Evaluation = %#v, want failed retry", eval)
	}
	decision, ok := eval.Metadata["retry_decision"].(RetryDecision)
	if !ok {
		t.Fatalf("retry_decision = %#v, want RetryDecision", eval.Metadata["retry_decision"])
	}
	if !decision.ShouldRetry || decision.Reason != "action failed" {
		t.Fatalf("RetryDecision = %#v, want retry action failed", decision)
	}
}

func TestRuleReflectorLowConfidenceFails(t *testing.T) {
	eval, err := NewRuleReflector().Reflect(context.Background(), agent.AgentContext{}, nil, &agent.ActionResult{
		Status: "completed",
		Output: "done",
		Metadata: map[string]any{
			"confidence": 0.2,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Reflect() error = %v", err)
	}
	if eval.Success || !eval.Retry || eval.Reason != "low confidence action result" {
		t.Fatalf("Evaluation = %#v, want low confidence failure", eval)
	}
}
