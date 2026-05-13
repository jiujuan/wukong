package tools

import (
	"fmt"

	wkstr "github.com/jiujuan/wukong/pkg/str"
)

func readString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		if source == nil {
			continue
		}
		v, ok := source[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			if wkstr.NotEmpty(value) {
				return value
			}
		default:
			text := fmt.Sprintf("%v", value)
			if wkstr.NotEmpty(text) {
				return text
			}
		}
	}
	return ""
}

func mapKeys(items map[string]any) []string {
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
