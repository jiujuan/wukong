package legacy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jiujuan/wukong/pkg/skillruntime"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/skills/model"
)

const RuntimeName = "legacy"

var (
	// ErrRegistryNil is returned when the legacy runtime has no registry.
	ErrRegistryNil = errors.New("legacy skills registry is nil")
	// ErrSkillNotFound is returned when a legacy skill cannot be found.
	ErrSkillNotFound = errors.New("legacy skill not found")
)

var _ skillruntime.SkillRuntime = (*Runtime)(nil)

// Registry is the subset of pkg/skills.Registry consumed by this adapter.
type Registry interface {
	Start(ctx context.Context) error
	Stop()
	List() []*skills.Skill
	Get(skillName string) (*skills.Skill, bool)
	ExecuteWithParams(ctx context.Context, skillName string, params map[string]any) (map[string]any, error)
}

// Runtime adapts the existing Wukong skills registry to skillruntime.SkillRuntime.
type Runtime struct {
	registry Registry
}

// NewRuntime wraps an existing legacy skills registry.
func NewRuntime(registry Registry) *Runtime {
	return &Runtime{registry: registry}
}

// Name returns the runtime name.
func (r *Runtime) Name() string { return RuntimeName }

// Start delegates to the wrapped registry.
func (r *Runtime) Start(ctx context.Context) error {
	if r.registry == nil {
		return ErrRegistryNil
	}
	return r.registry.Start(ctx)
}

// Stop delegates to the wrapped registry.
func (r *Runtime) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.registry == nil {
		return ErrRegistryNil
	}
	r.registry.Stop()
	return nil
}

// Discover returns lightweight manifests for registered legacy skills.
func (r *Runtime) Discover(ctx context.Context) ([]skillruntime.SkillManifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.registry == nil {
		return nil, ErrRegistryNil
	}
	items := r.registry.List()
	manifests := make([]skillruntime.SkillManifest, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.SkillName) == "" {
			continue
		}
		manifests = append(manifests, skillToManifest(item))
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Name < manifests[j].Name
	})
	return manifests, nil
}

// Get returns a full SkillSpec converted from one legacy skill.
func (r *Runtime) Get(ctx context.Context, name string) (*skillruntime.SkillSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.registry == nil {
		return nil, ErrRegistryNil
	}
	item, ok := r.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, name)
	}
	return skillToSpec(item), nil
}

