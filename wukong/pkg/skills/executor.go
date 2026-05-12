package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	scriptPath := item.Execute
	if item.SourcePath != "" {
		scriptPath = filepath.Join(filepath.Dir(item.SourcePath), item.Execute)
	}
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, r.execTimeout)
	defer cancel()
	envMap := map[string]string{
		"SKILL_NAME": item.SkillName,
	}
	if len(params) > 0 {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		envMap["SKILL_PARAMS"] = string(raw)
	}
	cmd, err := commandForScript(runCtx, absPath, envMap)
	if err != nil {
		return nil, err
	}
	cmd.Dir = filepath.Dir(absPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]any{
			"skill_name": item.SkillName,
			"output":     string(output),
		}, err
	}
	return map[string]any{
		"skill_name": item.SkillName,
		"output":     string(output),
	}, nil
}
func commandForScript(ctx context.Context, path string, envMap map[string]string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py":
		cmd = exec.CommandContext(ctx, "python", path)
	case ".sh":
		cmd = exec.CommandContext(ctx, "bash", path)
	case ".ps1":
		shell, err := findPowerShell()
		if err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, shell, "-ExecutionPolicy", "Bypass", "-File", path)
	default:
		return nil, fmt.Errorf("unsupported execute script extension: %s", ext)
	}
	if len(envMap) > 0 {
		env := append([]string(nil), os.Environ()...)
		for k, v := range envMap {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	return cmd, nil
}

func findPowerShell() (string, error) {
	candidates := []string{"pwsh", "powershell"}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("powershell runtime not found in PATH: tried %s", strings.Join(candidates, ", "))
}
