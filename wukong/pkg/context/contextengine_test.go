package context

import (
	stdctx "context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeSource struct {
	name   string
	blocks []ContextBlock
	err    error
	calls  int
}

func (s *fakeSource) Name() string { return s.name }

func (s *fakeSource) Load(ctx stdctx.Context, req BuildRequest) ([]ContextBlock, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]ContextBlock(nil), s.blocks...), nil
}

type fakePolicy struct {
	name  string
	apply func([]ContextBlock) []ContextBlock
	err   error
	calls int
}

func (p *fakePolicy) Name() string { return p.name }

func (p *fakePolicy) Apply(ctx stdctx.Context, blocks []ContextBlock, req BuildRequest) ([]ContextBlock, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	if p.apply == nil {
		return append([]ContextBlock(nil), blocks...), nil
	}
	return p.apply(append([]ContextBlock(nil), blocks...)), nil
}

func TestEngineRegisterAndGetScene(t *testing.T) {
	engine := New()
	source := &fakeSource{name: "chat_history"}
	policy := &fakePolicy{name: "dedupe"}

	if err := engine.RegisterSource(source); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := engine.RegisterPolicy(policy); err != nil {
		t.Fatalf("RegisterPolicy() error = %v", err)
	}
	if err := engine.RegisterScene(SceneConfig{
		Name:     "chat",
		Sources:  []string{"chat_history"},
		Policies: []string{"dedupe"},
		Options:  map[string]any{"window": 5},
	}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}

	scene, ok := engine.GetScene("chat")
	if !ok {
		t.Fatal("GetScene() ok = false, want true")
	}
	if scene.Name != "chat" {
		t.Fatalf("scene.Name = %q, want chat", scene.Name)
	}
	if !reflect.DeepEqual(scene.Sources, []string{"chat_history"}) {
		t.Fatalf("scene.Sources = %#v", scene.Sources)
	}
	scene.Sources[0] = "changed"
	again, _ := engine.GetScene("chat")
	if again.Sources[0] != "chat_history" {
		t.Fatal("GetScene() should return cloned scene")
	}
}

