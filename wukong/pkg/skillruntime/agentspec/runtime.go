package agentspec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jiujuan/wukong/pkg/skillruntime"
)

var (
	// ErrSkillNotFound is returned when a skill name is not known to the runtime.
	ErrSkillNotFound = errors.New("agentspec skill not found")
	// ErrExecuteUnsupported is returned until script execution is wired in a later task.
	ErrExecuteUnsupported = errors.New("agentspec execute unsupported")
)

var _ skillruntime.SkillRuntime = (*Runtime)(nil)

// Runtime discovers and prepares Agent Skills Spec skill directories.
type Runtime struct {
	mu             sync.RWMutex
	roots          []string
	parser         Parser
	toolMapper     skillruntime.ToolPolicyMapper
	manifestByName map[string]skillruntime.SkillManifest
	pathByName     map[string]string
}

// Option configures an Agent Skills Spec runtime.
type Option func(*Runtime)

// WithRoots configures root directories to scan.
func WithRoots(roots ...string) Option {
	return func(r *Runtime) {
		r.roots = cleanRoots(roots)
	}
}

// WithToolPolicyMapper configures the allowed-tools mapper used by Prepare.
func WithToolPolicyMapper(mapper skillruntime.ToolPolicyMapper) Option {
	return func(r *Runtime) {
		if mapper != nil {
			r.toolMapper = mapper
		}
	}
}

// NewRuntime creates an Agent Skills Spec runtime.
func NewRuntime(options ...Option) *Runtime {
	r := &Runtime{
		parser:         NewParser(),
		toolMapper:     skillruntime.NewDefaultToolPolicyMapper(),
		manifestByName: make(map[string]skillruntime.SkillManifest),
		pathByName:     make(map[string]string),
	}
	for _, option := range options {
		if option != nil {
			option(r)
		}
	}
	return r
}

// Name returns the runtime name.
func (r *Runtime) Name() string { return RuntimeName }

// Start performs an initial discovery pass.
func (r *Runtime) Start(ctx context.Context) error {
	_, err := r.Discover(ctx)
	return err
}

// Stop currently has no background resources to release.
func (r *Runtime) Stop(ctx context.Context) error {
	return ctx.Err()
}

// Discover scans roots for SKILL.md files and caches lightweight manifests.
func (r *Runtime) Discover(ctx context.Context) ([]skillruntime.SkillManifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	manifestByName := make(map[string]skillruntime.SkillManifest)
	pathByName := make(map[string]string)
	for _, root := range r.snapshotRoots() {
		if err := walkSkillFiles(ctx, root, func(path string) error {
			manifest, err := r.parser.ParseManifestFile(path)
			if err != nil {
				return err
			}
			manifest.Runtime = RuntimeName
			manifestByName[manifest.Name] = manifest
			pathByName[manifest.Name] = path
			return nil
		}); err != nil {
			return nil, err
		}
	}

	manifests := sortedManifestValues(manifestByName)
	r.mu.Lock()
	r.manifestByName = manifestByName
	r.pathByName = pathByName
	r.mu.Unlock()
	return manifests, nil
}

// Get loads the complete SkillSpec for one skill, including instructions and resource indexes.
func (r *Runtime) Get(ctx context.Context, name string) (*skillruntime.SkillSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, ok := r.skillPath(name)
	if !ok {
		if _, err := r.Discover(ctx); err != nil {
			return nil, err
		}
		path, ok = r.skillPath(name)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, name)
		}
	}

	spec, err := r.parser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	spec.RootDir = filepath.Clean(filepath.Dir(path))
	spec.Scripts, spec.References, spec.Assets = scanResourceIndex(spec.RootDir)
	return spec, nil
}

