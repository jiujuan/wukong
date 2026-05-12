package context

// BuildRequest 表示用于构建上下文包的输入，包括场景信息和变量。
type BuildRequest struct {
	Scene     string
	UserID    string
	SessionID string
	TaskID    string
	SkillName string
	Query     string
	Variables map[string]any
}

// ContextBlock 表示单个上下文信息块，可以是各种类型（例如，文本、表格、图像），包含内容和相关元数据。
type ContextBlock struct {
	Name      string
	Type      string
	Source    string
	Content   string
	Priority  int
	Tokens    int
	Timestamp int64
}

// ContextBundle 上下文信息整合。
type ContextBundle struct {
	Scene  string
	Blocks []ContextBlock
	Named  map[string]string
	Text   string
	Meta   map[string]any
}
