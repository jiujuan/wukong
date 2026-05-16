package agent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiujuan/wukong/pkg/agent"
	"github.com/jiujuan/wukong/pkg/agent/action"
	"github.com/jiujuan/wukong/pkg/skillruntime"
	"github.com/jiujuan/wukong/pkg/skillruntime/agentspec"
)

func TestPDFSkillAgentLoopSkillRuntimeIntegration(t *testing.T) {
	ctx := context.Background()
	pdfRoot := findPDFSkillRoot(t)

	pdfRuntime := agentspec.NewRuntime(agentspec.WithRoots(pdfRoot))
	manifests, err := pdfRuntime.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("Discover() returned %d manifests, want 1", len(manifests))
	}
	manifest := manifests[0]
	if manifest.Name != "pdf" || manifest.Runtime != agentspec.RuntimeName {
		t.Fatalf("manifest = %#v, want pdf agentspec manifest", manifest)
	}
	if !strings.Contains(strings.ToLower(manifest.Description), "pdf files") {
		t.Fatalf("manifest description = %q, want real pdf description", manifest.Description)
	}
	if !strings.Contains(strings.ToLower(manifest.License), "proprietary") {
		t.Fatalf("manifest license = %q, want proprietary license", manifest.License)
	}

	spec, err := pdfRuntime.Get(ctx, "pdf")
	if err != nil {
		t.Fatalf("Get(pdf) error = %v", err)
	}
	if spec.Manifest.Name != "pdf" || spec.RootDir != filepath.Clean(pdfRoot) {
		t.Fatalf("spec manifest/root = %#v/%q, want pdf root", spec.Manifest, spec.RootDir)
	}
	if !strings.Contains(spec.Instructions, "PDF Processing Guide") {
		t.Fatal("spec instructions do not include PDF Processing Guide")
	}
	assertResourcePath(t, spec.Scripts, "scripts/extract_form_structure.py")
	assertResourcePath(t, spec.Scripts, "scripts/fill_pdf_form_with_annotations.py")
	assertResourcePath(t, spec.References, "forms.md")
	assertResourcePath(t, spec.References, "reference.md")

	prepared, err := pdfRuntime.Prepare(ctx, skillruntime.SkillActivation{
		SkillName:   "pdf",
		RuntimeName: agentspec.RuntimeName,
		RequestedBy: "agent-pdf",
	}, skillruntime.RuntimeContext{
		Caller: skillruntime.Caller{Type: skillruntime.CallerTypeAgent, ID: "agent-pdf"},
		TaskID: "task-pdf",
		Goal:   "extract tables from a PDF",
		Params: map[string]any{"file": "sample.pdf"},
	})
	if err != nil {
		t.Fatalf("Prepare(pdf) error = %v", err)
	}
	if prepared.ExecutionMode != skillruntime.SkillExecutionModeContextOnly {
		t.Fatalf("ExecutionMode = %q, want context_only", prepared.ExecutionMode)
	}
	if prepared.WorkDir != filepath.Clean(pdfRoot) {
		t.Fatalf("WorkDir = %q, want pdf root", prepared.WorkDir)
	}
	if len(prepared.ContextBlocks) != 1 || !strings.Contains(prepared.ContextBlocks[0].Content, "PDF Processing Guide") {
		t.Fatalf("ContextBlocks = %#v, want pdf instructions block", prepared.ContextBlocks)
	}
	assertResourcePath(t, prepared.Resources, "forms.md")
	assertResourcePath(t, prepared.Resources, "scripts/check_fillable_fields.py")

	registry := skillruntime.NewRegistry()
	spyRuntime := &capturingSkillRuntime{SkillRuntime: pdfRuntime}
	if err := registry.Register(spyRuntime, 0); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Refresh(ctx); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	resolved, ok := registry.Resolve("pdf")
	if !ok || resolved == nil {
		t.Fatal("Resolve(pdf) did not return registered runtime")
	}
	candidates, err := registry.Match(ctx, skillruntime.SkillMatchRequest{Query: "extract tables from a pdf file"})
	if err != nil {
		t.Fatalf("Match() error = %v", err)
	}
	if len(candidates) == 0 || candidates[0].Manifest.Name != "pdf" {
		t.Fatalf("Match() = %#v, want pdf candidate", candidates)
	}

	loop := agent.NewAgentLoop(agent.AgentProfile{ID: agent.AgentID("agent-pdf"), Role: agent.AgentRoleTool},
		agent.WithAgentLoopController(agent.NewDefaultLoopController(agent.WithMaxRetries(0))),
		agent.WithAgentLoopActionRunner(agent.NewSequentialActionRunner(action.NewSkillRuntimeExecutor(registry, nil))),
	)
	result, err := loop.Run(ctx, agent.RunRequest{
		RunID:     "run-pdf",
		TaskID:    "task-pdf",
		SkillName: "pdf",
		Goal:      "extract tables from a PDF",
		Params:    map[string]any{"file": "sample.pdf"},
	})
	if !errors.Is(err, agentspec.ErrExecuteUnsupported) {
		t.Fatalf("Run() error = %v, want ErrExecuteUnsupported after successful prepare", err)
	}
	if result == nil || result.Status != string(agent.LoopStatusFailed) {
		t.Fatalf("RunResult = %#v, want failed result", result)
	}
	if len(result.Steps) != 1 || result.Steps[0].SkillName != "pdf" || result.Steps[0].Status != "failed" {
		t.Fatalf("RunResult Steps = %#v, want one failed pdf skill step", result.Steps)
	}
	if spyRuntime.prepared == nil || spyRuntime.prepared.Spec.Manifest.Name != "pdf" {
		t.Fatalf("captured prepared = %#v, want prepared pdf skill", spyRuntime.prepared)
	}
	if spyRuntime.runtimeCtx.Caller.Type != skillruntime.CallerTypeAgent || spyRuntime.runtimeCtx.Caller.ID != "agent-pdf" {
		t.Fatalf("runtime context caller = %#v, want agent-pdf caller", spyRuntime.runtimeCtx.Caller)
	}
	if spyRuntime.runtimeCtx.Params["file"] != "sample.pdf" || spyRuntime.executedInput.Params["file"] != "sample.pdf" {
		t.Fatalf("runtime params = %#v input params = %#v, want sample.pdf", spyRuntime.runtimeCtx.Params, spyRuntime.executedInput.Params)
	}
}

