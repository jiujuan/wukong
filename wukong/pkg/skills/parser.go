package skills

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func parseSkillFile(path string, dirName string) (*Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	item := &Skill{
		SkillName: strings.ToLower(strings.TrimSpace(dirName)),
		Version:   "1.0.0",
		Enabled:   true,
		Memory: MemoryConfig{
			MemoryType:     "working",
			WindowSize:     5,
			CompressSwitch: true,
		},
		SourcePath: path,
	}

	section := ""
	descBuf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(f)
	paramRegex := regexp.MustCompile(`^\s*-\s*([^:]+):\s*([^\(]+)\(([^)]*)\)\s*$`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# Skill:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "# Skill:"))
			if name != "" {
				item.SkillName = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}
		if line == "" {
			if section == "description" {
				descBuf.WriteString("\n")
			}
			continue
		}

		switch section {
		case "description":
			if descBuf.Len() > 0 {
				descBuf.WriteString(" ")
			}
			descBuf.WriteString(line)
		case "params":
			match := paramRegex.FindStringSubmatch(line)
			if len(match) == 4 {
				required := strings.Contains(match[3], "必填")
				defaultVal := extractDefaultValue(match[3])
				item.Params = append(item.Params, Param{
					Name:       strings.TrimSpace(match[1]),
					Type:       strings.TrimSpace(match[2]),
					Required:   required,
					DefaultVal: defaultVal,
				})
			}
		case "tools":
			tool := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if tool != "" {
				item.Tools = append(item.Tools, strings.ToLower(tool))
			}
		case "execute":
			item.Execute = strings.TrimSpace(strings.TrimPrefix(line, "-"))
			item.Execute = strings.TrimSpace(item.Execute)
		case "template":
			item.Template = strings.TrimSpace(strings.TrimPrefix(line, "-"))
			item.Template = strings.TrimSpace(item.Template)
		case "memory config":
			parseMemoryConfigLine(item, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	item.Description = strings.TrimSpace(descBuf.String())
	if item.SkillName == "" {
		return nil, fmt.Errorf("skill name empty: %s", path)
	}
	return item, nil
}

// ParseSkillFile parses a legacy-style SKILL.md file.
func ParseSkillFile(path string, dirName string) (*Skill, error) {
	return parseSkillFile(path, dirName)
}

func parseMemoryConfigLine(item *Skill, line string) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	val := strings.TrimSpace(parts[1])
	switch key {
	case "memory_type":
		if val != "" {
			item.Memory.MemoryType = val
		}
	case "window_size":
		var n int
		_, _ = fmt.Sscanf(val, "%d", &n)
		if n > 0 {
			item.Memory.WindowSize = n
		}
	case "compress_switch":
		item.Memory.CompressSwitch = strings.EqualFold(val, "true")
	case "rag_collection":
		item.Memory.RAGCollection = val
	case "expire_time":
		item.Memory.ExpireTime = val
	}
}

func extractDefaultValue(raw string) string {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "默认值") {
			return strings.TrimSpace(strings.TrimPrefix(part, "默认值"))
		}
	}
	return ""
}