// Match returns skill candidates that match explicit skill name, action, tags, or text fields.
func (r *Runtime) Match(ctx context.Context, req skillruntime.SkillMatchRequest) ([]skillruntime.SkillCandidate, error) {
	manifests, err := r.Discover(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]skillruntime.SkillCandidate, 0)
	for _, manifest := range manifests {
		if score, reason := matchManifest(manifest, req); score > 0 {
			out = append(out, skillruntime.SkillCandidate{
				Manifest: manifest,
				Score:    score,
				Reason:   reason,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Manifest.Name < out[j].Manifest.Name
	})
	return out, nil
}

// Prepare builds the context and policy needed by Agent Loop before planning or acting.
func (r *Runtime) Prepare(ctx context.Context, activation skillruntime.SkillActivation, runtimeCtx skillruntime.RuntimeContext) (*skillruntime.PreparedSkill, error) {
	spec, err := r.Get(ctx, activation.SkillName)
	if err != nil {
		return nil, err
	}
	policy, err := r.toolMapper.MapAllowedTools(ctx, spec.AllowedTools)
	if err != nil {
		return nil, err
	}
	if activation.RuntimeName == "" {
		activation.RuntimeName = RuntimeName
	}

	resources := make([]skillruntime.SkillResource, 0, len(spec.Scripts)+len(spec.References)+len(spec.Assets))
	resources = append(resources, spec.Scripts...)
	resources = append(resources, spec.References...)
	resources = append(resources, spec.Assets...)

	prepared := &skillruntime.PreparedSkill{
		Spec:          spec,
		Activation:    activation,
		ContextBlocks: instructionBlocks(spec),
		ToolPolicy:    policy,
		ExecutionMode: skillruntime.SkillExecutionModeContextOnly,
		WorkDir:       spec.RootDir,
		Resources:     resources,
		Metadata: map[string]any{
			"runtime_context": runtimeCtx,
		},
	}
	return prepared, nil
}

// Execute is intentionally disabled until M2.7 wires controlled script runners.
func (r *Runtime) Execute(ctx context.Context, prepared *skillruntime.PreparedSkill, input skillruntime.SkillInput) (*skillruntime.SkillOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrExecuteUnsupported
}

func (r *Runtime) snapshotRoots() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roots := make([]string, len(r.roots))
	copy(roots, r.roots)
	return roots
}

func (r *Runtime) skillPath(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path, ok := r.pathByName[name]
	return path, ok
}

func walkSkillFiles(ctx context.Context, root string, visit func(path string) error) error {
	if root == "" {
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), SkillFileName) {
			return visit(path)
		}
		return nil
	})
}

func instructionBlocks(spec *skillruntime.SkillSpec) []skillruntime.ContextBlock {
	if spec == nil || strings.TrimSpace(spec.Instructions) == "" {
		return nil
	}
	return []skillruntime.ContextBlock{
		{
			Name:     "skill_instructions",
			Type:     "skill_instructions",
			Source:   spec.Manifest.Name,
			Content:  spec.Instructions,
			Priority: 100,
			Metadata: map[string]any{
				"skill_name": spec.Manifest.Name,
				"runtime":    RuntimeName,
			},
		},
	}
}

func scanResourceIndex(rootDir string) (scripts, references, assets []skillruntime.SkillResource) {
	return scanResourceDir(rootDir, "scripts", "script"),
		scanReferenceResources(rootDir),
		scanResourceDir(rootDir, "assets", "asset")
}

func scanReferenceResources(rootDir string) []skillruntime.SkillResource {
	entries := scanResourceDir(rootDir, "references", "reference")
	rootEntries, err := os.ReadDir(rootDir)
	if err != nil {
		return entries
	}
	for _, entry := range rootEntries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, SkillFileName) || !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		entries = append(entries, skillruntime.SkillResource{
			Kind: "reference",
			Name: name,
			Path: filepath.ToSlash(name),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func scanResourceDir(rootDir, dirName, kind string) []skillruntime.SkillResource {
	dir := filepath.Join(rootDir, dirName)
	entries := make([]skillruntime.SkillResource, 0)
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			rel = path
		}
		entries = append(entries, skillruntime.SkillResource{
			Kind: kind,
			Name: entry.Name(),
			Path: filepath.ToSlash(rel),
		})
		return nil
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func matchManifest(manifest skillruntime.SkillManifest, req skillruntime.SkillMatchRequest) (float64, string) {
	if req.SkillName != "" && strings.EqualFold(req.SkillName, manifest.Name) {
		return 1.0, "skill name match"
	}
	if req.Action != "" && strings.EqualFold(req.Action, manifest.Name) {
		return 0.9, "action match"
	}
	for _, tag := range req.Tags {
		for _, manifestTag := range manifest.Tags {
			if strings.EqualFold(tag, manifestTag) {
				return 0.7, "tag match"
			}
		}
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

func sortedManifestValues(items map[string]skillruntime.SkillManifest) []skillruntime.SkillManifest {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]skillruntime.SkillManifest, 0, len(names))
	for _, name := range names {
		out = append(out, items[name])
	}
	return out
}

func cleanRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root != "" {
			out = append(out, filepath.Clean(root))
		}
	}
	return out
}
