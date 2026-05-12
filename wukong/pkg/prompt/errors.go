package prompt

import (
	"fmt"
	"strings"
)

type MissingVariablesError struct {
	TemplateKey string
	Keys        []string
}

func (e *MissingVariablesError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("render prompt %q missing variables: %s", e.TemplateKey, strings.Join(e.Keys, ", "))
}
