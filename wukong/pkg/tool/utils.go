package tool

import (
	"fmt"
	"strings"
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
			if strings.TrimSpace(value) != "" {
				return value
			}
		default:
			text := fmt.Sprintf("%v", value)
			if strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}
