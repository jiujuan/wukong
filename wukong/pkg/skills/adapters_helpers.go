package skills

import "strings"

func parseListValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t'
	})
	result := make([]string, 0, len(fields))
	for _, item := range fields {
		item = strings.TrimSpace(strings.Trim(item, "\"'"))
		if item != "" {
			result = append(result, strings.ToLower(item))
		}
	}
	return result
}
