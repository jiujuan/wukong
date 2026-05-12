package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jiujuan/wukong/pkg/sandbox"
)

var (
	skillSandboxOnce sync.Once
	skillSandboxInst *sandbox.Sandbox
)

func skillSandbox() *sandbox.Sandbox {
	skillSandboxOnce.Do(func() {
		skillSandboxInst = sandbox.New(
			sandbox.WithAllowedEnvKeys("SKILL_NAME", "SKILL_PARAMS", "WUKONG_SKILL_ROOT", "WUKONG_OUTPUT_DIR"),
		)
	})
	return skillSandboxInst
}

func (r *Registry) Execute(ctx context.Context, skillName string) (string, error) {
	result, err := r.ExecuteWithParams(ctx, skillName, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", result["output"]), nil
}

func (r *Registry) ExecuteWithParams(ctx context.Context, skillName string, params map[string]any) (map[string]any, error) {
	item, ok := r.Get(skillName)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}
	if !item.Enabled {
		return nil, fmt.Errorf("skill disabled: %s", skillName)
	}
	if item.Execute == "" {
		return nil, fmt.Errorf("skill execute entry empty: %s", skillName)
	}

	rootDir := skillRootDir(item)
	if rootDir == "" {
		return nil, fmt.Errorf("skill root empty: %s", skillName)
	}
	scriptPath := item.Execute
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(rootDir, item.Execute)
	}
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return nil, err
	}
	if err := validateSkillPackage(rootDir, item); err != nil {
		return nil, err
	}
	outputDir := skillOutputDir(item.SkillName)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, r.execTimeout)
	defer cancel()

	envMap := map[string]string{
		"SKILL_NAME":        item.SkillName,
		"WUKONG_SKILL_ROOT": rootDir,
		"WUKONG_OUTPUT_DIR": outputDir,
	}
	if len(params) > 0 {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		envMap["SKILL_PARAMS"] = string(raw)
	}
	runtimeName := strings.TrimSpace(item.Package.Runtime)
	if runtimeName == "" {
		runtimeName = runtimeFromScript(absPath)
	}
	if strings.TrimSpace(runtimeName) == "" {
		return nil, fmt.Errorf("unsupported execute script extension: %s", filepath.Ext(absPath))
	}

	result, err := skillSandbox().Execute(runCtx, sandbox.Request{
		Runtime:          runtimeName,
		ScriptPath:       absPath,
		WorkDir:          rootDir,
		Env:              envMap,
		Timeout:          r.execTimeout,
		AllowedWorkRoots: []string{rootDir, outputDir},
	})
	output := combineSkillOutput(result.Stdout, result.Stderr)
	payload := map[string]any{
		"skill_name": item.SkillName,
		"output":     output,
		"stdout":     result.Stdout,
		"stderr":     result.Stderr,
		"exit_code":  result.ExitCode,
		"truncated":  result.Truncated,
		"skill_root": rootDir,
		"output_dir": outputDir,
		"package":    item.Package,
	}
	if err != nil {
		payload["error"] = err.Error()
		if strings.TrimSpace(output) == "" && result.Error != "" {
			payload["output"] = result.Error
		}
		return payload, err
	}
	return payload, nil
}

func skillRootDir(item *Skill) string {
	if item == nil {
		return ""
	}
	if strings.TrimSpace(item.Package.RootDir) != "" {
		return filepath.Clean(item.Package.RootDir)
	}
	if strings.TrimSpace(item.SourcePath) != "" {
		return filepath.Clean(filepath.Dir(item.SourcePath))
	}
	return ""
}

func skillOutputDir(skillName string) string {
	name := normalizeSkillName(skillName)
	if name == "" {
		name = "skill"
	}
	return filepath.Join("storage", "output_data", name)
}

func runtimeFromScript(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python"
	case ".sh":
		return "bash"
	case ".ps1":
		return "powershell"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".go":
		return "go"
	case ".java":
		return "java"
	default:
		return ""
	}
}

func combineSkillOutput(stdout, stderr string) string {
	if strings.TrimSpace(stdout) == "" {
		return stderr
	}
	if strings.TrimSpace(stderr) == "" {
		return stdout
	}
	return stdout + "\n" + stderr
}
