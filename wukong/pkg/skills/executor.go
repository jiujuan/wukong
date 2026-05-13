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
			sandbox.WithAllowedEnvKeys(
				"SKILL_NAME",
				"SKILL_VERSION",
				"SKILL_SOURCE_TYPE",
				"SKILL_RUNTIME",
				"SKILL_ENTRY",
				"SKILL_PARAMS",
				"SKILL_REFERENCES",
				"SKILL_ASSETS",
				"WUKONG_SKILL_ROOT",
				"WUKONG_OUTPUT_DIR",
			),
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
	return r.ExecuteWithSkill(ctx, item, params)
}

func (r *Registry) ExecuteWithSkill(ctx context.Context, skill *Skill, params map[string]any) (map[string]any, error) {
	if skill == nil {
		return nil, fmt.Errorf("skill is nil")
	}
	if !skill.Enabled {
		return nil, fmt.Errorf("skill disabled: %s", skill.SkillName)
	}
	runtimeName, entry, err := resolveRuntimeEntry(skill)
	if err != nil {
		return nil, err
	}
	rootDir := skillRootDir(skill)
	if rootDir == "" {
		return nil, fmt.Errorf("skill root empty: %s", skill.SkillName)
	}
	outputDir := skillOutputDir(skill.SkillName)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	envMap := skillRuntimeEnv(skill, rootDir, outputDir, runtimeName, entry, params)
	runCtx, cancel := context.WithTimeout(ctx, r.execTimeout)
	defer cancel()

	request := sandbox.Request{
		Runtime:          runtimeName,
		WorkDir:          rootDir,
		Env:              envMap,
		Timeout:          r.execTimeout,
		AllowedWorkRoots: []string{rootDir, outputDir},
	}
	if runtimeName == "command" {
		request.Command = entry
	} else {
		scriptPath := entry
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(rootDir, scriptPath)
		}
		absPath, err := filepath.Abs(scriptPath)
		if err != nil {
			return nil, err
		}
		request.ScriptPath = absPath
	}

	result, err := skillSandbox().Execute(runCtx, request)
	output := combineSkillOutput(result.Stdout, result.Stderr)
	payload := map[string]any{
		"skill_name": skill.SkillName,
		"output":     output,
		"stdout":     result.Stdout,
		"stderr":     result.Stderr,
		"exit_code":  result.ExitCode,
		"truncated":  result.Truncated,
		"skill_root": rootDir,
		"output_dir": outputDir,
		"package":    skill.Package,
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

func skillRuntimeEnv(skill *Skill, rootDir, outputDir, runtimeName, entry string, params map[string]any) map[string]string {
	envMap := map[string]string{
		"SKILL_NAME":        skill.SkillName,
		"SKILL_VERSION":     skill.Version,
		"SKILL_SOURCE_TYPE": string(skill.Package.SourceType),
		"SKILL_RUNTIME":     runtimeName,
		"SKILL_ENTRY":       entry,
		"WUKONG_SKILL_ROOT": rootDir,
		"WUKONG_OUTPUT_DIR": outputDir,
	}
	if len(skill.References) > 0 {
		raw, _ := json.Marshal(skill.References)
		envMap["SKILL_REFERENCES"] = string(raw)
	}
	if len(skill.Assets) > 0 {
		raw, _ := json.Marshal(skill.Assets)
		envMap["SKILL_ASSETS"] = string(raw)
	}
	if len(params) > 0 {
		raw, err := json.Marshal(params)
		if err == nil {
			envMap["SKILL_PARAMS"] = string(raw)
		}
	}
	return envMap
}

func resolveRuntimeEntry(skill *Skill) (runtime string, entry string, err error) {
	if skill == nil {
		return "", "", fmt.Errorf("skill is nil")
	}
	canon := skill.Canonical()
	runtime = strings.TrimSpace(canon.Runtime.Runtime)
	entry = strings.TrimSpace(canon.Runtime.Entry)
	if runtime == "" {
		switch {
		case entry != "":
			runtime = runtimeFromScript(entry)
		case strings.TrimSpace(skill.Execute) != "":
			entry = strings.TrimSpace(skill.Execute)
			runtime = runtimeFromScript(entry)
		}
	}
	if runtime == "" {
		return "", "", fmt.Errorf("unsupported execute script extension: %s", filepath.Ext(entry))
	}
	if runtime == "command" {
		if entry == "" {
			return "", "", fmt.Errorf("skill command entry is empty: %s", skill.SkillName)
		}
		return runtime, entry, nil
	}
	if entry == "" {
		return "", "", fmt.Errorf("skill execute entry empty: %s", skill.SkillName)
	}
	return runtime, entry, nil
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