func TestEngineRegisterDuplicate(t *testing.T) {
	engine := New()
	if err := engine.RegisterSource(&fakeSource{name: "chat_history"}); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := engine.RegisterSource(&fakeSource{name: "chat_history"}); err == nil {
		t.Fatal("duplicate RegisterSource() error = nil")
	}

	if err := engine.RegisterPolicy(&fakePolicy{name: "dedupe"}); err != nil {
		t.Fatalf("RegisterPolicy() error = %v", err)
	}
	if err := engine.RegisterPolicy(&fakePolicy{name: "dedupe"}); err == nil {
		t.Fatal("duplicate RegisterPolicy() error = nil")
	}

	if err := engine.RegisterScene(SceneConfig{Name: "chat"}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	if err := engine.RegisterScene(SceneConfig{Name: "chat"}); err == nil {
		t.Fatal("duplicate RegisterScene() error = nil")
	}
}

func TestEngineBuildPipeline(t *testing.T) {
	engine := New()
	sourceA := &fakeSource{
		name: "memory",
		blocks: []ContextBlock{
			{Name: "memory_summary", Type: "memory", Content: "remember user preference", Priority: 90},
			{Name: "empty", Type: "memory", Content: "   ", Priority: 100},
		},
	}
	sourceB := &fakeSource{
		name: "history",
		blocks: []ContextBlock{
			{Name: "recent_messages", Type: "history", Content: "user: hi", Priority: 50, Timestamp: 2},
			{Name: "rules", Type: "static", Source: "manual", Content: "answer in Chinese", Priority: 70, Timestamp: 1},
		},
	}
	policy := &fakePolicy{
		name: "promote_rules",
		apply: func(blocks []ContextBlock) []ContextBlock {
			for i := range blocks {
				if blocks[i].Name == "rules" {
					blocks[i].Priority = 120
				}
			}
			return blocks
		},
	}

	if err := engine.RegisterSource(sourceA); err != nil {
		t.Fatalf("RegisterSource(memory) error = %v", err)
	}
	if err := engine.RegisterSource(sourceB); err != nil {
		t.Fatalf("RegisterSource(history) error = %v", err)
	}
	if err := engine.RegisterPolicy(policy); err != nil {
		t.Fatalf("RegisterPolicy() error = %v", err)
	}
	if err := engine.RegisterScene(SceneConfig{
		Name:     "chat",
		Sources:  []string{"memory", "history"},
		Policies: []string{"promote_rules"},
	}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}

	bundle, err := engine.Build(stdctx.Background(), BuildRequest{Scene: "chat"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if sourceA.calls != 1 || sourceB.calls != 1 || policy.calls != 1 {
		t.Fatalf("unexpected call counts: sourceA=%d sourceB=%d policy=%d", sourceA.calls, sourceB.calls, policy.calls)
	}
	if bundle.Scene != "chat" {
		t.Fatalf("bundle.Scene = %q, want chat", bundle.Scene)
	}
	if len(bundle.Blocks) != 3 {
		t.Fatalf("len(bundle.Blocks) = %d, want 3", len(bundle.Blocks))
	}
	if bundle.Blocks[0].Name != "rules" {
		t.Fatalf("first block should be rules after policy sort, got %#v", bundle.Blocks[0])
	}
	if bundle.Blocks[0].Source != "manual" {
		t.Fatalf("explicit source should be preserved, got %q", bundle.Blocks[0].Source)
	}
	if bundle.Blocks[1].Source != "memory" {
		t.Fatalf("default source should be injected, got %q", bundle.Blocks[1].Source)
	}
	if bundle.Named["memory_summary"] != "remember user preference" {
		t.Fatalf("bundle.Named memory_summary = %q", bundle.Named["memory_summary"])
	}
	if bundle.Meta["block_count"] != 3 {
		t.Fatalf("bundle.Meta block_count = %#v", bundle.Meta["block_count"])
	}
	if !strings.Contains(bundle.Text, "[rules]") || !strings.Contains(bundle.Text, "answer in Chinese") {
		t.Fatalf("bundle.Text = %q", bundle.Text)
	}
}

func TestEngineBuildSourceOrPolicyError(t *testing.T) {
	engine := New()
	sourceErr := errors.New("source failed")
	policyErr := errors.New("policy failed")

	if err := engine.RegisterSource(&fakeSource{name: "broken", err: sourceErr}); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := engine.RegisterScene(SceneConfig{Name: "chat", Sources: []string{"broken"}}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	if _, err := engine.Build(stdctx.Background(), BuildRequest{Scene: "chat"}); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("Build() source error = %v, want source failure", err)
	}

	engine2 := New()
	if err := engine2.RegisterSource(&fakeSource{name: "ok", blocks: []ContextBlock{{Name: "a", Content: "x"}}}); err != nil {
		t.Fatalf("RegisterSource() error = %v", err)
	}
	if err := engine2.RegisterPolicy(&fakePolicy{name: "broken_policy", err: policyErr}); err != nil {
		t.Fatalf("RegisterPolicy() error = %v", err)
	}
	if err := engine2.RegisterScene(SceneConfig{Name: "chat", Sources: []string{"ok"}, Policies: []string{"broken_policy"}}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	if _, err := engine2.Build(stdctx.Background(), BuildRequest{Scene: "chat"}); err == nil || !strings.Contains(err.Error(), "policy") {
		t.Fatalf("Build() policy error = %v, want policy failure", err)
	}
}

func TestEngineBuildMissingSceneOrDependency(t *testing.T) {
	engine := New()
	if _, err := engine.Build(stdctx.Background(), BuildRequest{Scene: "missing"}); err == nil {
		t.Fatal("Build() missing scene error = nil")
	}

	if err := engine.RegisterScene(SceneConfig{Name: "chat", Sources: []string{"missing_source"}}); err != nil {
		t.Fatalf("RegisterScene() error = %v", err)
	}
	if _, err := engine.Build(stdctx.Background(), BuildRequest{Scene: "chat"}); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("Build() missing source error = %v", err)
	}
}
