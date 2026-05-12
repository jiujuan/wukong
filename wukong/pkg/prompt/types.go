package prompt

type Template struct {
	Key         string
	Description string
	Version     string
	Messages    []MessageTemplate
}

type MessageTemplate struct {
	Role    string
	Content string
}

type RenderInput struct {
	Variables map[string]any
	Context   any
}
