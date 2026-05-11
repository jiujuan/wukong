package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/tool"
)

func TestParseReActReply(t *testing.T) {
	reply, err := parseReActReply("```json\n{\"thought\":\"plan\",\"action\":\"final\",\"final_answer\":\"done\"}\n```")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if reply.Action != "final" || reply.FinalAnswer != "done" {
		t.Fatalf("unexpected reply: %+v", reply)
	}
}

func TestBuildReActSystemPrompt(t *testing.T) {
	prompt := buildReActSystemPrompt("chat", []string{"web_search", "llm_chat"})
	if !strings.Contains(prompt, "chat") || !strings.Contains(prompt, "web_search") {
		t.Fatalf("prompt missing expected content: %s", prompt)
	}
}

func TestReActExecutorExecuteToolAndFinalAnswer(t *testing.T) {
	server, script := newLLMServerScript([]llm.ChatResponse{
		{
			Model: "test-model",
			Choices: []llm.Choice{{
				Message: llm.Message{Role: "assistant", Content: `{"thought":"search first","action":"tool","tool_name":"web_search","tool_params":{"query":"golang"}}`},
			}},
			Usage: llm.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		},
		{
			Model: "test-model",
			Choices: []llm.Choice{{
				Message: llm.Message{Role: "assistant", Content: `{"thought":"done","action":"final","final_answer":"golang is great"}`},
			}},
			Usage: llm.Usage{PromptTokens: 6, CompletionTokens: 2, TotalTokens: 8},
		},
	})
	defer server.Close()

	provider := newTestProvider(server.URL)
	manager := tool.NewManager()
	fake := &fakeTool{name: "web_search", result: map[string]any{"summary": "found"}}
	manager.Register(fake)
	registry := newTestRegistry()
	executor := NewReActExecutor(provider, manager, registry, nil)

	result, err := executor.Execute(context.Background(), &fakeSubTask{
		subTaskID: "sub-1",
		taskID:    "task-1",
		action:    "web_search",
		params:    map[string]any{"query": "golang"},
	})
	if err != nil {
		t.Fatalf("react execute failed: %v", err)
	}
	if result["output"] != "golang is great" {
		t.Fatalf("unexpected output: %#v", result)
	}
	steps, ok := result["react_steps"].([]ReActStep)
	if !ok || len(steps) != 2 {
		t.Fatalf("unexpected react steps: %#v", result["react_steps"])
	}
	if steps[0].Action != "tool" || steps[1].Action != "final" {
		t.Fatalf("unexpected step sequence: %#v", steps)
	}
	if fake.called != 1 {
		t.Fatalf("tool should be called once")
	}
	if len(script.requests) != 2 {
		t.Fatalf("expected two llm calls, got %d", len(script.requests))
	}
	secondRequest := script.requests[1]
	foundObservation := false
	for _, msg := range secondRequest.Messages {
		if strings.Contains(msg.Content, "Observation") {
			foundObservation = true
			break
		}
	}
	if !foundObservation {
		t.Fatalf("second request should contain observation: %#v", secondRequest.Messages)
	}
}

func TestReActExecutorPlainTextFallback(t *testing.T) {
	server, _ := newLLMServerScript([]llm.ChatResponse{{
		Model: "test-model",
		Choices: []llm.Choice{{
			Message: llm.Message{Role: "assistant", Content: "plain answer"},
		}},
	}})
	defer server.Close()

	provider := newTestProvider(server.URL)
	executor := NewReActExecutor(provider, nil, nil, nil)

	result, err := executor.Execute(context.Background(), &fakeSubTask{
		subTaskID: "sub-2",
		taskID:    "task-2",
		action:    "execute",
		params:    map[string]any{},
	})
	if err != nil {
		t.Fatalf("plain text fallback should succeed: %v", err)
	}
	if result["output"] != "plain answer" {
		t.Fatalf("unexpected fallback output: %#v", result)
	}
}

func TestReActExecutorNilProvider(t *testing.T) {
	executor := NewReActExecutor(nil, nil, nil, nil)
	_, err := executor.Execute(context.Background(), &fakeSubTask{})
	if err == nil {
		t.Fatalf("expected nil provider error")
	}
}

func TestReActReplyJSONRoundTrip(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"thought":      "x",
		"action":       "tool",
		"tool_name":    "web_search",
		"tool_params":  map[string]any{"query": "x"},
		"final_answer": "",
	})
	reply, err := parseReActReply(string(raw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if fmt.Sprint(reply.ToolParams["query"]) != "x" {
		t.Fatalf("unexpected reply params: %+v", reply)
	}
}
