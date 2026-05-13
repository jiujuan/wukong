package skills

import (
	"fmt"
	"path/filepath"

	"github.com/jiujuan/wukong/pkg/sandbox"
	wkstr "github.com/jiujuan/wukong/pkg/str"
)

var allowedSkillRuntimes = map[string]struct{}{
	"command":    {},
	"python":     {},
	"py":         {},
	"python3":    {},
	"javascript": {},
	"js":         {},
	"node":       {},
	"bash":       {},
	"sh":         {},
	"shell":      {},
	"powershell": {},
	"ps1":        {},
	"go":         {},
	"java":       {},
	"typescript": {},
	"ts":         {},
}

var allowedSkillTools = map[string]struct{}{
	"llm_chat":     {},
	"web_search":   {},
	"file_read":    {},
	"file_write":   {},
	"http_request": {},
	"code_exec":    {},
	"generate_ppt": {},
	"memory_read":  {},
	"memory_write": {},
}

func skillPackageRoot(skillFile string) string {
	if wkstr.Empty(skillFile) {
		return ""
	}
	return filepath.Clean(filepath.Dir(skillFile))
}

func normalizeSkillName(name string) string {
	name = wkstr.S(name).Trim().Lower().Replace(" ", "_").Replace("/", "_").Replace("\\", "_").Replace("@", "").Replace(":", "_").Val()
	if name == "" {
		return ""
	}
	return name
}

func validateSkillPackage(root string, item *Skill) error {
	if item == nil {
		return fmt.Errorf("skill is nil")
	}
	if wkstr.Empty(root) {
		return fmt.Errorf("skill root is empty")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if wkstr.Empty(item.SourcePath) {
		return fmt.Errorf("skill source path is empty")
	}
	skillAbs, err := filepath.Abs(item.SourcePath)
	if err != nil {
		return err
	}
	if !sandbox.WithinAllowedRoots(skillAbs, []string{rootAbs}) {
		return fmt.Errorf("skill source path is outside root: %s", skillAbs)
	}
	if wkstr.NotEmpty(item.Execute) {
		entry := item.Execute
		if !filepath.IsAbs(entry) {
			entry = filepath.Join(rootAbs, entry)
		}
		entryAbs, err := filepath.Abs(entry)
		if err != nil {
			return err
		}
		if !sandbox.WithinAllowedRoots(entryAbs, []string{rootAbs}) {
			return fmt.Errorf("skill execute path is outside root: %s", item.Execute)
		}
	}
	if err := validateSkillRuntime(item.Package.Runtime, item.Execute); err != nil {
		return err
	}
	if err := validateSkillTools(item.Tools); err != nil {
		return err
	}
	return nil
}

func validateSkillRuntime(runtimeValue, execute string) error {
	runtimeValue = wkstr.TrimLower(runtimeValue)
	if runtimeValue == "" {
		if wkstr.Empty(execute) {
			return nil
		}
		return fmt.Errorf("skill runtime is empty")
	}
	if _, ok := allowedSkillRuntimes[runtimeValue]; !ok {
		return fmt.Errorf("skill runtime not allowed: %s", runtimeValue)
	}
	return nil
}

func validateSkillTools(tools []string) error {
	for _, toolName := range tools {
		normalized := wkstr.TrimLower(toolName)
		if normalized == "" {
			continue
		}
		if _, ok := allowedSkillTools[normalized]; !ok {
			return fmt.Errorf("skill tool not allowed: %s", normalized)
		}
	}
	return nil
}