// Match matches legacy skills by explicit skill name, action, tags, goal, or query text.
func (r *Runtime) Match(ctx context.Context, req skillruntime.SkillMatchRequest) ([]skillruntime.SkillCandidate, error) {
	manifests, err := r.Discover(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]skillruntime.SkillCandidate, 0)
	for _, manifest := range manifests {
		if score, reason := matchManifest(manifest, req); score > 0 {
			candidates = append(candidates, skillruntime.SkillCandidate{
				Manifest: manifest,
				Score:    score,
				Reason:   reason,
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Manifest.Name < candidates[j].Manifest.Name
	})
	return candidates, nil
}

// Prepare returns a PreparedSkill for a legacy skill.
func (r *Runtime) Prepare(ctx context.Context, activation skillruntime.SkillActivation, runtimeCtx skillruntime.RuntimeContext) (*skillruntime.PreparedSkill, error) {
	spec, err := r.Get(ctx, activation.SkillName)
	if err != nil {
		return nil, err
	}
	if activation.RuntimeName == "" {
		activation.RuntimeName = RuntimeName
	}
	return &skillruntime.PreparedSkill{
		Spec:          spec,
		Activation:    activation,
		ContextBlocks: instructionBlocks(spec),
		ToolPolicy:    skillruntime.ToolPolicy{AllowedTools: append([]string(nil), spec.AllowedTools...)},
		ExecutionMode: skillruntime.SkillExecutionModeScript,
		WorkDir:       spec.RootDir,
		Resources:     append(append([]skillruntime.SkillResource{}, spec.References...), spec.Assets...),
		Metadata: map[string]any{
			"runtime_context": runtimeCtx,
		},
	}, nil
}

// Execute delegates execution to the existing skills registry and converts the result.
func (r *Runtime) Execute(ctx context.Context, prepared *skillruntime.PreparedSkill, input skillruntime.SkillInput) (*skillruntime.SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.registry == nil {
		return nil, ErrRegistryNil
	}
	skillName := preparedSkillName(prepared)
	if skillName == "" {
		return nil, fmt.Errorf("%w: empty prepared skill name", ErrSkillNotFound)
	}
	result, err := r.registry.ExecuteWithParams(ctx, skillName, input.Params)
	output := skillOutputFromResult(result, err)
	return output, err
}

func skillToManifest(item *skills.Skill) skillruntime.SkillManifest {
	return skillruntime.SkillManifest{
		Name:        strings.TrimSpace(item.SkillName),
		Description: strings.TrimSpace(item.Description),
		Runtime:     RuntimeName,
		Version:     strings.TrimSpace(item.Version),
		Metadata: map[string]string{
			"source_type": string(item.Package.SourceType),
			"runtime":     strings.TrimSpace(item.Package.Runtime),
			"entry":       strings.TrimSpace(item.Package.Entry),
		},
	}
}

func skillToSpec(item *skills.Skill) *skillruntime.SkillSpec {
	if item == nil {
		return nil
	}
	canon := item.Canonical()
	spec := &skillruntime.SkillSpec{
		Manifest:     skillToManifest(item),
		Instructions: strings.TrimSpace(canon.Instructions),
		AllowedTools: append([]string(nil), canon.AllowedTools...),
		RootDir:      strings.TrimSpace(canon.Source.RootDir),
		References:   resourcesFromCanonical(canon.References),
		Assets:       resourcesFromCanonical(canon.Assets),
		Metadata:     cloneAnyMap(canon.Metadata),
	}
	if spec.RootDir == "" {
		spec.RootDir = strings.TrimSpace(item.Package.RootDir)
	}
	return spec
}

func resourcesFromCanonical(items []model.SkillResource) []skillruntime.SkillResource {
	out := make([]skillruntime.SkillResource, 0, len(items))
	for _, item := range items {
		out = append(out, skillruntime.SkillResource{
			Kind:     item.Kind,
			Name:     item.Name,
			Path:     item.Path,
			MIMEType: item.MIMEType,
			Text:     item.Text,
			Metadata: cloneAnyMap(item.Metadata),
		})
	}
	return out
}

func instructionBlocks(spec *skillruntime.SkillSpec) []skillruntime.ContextBlock {
	if spec == nil || strings.TrimSpace(spec.Instructions) == "" {
		return nil
	}
	return []skillruntime.ContextBlock{
		{
			Name:     "legacy_skill_instructions",
			Type:     "skill_instructions",
			Source:   spec.Manifest.Name,
			Content:  spec.Instructions,
			Priority: 80,
			Metadata: map[string]any{
				"skill_name": spec.Manifest.Name,
				"runtime":    RuntimeName,
			},
		},
	}
}

func matchManifest(manifest skillruntime.SkillManifest, req skillruntime.SkillMatchRequest) (float64, string) {
	if req.SkillName != "" && strings.EqualFold(req.SkillName, manifest.Name) {
		return 1.0, "skill name match"
	}
	if req.Action != "" && strings.EqualFold(req.Action, manifest.Name) {
		return 0.9, "action match"
	}
	haystack := strings.ToLower(manifest.Name + " " + manifest.Description)
	for _, text := range []string{req.Goal, req.Query, req.Action} {
		for _, token := range strings.Fields(strings.ToLower(text)) {
			if len(token) >= 3 && strings.Contains(haystack, token) {
				return 0.5, "text match"
			}
		}
	}
	return 0, ""
}

func preparedSkillName(prepared *skillruntime.PreparedSkill) string {
	if prepared == nil {
		return ""
	}
	if strings.TrimSpace(prepared.Activation.SkillName) != "" {
		return strings.TrimSpace(prepared.Activation.SkillName)
	}
	if prepared.Spec != nil {
		return strings.TrimSpace(prepared.Spec.Manifest.Name)
	}
	return ""
}

func skillOutputFromResult(result map[string]any, err error) *skillruntime.SkillOutput {
	output := &skillruntime.SkillOutput{
		Status: "completed",
		Result: cloneAnyMap(result),
	}
	if raw, ok := result["output"]; ok && raw != nil {
		output.Output = fmt.Sprint(raw)
	}
	if err != nil {
		output.Status = "failed"
		output.Error = err.Error()
	}
	return output
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
