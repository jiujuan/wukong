package tools

type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Example     any    `json:"example,omitempty"`
}

type ToolInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Schema      []ParamSchema `json:"schema,omitempty"`
}

func schemaItem(name, typ string, required bool, description string, def any, example any) ParamSchema {
	return ParamSchema{
		Name:        name,
		Type:        typ,
		Required:    required,
		Description: description,
		Default:     def,
		Example:     example,
	}
}
