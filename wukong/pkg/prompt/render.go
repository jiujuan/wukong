package prompt

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/jiujuan/wukong/pkg/llm"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

func (e *Engine) Render(key string, input RenderInput) ([]llm.Message, error) {
	t, ok := e.Get(key)
	if !ok {
		return nil, fmt.Errorf("template %q not found", strings.TrimSpace(key))
	}
	vars := buildRenderVars(input)
	out := make([]llm.Message, 0, len(t.Messages))
	missingSet := make(map[string]struct{})
	for _, item := range t.Messages {
		content, missing := renderText(item.Content, vars)
		for _, name := range missing {
			missingSet[name] = struct{}{}
		}
		out = append(out, llm.Message{
			Role:    strings.TrimSpace(item.Role),
			Content: content,
		})
	}
	if len(missingSet) > 0 {
		keys := make([]string, 0, len(missingSet))
		for name := range missingSet {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		return nil, &MissingVariablesError{
			TemplateKey: t.Key,
			Keys:        keys,
		}
	}
	return out, nil
}

func buildRenderVars(input RenderInput) map[string]string {
	vars := make(map[string]string)
	for k, v := range input.Variables {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		vars[key] = stringify(v)
	}
	addContextVars(vars, input.Context)
	return vars
}

func addContextVars(vars map[string]string, contextValue any) {
	if contextValue == nil {
		return
	}
	switch v := contextValue.(type) {
	case string:
		vars["context"] = v
		vars["context_text"] = v
	case map[string]string:
		for key, value := range v {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			vars["context."+key] = value
		}
	case map[string]any:
		for key, value := range v {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			vars["context."+key] = stringify(value)
		}
	default:
		rv := reflect.ValueOf(contextValue)
		if rv.Kind() == reflect.Map {
			iter := rv.MapRange()
			for iter.Next() {
				key := strings.TrimSpace(fmt.Sprint(iter.Key().Interface()))
				if key == "" {
					continue
				}
				vars["context."+key] = stringify(iter.Value().Interface())
			}
			return
		}
		vars["context"] = stringify(contextValue)
		vars["context_text"] = stringify(contextValue)
	}
}

func renderText(raw string, vars map[string]string) (string, []string) {
	matches := placeholderPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return raw, nil
	}
	missingSet := make(map[string]struct{})
	rendered := placeholderPattern.ReplaceAllStringFunc(raw, func(match string) string {
		sub := placeholderPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := strings.TrimSpace(sub[1])
		value, ok := vars[key]
		if !ok {
			missingSet[key] = struct{}{}
			return match
		}
		return value
	})
	if len(missingSet) == 0 {
		return rendered, nil
	}
	missing := make([]string, 0, len(missingSet))
	for key := range missingSet {
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return rendered, missing
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprint(v)
	}
}
