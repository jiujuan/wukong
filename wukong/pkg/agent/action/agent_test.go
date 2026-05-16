package action

import (
	"context"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
)

func TestAgentExecutorConvertsDelegateStepToChildRunRequest(t *testing.T) {
	runtime := &fakeAgentRuntime{
		result: &agent.RunResult{
			RunID:   "run-1:delegate-1",
			AgentID: agent.AgentID("child-agent"),
			Status:  "completed",
			Output:  "child done",
		},
	}
	executor := NewAgentExecutor(runtime, &fakeHandoffRouter{
		profile: agent.AgentProfile{ID: agent.AgentID("child-agent")},
	})

	_, err := executor.Execute(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID:  "run-1",
			TaskID: "task-1",
			Goal:   "parent goal",
			Context: map[string]any{
				"trace_id": "trace-1",
			},
		},
		Agent: agent.AgentProfile{
			ID: agent.AgentID("parent-agent"),
			Collaboration: agent.CollaborationConfig{
				MaxDepth: 2,
			},
		},
	}, agent.PlanStep{
		StepID:   "delegate-1",
		Type:     agent.StepTypeAgent,
		Action:   "summarize",
		Expected: "summarize result",
		Params:   map[string]any{"topic": "wukong"},
		Context:  map[string]any{"local": true},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	req := runtime.req
	if req.RunID != "run-1:delegate-1" || req.ParentRunID != "run-1" {
		t.Fatalf("child RunID = %q ParentRunID = %q, want handoff child", req.RunID, req.ParentRunID)
	}
	if req.AgentID != agent.AgentID("child-agent") || req.Action != "summarize" || req.Goal != "summarize result" {
		t.Fatalf("child request = %#v, want routed summarize request", req)
	}
	if req.Params["topic"] != "wukong" {
		t.Fatalf("child params = %#v, want topic", req.Params)
	}
	if req.Context["trace_id"] != "trace-1" || req.Context["local"] != true || req.Context[handoffDepthContextKey] != 1 {
		t.Fatalf("child context = %#v, want merged context with depth 1", req.Context)
	}
	if !req.Constraints.AllowDelegate {
		t.Fatal("child AllowDelegate = false, want true below max depth")
	}
}

func TestAgentExecutorConvertsChildRunResultToStepResult(t *testing.T) {
	executor := NewAgentExecutor(&fakeAgentRuntime{
		result: &agent.RunResult{
			RunID:   "run-1:delegate-1",
			AgentID: agent.AgentID("child-agent"),
			Status:  "completed",
			Output:  "child done",
			Result:  map[string]any{"ok": true},
		},
	}, &fakeHandoffRouter{
		profile: agent.AgentProfile{ID: agent.AgentID("child-agent")},
	})

	result, err := executor.Execute(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{RunID: "run-1", TaskID: "task-1"},
		Agent: agent.AgentProfile{
			ID:            agent.AgentID("parent-agent"),
			Collaboration: agent.CollaborationConfig{MaxDepth: 1},
		},
	}, agent.PlanStep{StepID: "delegate-1", Type: agent.StepTypeAgent})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != "completed" || result.Output != "child done" || result.Result["ok"] != true {
		t.Fatalf("StepResult = %#v, want child run result", result)
	}
	if result.AgentID != agent.AgentID("child-agent") {
		t.Fatalf("StepResult AgentID = %q, want child-agent", result.AgentID)
	}
	if result.Metadata["handoff_id"] != "run-1:delegate-1" || result.Metadata["child_run_id"] != "run-1:delegate-1" {
		t.Fatalf("StepResult metadata = %#v, want handoff ids", result.Metadata)
	}
}

func TestAgentExecutorRejectsHandoffDepthLimit(t *testing.T) {
	runtime := &fakeAgentRuntime{}
	executor := NewAgentExecutor(runtime, &fakeHandoffRouter{
		profile: agent.AgentProfile{ID: agent.AgentID("child-agent")},
	})

	result, err := executor.Execute(context.Background(), agent.AgentContext{
		Request: agent.RunRequest{
			RunID: "run-1",
			Context: map[string]any{
				handoffDepthContextKey: 1,
			},
		},
		Agent: agent.AgentProfile{
			ID:            agent.AgentID("parent-agent"),
			Collaboration: agent.CollaborationConfig{MaxDepth: 1},
		},
	}, agent.PlanStep{StepID: "delegate-1", Type: agent.StepTypeAgent})
	if err == nil {
		t.Fatal("Execute() error = nil, want depth limit error")
	}
	if result == nil || result.Status != "failed" {
		t.Fatalf("StepResult = %#v, want failed result", result)
	}
	if runtime.called {
		t.Fatal("runtime was called after handoff depth exceeded")
	}
}

type fakeAgentRuntime struct {
	req    agent.RunRequest
	result *agent.RunResult
	called bool
}

func (r *fakeAgentRuntime) Run(_ context.Context, req agent.RunRequest) (*agent.RunResult, error) {
	r.req = req
	r.called = true
	return r.result, nil
}

type fakeHandoffRouter struct {
	profile agent.AgentProfile
}

func (r *fakeHandoffRouter) Route(context.Context, agent.RunRequest) (agent.AgentProfile, error) {
	return r.profile, nil
}

func (r *fakeHandoffRouter) RouteHandoff(context.Context, agent.AgentProfile, agent.PlanStep) (agent.AgentProfile, error) {
	return r.profile, nil
}
