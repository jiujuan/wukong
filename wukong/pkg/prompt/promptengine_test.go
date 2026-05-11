package prompt

import (
	"errors"
	"reflect"
	"testing"
)

func TestEngineRegisterAndGet(t *testing.T) {
	engine := New()
	tpl := &Template{
		Key:         "chat.session.default",
		Description: "chat prompt",
		Version:     "v1",
		Messages: []MessageTemplate{
			{Role: "system", Content: "system {{query}}"},
			{Role: "user", Content: "user {{query}}"},
		},
	}

	if err := engine.Register(tpl); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := engine.Get("chat.session.default")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if got.Key != tpl.Key || got.Description != tpl.Description || got.Version != tpl.Version {
		t.Fatalf("Get() template mismatch = %#v", got)
	}
	if !reflect.DeepEqual(got.Messages, tpl.Messages) {
		t.Fatalf("Get() messages mismatch = %#v", got.Messages)
	}

	got.Messages[0].Content = "changed"
	again, _ := engine.Get("chat.session.default")
	if again.Messages[0].Content != tpl.Messages[0].Content {
		t.Fatal("Get() should return cloned template")
	}
}

func TestEngineRegisterDuplicate(t *testing.T) {
	engine := New()
	tpl := &Template{
		Key: "worker.action.default",
		Messages: []MessageTemplate{
			{Role: "system", Content: "system"},
		},
	}

	if err := engine.Register(tpl); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := engine.Register(tpl); err == nil {
		t.Fatal("second Register() error = nil, want duplicate error")
	}
}

func TestEngineRenderSuccess(t *testing.T) {
	engine := New()
	if err := engine.Register(&Template{
		Key: "planner.task.default",
		Messages: []MessageTemplate{
			{Role: "system", Content: "scene {{scene}}"},
			{Role: "user", Content: "query={{query}}\nctx={{context.summary}}"},
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	msgs, err := engine.Render("planner.task.default", RenderInput{
		Variables: map[string]any{
			"scene": "planner",
			"query": "build task dag",
		},
		Context: map[string]any{
			"summary": "recent task context",
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "scene planner" {
		t.Fatalf("msgs[0] = %#v", msgs[0])
	}
	wantUser := "query=build task dag\nctx=recent task context"
	if msgs[1].Role != "user" || msgs[1].Content != wantUser {
		t.Fatalf("msgs[1] = %#v, want content %q", msgs[1], wantUser)
	}
}

func TestEngineRenderMissingVariables(t *testing.T) {
	engine := New()
	if err := engine.Register(&Template{
		Key: "worker.react.default",
		Messages: []MessageTemplate{
			{Role: "system", Content: "skill={{skill_name}}"},
			{Role: "user", Content: "query={{query}}"},
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := engine.Render("worker.react.default", RenderInput{
		Variables: map[string]any{
			"query": "search docs",
		},
	})
	if err == nil {
		t.Fatal("Render() error = nil, want missing variables error")
	}

	var missingErr *MissingVariablesError
	if !errors.As(err, &missingErr) {
		t.Fatalf("Render() error type = %T, want *MissingVariablesError", err)
	}
	if missingErr.TemplateKey != "worker.react.default" {
		t.Fatalf("TemplateKey = %q, want worker.react.default", missingErr.TemplateKey)
	}
	if !reflect.DeepEqual(missingErr.Keys, []string{"skill_name"}) {
		t.Fatalf("Keys = %#v, want [skill_name]", missingErr.Keys)
	}
}

func TestEngineRenderTemplateNotFound(t *testing.T) {
	engine := New()
	if _, err := engine.Render("missing.template", RenderInput{}); err == nil {
		t.Fatal("Render() error = nil, want not found error")
	}
}

func TestNewDefaultEngineRegistersBuiltins(t *testing.T) {
	engine := NewDefaultEngine()
	keys := []string{
		TemplateWorkerActionDefault,
		TemplateWorkerActionSearch,
		TemplateWorkerActionReport,
		TemplateWorkerReactDefault,
		TemplatePlannerTaskDefault,
		TemplateChatSessionDefault,
	}
	for _, key := range keys {
		if _, ok := engine.Get(key); !ok {
			t.Fatalf("builtin template %q not registered", key)
		}
	}
}
