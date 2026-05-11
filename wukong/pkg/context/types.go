package context

type BuildRequest struct {
	Scene     string
	UserID    string
	SessionID string
	TaskID    string
	SkillName string
	Query     string
	Variables map[string]any
}

type ContextBlock struct {
	Name      string
	Type      string
	Source    string
	Content   string
	Priority  int
	Tokens    int
	Timestamp int64
}

type ContextBundle struct {
	Scene  string
	Blocks []ContextBlock
	Named  map[string]string
	Text   string
	Meta   map[string]any
}
