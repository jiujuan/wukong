package action

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/skillruntime"
)

func TestSkillRuntimeExecutorCallsRuntimeLifecycle(t *testing.T) {
	runtime := &fakeSkillRuntime{name: "fake-runtime"}
	registry := &fakeSkillRegistry{runtime: runtime}
	executor := NewSkillRuntimeExecutor(registry, nil)
	agentCtx := agent.AgentContext{
		Request: agent.RunRequest{
			RunID:     "run-1",
			TaskID:    "task-1",
			SessionID: "session-1",
			Goal:      "write report",
			Params:    map[string]any{"topic": "old"},
		},
		Agent: agent.AgentProfile{ID: "agent-1", Role: agent.AgentRoleTool},
	}
	step := agent.PlanStep{
		StepID:    "step-1",
		Type:      agent.StepTypeSkill,
		SkillName: "writer",
		Thought:   "Use writer skill",
		Params:    map[string]any{"topic": "wukong"},
		Expected:  "short report",
	}

	result, err := executor.Execute(context.Background(), agentCtx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if registry.resolved != "writer" {
		t.Fatalf("resolved skill = %q, want writer", registry.resolved)
	}
	if runtime.prepared == nil || runtime.prepared.SkillName != "writer" || runtime.prepared.RuntimeName != "fake-runtime" {
		t.Fatalf("prepared activation = %#v, want writer fake-runtime", runtime.prepared)
	}
	if runtime.executed == nil || runtime.executed.Params["topic"] != "wukong" {
		t.Fatalf("executed input = %#v, want patched topic", runtime.executed)
	}
	if result.Status != "completed" || result.Output != "skill done" {
		t.Fatalf("StepResult = %#v, want completed skill output", result)
	}
}

func TestDefaultSkillRuntimeContextMapperDoesNotLeakAgentContext(t *testing.T) {
	mapper := DefaultSkillRuntimeContextMapper{}
	runtimeCtx := mapper.Map(agent.AgentContext{
		Request: agent.RunRequest{
			RunID:     "run-1",
			TaskID:    "task-1",
			SessionID: "session-1",
			Goal:      "goal",
			Params:    map[string]any{"request": "value"},
		},
		Agent:        agent.AgentProfile{ID: "agent-1", Role: agent.AgentRoleTool},
		SharedMemory: map[string]any{"shared": "memory"},
	}, agent.PlanStep{
		StepID:    "step-1",
		SkillName: "writer",
		Params:    map[string]any{"step": "value"},
	})

	if runtimeCtx.Caller.Type != skillruntime.CallerTypeAgent || runtimeCtx.Caller.ID != "agent-1" {
		t.Fatalf("Caller = %#v, want agent caller", runtimeCtx.Caller)
	}
	if runtimeCtx.TaskID != "task-1" || runtimeCtx.SessionID != "session-1" || runtimeCtx.Goal != "goal" {
		t.Fatalf("RuntimeContext = %#v, want request fields", runtimeCtx)
	}
	if runtimeCtx.Params["request"] != "value" || runtimeCtx.Params["step"] != "value" {
		t.Fatalf("Params = %#v, want request and step params", runtimeCtx.Params)
	}
	if _, ok := runtimeCtx.Metadata["agent_context"]; ok {
		t.Fatalf("Metadata leaks agent context: %#v", runtimeCtx.Metadata)
	}
}

func TestSkillRuntimeExecutorConvertsSkillOutputToStepResult(t *testing.T) {
	runtime := &fakeSkillRuntime{
		output: &skillruntime.SkillOutput{
			Status: "completed",
			Output: "generated text",
			Result: map[string]any{"path": "out.md"},
			Metadata: map[string]any{
				"mode": "script",
			},
			Artifacts: []skillruntime.SkillResource{{Name: "out.md"}},
		},
	}
	executor := NewSkillRuntimeExecutor(&fakeSkillRegistry{runtime: runtime}, nil)

	result, err := executor.Execute(context.Background(), agent.AgentContext{}, agent.PlanStep{
		StepID:    "step-1",
		Type:      agent.StepTypeSkill,
		SkillName: "writer",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Output != "generated text" || result.Result["path"] != "out.md" {
		t.Fatalf("StepResult = %#v, want converted output and result", result)
	}
	if result.Metadata["mode"] != "script" || result.Metadata["artifacts_count"] != 1 {
		t.Fatalf("Metadata = %#v, want mode and artifacts count", result.Metadata)
	}
}

func TestSkillRuntimePackageDoesNotImportAgent(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "skillruntime"))
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			if strings.Trim(spec.Path.Value, "\"") == "github.com/jiujuan/wukong/pkg/agent" {
				t.Fatalf("%s imports pkg/agent", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk skillruntime imports: %v", err)
	}
}

type fakeSkillRegistry struct {
	runtime  skillruntime.SkillRuntime
	resolved string
}

func (r *fakeSkillRegistry) Resolve(skillName string) (skillruntime.SkillRuntime, bool) {
	r.resolved = skillName
	return r.runtime, r.runtime != nil
}

type fakeSkillRuntime struct {
	name     string
	prepared *skillruntime.SkillActivation
	executed *skillruntime.SkillInput
	output   *skillruntime.SkillOutput
}

func (r *fakeSkillRuntime) Name() string {
	if r.name == "" {
		return "fake"
	}
	return r.name
}

func (r *fakeSkillRuntime) Start(context.Context) error { return nil }

func (r *fakeSkillRuntime) Stop(context.Context) error { return nil }

func (r *fakeSkillRuntime) Discover(context.Context) ([]skillruntime.SkillManifest, error) {
	return nil, nil
}

func (r *fakeSkillRuntime) Get(context.Context, string) (*skillruntime.SkillSpec, error) {
	return nil, nil
}

func (r *fakeSkillRuntime) Match(context.Context, skillruntime.SkillMatchRequest) ([]skillruntime.SkillCandidate, error) {
	return nil, nil
}

func (r *fakeSkillRuntime) Prepare(_ context.Context, activation skillruntime.SkillActivation, runtimeCtx skillruntime.RuntimeContext) (*skillruntime.PreparedSkill, error) {
	r.prepared = &activation
	return &skillruntime.PreparedSkill{
		Activation: activation,
		Metadata: map[string]any{
			"task_id": runtimeCtx.TaskID,
		},
	}, nil
}

func (r *fakeSkillRuntime) Execute(_ context.Context, _ *skillruntime.PreparedSkill, input skillruntime.SkillInput) (*skillruntime.SkillOutput, error) {
	r.executed = &input
	if r.output != nil {
		return r.output, nil
	}
	return &skillruntime.SkillOutput{
		Status: "completed",
		Output: "skill done",
		Result: map[string]any{"ok": true},
	}, nil
}