func TestPDFSkillAgentLoopRunsToCompletionWithExecutableRuntime(t *testing.T) {
	ctx := context.Background()
	pdfRuntime := agentspec.NewRuntime(agentspec.WithRoots(findPDFSkillRoot(t)))
	runtime := &successfulPDFSkillRuntime{SkillRuntime: pdfRuntime}

	registry := skillruntime.NewRegistry()
	if err := registry.Register(runtime, 0); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Refresh(ctx); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	loop := agent.NewAgentLoop(agent.AgentProfile{ID: agent.AgentID("agent-pdf"), Role: agent.AgentRoleTool},
		agent.WithAgentLoopActionRunner(agent.NewSequentialActionRunner(action.NewSkillRuntimeExecutor(registry, nil))),
	)
	result, err := loop.Run(ctx, agent.RunRequest{
		RunID:     "run-pdf-success",
		TaskID:    "task-pdf",
		SkillName: "pdf",
		Goal:      "summarize a PDF",
		Params:    map[string]any{"file": "sample.pdf", "operation": "summarize"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != string(agent.LoopStatusCompleted) {
		t.Fatalf("RunResult Status = %q, want completed", result.Status)
	}
	if result.Output != "pdf skill completed" {
		t.Fatalf("RunResult Output = %q, want pdf skill completed", result.Output)
	}
	if result.Result["skill"] != "pdf" || result.Result["file"] != "sample.pdf" || result.Result["resources_count"] == nil {
		t.Fatalf("RunResult Result = %#v, want pdf execution result", result.Result)
	}
	if result.Evaluation == nil || !result.Evaluation.Success {
		t.Fatalf("Evaluation = %#v, want successful evaluation", result.Evaluation)
	}
	if len(result.Steps) != 1 || result.Steps[0].Status != "completed" || result.Steps[0].SkillName != "pdf" {
		t.Fatalf("RunResult Steps = %#v, want one completed pdf skill step", result.Steps)
	}
	if runtime.prepared == nil || runtime.prepared.Spec.Manifest.Name != "pdf" {
		t.Fatalf("prepared = %#v, want prepared pdf spec", runtime.prepared)
	}
	if runtime.input.Params["file"] != "sample.pdf" || runtime.input.Context.Caller.ID != "agent-pdf" {
		t.Fatalf("input = %#v, want agent-pdf sample.pdf execution input", runtime.input)
	}
}

type capturingSkillRuntime struct {
	skillruntime.SkillRuntime
	runtimeCtx    skillruntime.RuntimeContext
	prepared      *skillruntime.PreparedSkill
	executedInput skillruntime.SkillInput
}

func (r *capturingSkillRuntime) Prepare(ctx context.Context, activation skillruntime.SkillActivation, runtimeCtx skillruntime.RuntimeContext) (*skillruntime.PreparedSkill, error) {
	r.runtimeCtx = runtimeCtx
	prepared, err := r.SkillRuntime.Prepare(ctx, activation, runtimeCtx)
	r.prepared = prepared
	return prepared, err
}

func (r *capturingSkillRuntime) Execute(ctx context.Context, prepared *skillruntime.PreparedSkill, input skillruntime.SkillInput) (*skillruntime.SkillOutput, error) {
	r.executedInput = input
	return r.SkillRuntime.Execute(ctx, prepared, input)
}

type successfulPDFSkillRuntime struct {
	skillruntime.SkillRuntime
	prepared *skillruntime.PreparedSkill
	input    skillruntime.SkillInput
}

func (r *successfulPDFSkillRuntime) Prepare(ctx context.Context, activation skillruntime.SkillActivation, runtimeCtx skillruntime.RuntimeContext) (*skillruntime.PreparedSkill, error) {
	prepared, err := r.SkillRuntime.Prepare(ctx, activation, runtimeCtx)
	r.prepared = prepared
	return prepared, err
}

func (r *successfulPDFSkillRuntime) Execute(ctx context.Context, prepared *skillruntime.PreparedSkill, input skillruntime.SkillInput) (*skillruntime.SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.input = input
	return &skillruntime.SkillOutput{
		Status: "completed",
		Output: "pdf skill completed",
		Result: map[string]any{
			"skill":           prepared.Spec.Manifest.Name,
			"file":            input.Params["file"],
			"operation":       input.Params["operation"],
			"resources_count": len(prepared.Resources),
		},
		Metadata: map[string]any{
			"execution_mode": string(prepared.ExecutionMode),
		},
	}, nil
}

func findPDFSkillRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		candidate := filepath.Join(dir, "skills", "vendor", "pdf")
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return filepath.Clean(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find skills/vendor/pdf/SKILL.md")
		}
		dir = parent
	}
}

func assertResourcePath(t *testing.T, resources []skillruntime.SkillResource, want string) {
	t.Helper()
	for _, resource := range resources {
		if resource.Path == want {
			return
		}
	}
	t.Fatalf("resources missing %q: %#v", want, resources)
}
