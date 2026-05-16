package action

import (
	"context"
	"errors"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestToolExecutorFakeReturnsMapResult(t *testing.T) {
	manager := &fakeToolManager{
		result: map[string]any{"ok": true},
	}
	executor := NewToolExecutor(manager)
	step := agent.PlanStep{
		StepID: "step-1",
		Type:   agent.StepTypeTool,
		Action: "search",
		Params: map[string]any{"query": "wukong"},
	}

	if !executor.CanExecute(context.Background(), agent.AgentContext{}, step) {
		t.Fatal("CanExecute() = false, want true")
	}
	result, err := executor.Execute(context.Background(), agent.AgentContext{}, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != "completed" || result.Result["ok"] != true {
		t.Fatalf("StepResult = %#v, want completed map result", result)
	}
	if manager.name != "search" || manager.params["query"] != "wukong" {
		t.Fatalf("tool call = %q %#v, want search query", manager.name, manager.params)
	}
}

func TestToolExecutorPropagatesError(t *testing.T) {
	wantErr := errors.New("tool failed")
	executor := NewToolExecutor(&fakeToolManager{err: wantErr})

	result, err := executor.Execute(context.Background(), agent.AgentContext{}, agent.PlanStep{
		StepID: "step-1",
		Type:   agent.StepTypeTool,
		Action: "search",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if result == nil || result.Status != "failed" || result.Error != "tool failed" {
		t.Fatalf("StepResult = %#v, want failed error result", result)
	}
}

type fakeToolManager struct {
	name   string
	params map[string]any
	result map[string]any
	err    error
}

func (m *fakeToolManager) Execute(ctx context.Context, name string, params map[string]any) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.name = name
	m.params = params
	return m.result, m.err
}
