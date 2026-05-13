package adapter

import (
	"path/filepath"
	"strings"
)

func normalizeSkillName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		"@", "",
		":", "_",
	)
	return replacer.Replace(name)
}

func runtimeFromEntry(path string) string {
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
