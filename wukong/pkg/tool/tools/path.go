package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func resolvePath(baseDir, target string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	targetPath := target
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(baseAbs, targetPath)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	if targetAbs != baseAbs && !strings.HasPrefix(strings.ToLower(targetAbs), strings.ToLower(baseAbs)+string(filepath.Separator)) {
		return "", fmt.Errorf("path out of base dir")
	}
	return targetAbs, nil
}

func buildAutoWritePath(params map[string]any, now time.Time) string {
	title := readString(params, "title", "name", "topic", "subject", "prompt", "query", "input")
	title = sanitizePathFragment(title)
	if strings.TrimSpace(title) == "" {
		title = "untitled"
	}
	datePart := now.Format("20060102")
	timePart := fmt.Sprintf("%02d%02d%02d%06d", now.Hour(), now.Minute(), now.Second(), now.Nanosecond()/1000)
	return filepath.Join(datePart, fmt.Sprintf("%s-%s.md", title, timePart))
}

func sanitizePathFragment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastUnderscore = false
		case r == ' ':
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		default:
			if r <= 0x1f || r == 0x7f {
				continue
			}
			b.WriteRune(r)
			lastUnderscore = false
		}
	}
	return strings.Trim(b.String(), "._ ")
}
