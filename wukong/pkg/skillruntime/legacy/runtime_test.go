package legacy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jiujuan/wukong/pkg/skillruntime"
	"github.com/jiujuan/wukong/pkg/skills"
)

func TestRuntimeDiscoverUsesLegacyRegistryList(t *testing.T) {
	registry := &fakeRegistry{
		skills: []*skills.Skill{
			{
				SkillName:   "web_search",
				Description: "Search the web",
				Version:     "1.0.0",
				Enabled:     true,
				Tools:       []string{"web_search", "llm_chat"},
				Package: skills.PackageMeta{
					SourceType: skills.SourceBuiltin,
					Runtime:    "builtin",
				},
			},
		},
	}
	runtime := NewRuntime(registry)

	manifests, err := runtime.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(manifests) != 1 {
		t.Fatalf("Discover() returned %d manifests, want 1", len(manifests))
	}
	if manifests[0].Name != "web_search" || manifests[0].Runtime != RuntimeName {
		t.Fatalf("manifest = %#v", manifests[0])
	}
	if manifests[0].Metadata["source_type"] != string(skills.SourceBuiltin) {
		t.Fatalf("manifest metadata = %#v", manifests[0].Metadata)
	}
}

func TestRuntimeGetAndPrepareConvertLegacySkill(t *testing.T) {
	registry := &fakeRegistry{
		skills: []*skills.Skill{
			{
				SkillName:   "writer",
				Description: "Write reports",
				Version:     "1.2.0",
				Enabled:     true,
				Tools:       []string{"llm_chat", "file_write"},
				Template:    "Write a concise report.",
				References:  []string{"references/guide.md"},
				Assets:      []string{"assets/logo.png"},
				Package: skills.PackageMeta{
					SourceType: skills.SourceVendor,
					Runtime:    "python",
					Entry:      "scripts/write.py",
					RootDir:    "/skills/writer",
				},
			},
		},
	}
	runtime := NewRuntime(registry)

	spec, err := runtime.Get(context.Background(), "writer")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if spec.Instructions != "Write a concise report." {
		t.Fatalf("Instructions = %q", spec.Instructions)
	}
	if !reflect.DeepEqual(spec.AllowedTools, []string{"llm_chat", "file_write"}) {
		t.Fatalf("AllowedTools = %#v", spec.AllowedTools)
	}
	if spec.RootDir != "/skills/writer" {
		t.Fatalf("RootDir = %q", spec.RootDir)
	}

	prepared, err := runtime.Prepare(context.Background(), skillruntime.SkillActivation{SkillName: "writer"}, skillruntime.RuntimeContext{})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.ExecutionMode != skillruntime.SkillExecutionModeScript {
		t.Fatalf("ExecutionMode = %q", prepared.ExecutionMode)
	}
	if len(prepared.ContextBlocks) != 1 || prepared.ContextBlocks[0].Content != spec.Instructions {
		t.Fatalf("ContextBlocks = %#v", prepared.ContextBlocks)
	}
	if !reflect.DeepEqual(prepared.ToolPolicy.AllowedTools, spec.AllowedTools) {
		t.Fatalf("ToolPolicy = %#v, want %#v", prepared.ToolPolicy.AllowedTools, spec.AllowedTools)
	}
	if len(prepared.Resources) != 2 {
		t.Fatalf("Resources = %#v, want reference + asset", prepared.Resources)
	}
}

func TestRuntimeExecuteConvertsResultToSkillOutput(t *testing.T) {
	registry := &fakeRegistry{
		execResult: map[string]any{
			"output":    "done",
			"exit_code": 0,
		},
	}
	runtime := NewRuntime(registry)

	output, err := runtime.Execute(context.Background(), &skillruntime.PreparedSkill{
		Activation: skillruntime.SkillActivation{SkillName: "writer"},
	}, skillruntime.SkillInput{
		Params: map[string]any{"topic": "agents"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Status != "completed" || output.Output != "done" {
		t.Fatalf("SkillOutput = %#v", output)
	}
	if registry.execSkillName != "writer" {
		t.Fatalf("exec skill name = %q, want writer", registry.execSkillName)
	}
	if registry.execParams["topic"] != "agents" {
		t.Fatalf("exec params = %#v", registry.execParams)
	}
}

func TestRuntimeExecuteConvertsErrorToFailedOutput(t *testing.T) {
	execErr := errors.New("boom")
	registry := &fakeRegistry{
		execResult: map[string]any{"output": "partial"},
		execErr:    execErr,
	}
	runtime := NewRuntime(registry)

	output, err := runtime.Execute(context.Background(), &skillruntime.PreparedSkill{
		Spec: &skillruntime.SkillSpec{Manifest: skillruntime.SkillManifest{Name: "writer"}},
	}, skillruntime.SkillInput{})
	if !errors.Is(err, execErr) {
		t.Fatalf("Execute() error = %v, want execErr", err)
	}
	if output.Status != "failed" || output.Error != "boom" || output.Output != "partial" {
		t.Fatalf("SkillOutput = %#v", output)
	}
}

func TestRuntimeNilRegistry(t *testing.T) {
	runtime := NewRuntime(nil)

	_, err := runtime.Discover(context.Background())
	if !errors.Is(err, ErrRegistryNil) {
		t.Fatalf("Discover() error = %v, want ErrRegistryNil", err)
	}
}

type fakeRegistry struct {
	started bool
	skills  []*skills.Skill

	execSkillName string
	execParams    map[string]any
	execResult    map[string]any
	execErr       error
}

func (r *fakeRegistry) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.started = true
	return nil
}

func (r *fakeRegistry) Stop() {
	r.started = false
}

func (r *fakeRegistry) List() []*skills.Skill {
	return append([]*skills.Skill(nil), r.skills...)
}

func (r *fakeRegistry) Get(skillName string) (*skills.Skill, bool) {
	for _, item := range r.skills {
		if item.SkillName == skillName {
			return item, true
		}
	}
	return nil, false
}

func (r *fakeRegistry) ExecuteWithParams(ctx context.Context, skillName string, params map[string]any) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.execSkillName = skillName
	r.execParams = params
	return r.execResult, r.execErr
}
