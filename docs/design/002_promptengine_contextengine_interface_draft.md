# **目录下每个核心结构体的 Go 接口草案**



好，我们直接把这 3 个目录的核心结构体接口草案铺出来，尽量做到“今天能落代码，后面还能长”。

---

# 一、`pkg/prompt/` 接口草案

## 1. `types.go`

```go
package prompt

import "github.com/jiujuan/wukong/pkg/llm"

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

type RenderOptions struct {
	IgnoreMissing bool
}

type RenderResult struct {
	TemplateKey string
	Version     string
	Messages    []llm.Message
	MissingKeys []string
}
```

---

## 2. `promptengine.go`

```go
package prompt

type Engine struct {
	templates map[string]*Template
}

func New() *Engine

func NewDefaultEngine() *Engine

func (e *Engine) Register(t *Template) error

func (e *Engine) MustRegister(t *Template)

func (e *Engine) Get(key string) (*Template, bool)

func (e *Engine) ListKeys() []string

func (e *Engine) Render(key string, input RenderInput) (*RenderResult, error)

func (e *Engine) RenderWithOptions(key string, input RenderInput, opts RenderOptions) (*RenderResult, error)

func (e *Engine) RenderTemplate(t *Template, input RenderInput, opts RenderOptions) (*RenderResult, error)
```

---

## 3. `builtin.go`

```go
package prompt

func RegisterBuiltins(e *Engine) error

func BuiltinTemplates() []*Template
```

建议内置模板先放这些 key：

```go
const (
	ChatSessionDefault   = "chat.session.default"
	PlannerTaskDefault   = "planner.task.default"
	WorkerActionDefault  = "worker.action.default"
	WorkerReactDefault   = "worker.react.default"
	ToolLLMChatDefault   = "tool.llm_chat.default"
)
```

---

## 4. `render.go`

```go
package prompt

func ExtractVariables(input RenderInput) map[string]string

func RenderString(raw string, vars map[string]string, opts RenderOptions) (string, []string, error)

func RenderMessages(msgs []MessageTemplate, vars map[string]string, opts RenderOptions) ([]llm.Message, []string, error)
```

---

# 二、`pkg/context/` 接口草案

## 1. `types.go`

```go
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
```

---

## 2. `source.go`

```go
package context

import stdctx "context"

type Source interface {
	Name() string
	Load(ctx stdctx.Context, req BuildRequest) ([]ContextBlock, error)
}
```

如果你想后面调试更强一点，也可以扩成：

```go
type SourceResult struct {
	Source string
	Blocks []ContextBlock
	Meta   map[string]any
}

type Source interface {
	Name() string
	Load(ctx stdctx.Context, req BuildRequest) (*SourceResult, error)
}
```

但第一版我建议先简单。

---

## 3. `policy.go`

```go
package context

import stdctx "context"

type Policy interface {
	Name() string
	Apply(ctx stdctx.Context, blocks []ContextBlock, req BuildRequest) ([]ContextBlock, error)
}
```

可以先预留几个实现：

```go
type DedupePolicy struct{}
type PrioritySortPolicy struct{}
type RecentWindowPolicy struct {
	MaxBlocks int
}
type TokenBudgetPolicy struct {
	MaxTokens int
}
```

---

## 4. `scene.go`

```go
package context

type SceneConfig struct {
	Name     string
	Sources  []string
	Policies []string
	Options  map[string]any
}
```

建议默认 scene：

```go
const (
	SceneChat    = "chat"
	SceneTask    = "task"
	ScenePlanner = "planner"
	SceneWorker  = "worker"
	SceneReact   = "react"
)
```

---

## 5. `registry.go`

```go
package context

type SourceRegistry struct {
	sources map[string]Source
}

func NewSourceRegistry() *SourceRegistry

func (r *SourceRegistry) Register(source Source) error

func (r *SourceRegistry) Get(name string) (Source, bool)

func (r *SourceRegistry) List() []string
```

也可以直接把 policy 注册也放一起：

```go
type PolicyRegistry struct {
	policies map[string]Policy
}

func NewPolicyRegistry() *PolicyRegistry

func (r *PolicyRegistry) Register(policy Policy) error

func (r *PolicyRegistry) Get(name string) (Policy, bool)

func (r *PolicyRegistry) List() []string
```

---

## 6. `contextengine.go`

```go
package context

import stdctx "context"

type Engine struct {
	sources  *SourceRegistry
	policies *PolicyRegistry
	scenes   map[string]SceneConfig
}

func New() *Engine

func NewDefaultEngine() *Engine

func (e *Engine) RegisterSource(source Source) error

func (e *Engine) RegisterPolicy(policy Policy) error

func (e *Engine) RegisterScene(scene SceneConfig) error

func (e *Engine) GetScene(name string) (SceneConfig, bool)

func (e *Engine) Build(ctx stdctx.Context, req BuildRequest) (*ContextBundle, error)
```

---

# 三、`pkg/messagebuilder/` 接口草案

这个目录是桥接层，专门把 `context + prompt` 串起来。

## 1. `types.go`

```go
package messagebuilder

import (
	stdctx "context"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type BuildRequest struct {
	Scene       string
	TemplateKey string
	Context     wkcontext.BuildRequest
	Variables   map[string]any
}

type BuildResult struct {
	Messages      []llm.Message
	ContextBundle *wkcontext.ContextBundle
	PromptResult  *prompt.RenderResult
}
```

这里需要补 `llm` 引用：

```go
import "github.com/jiujuan/wukong/pkg/llm"
```

---

## 2. `builder.go`

```go
package messagebuilder

import (
	stdctx "context"

	wkcontext "github.com/jiujuan/wukong/pkg/context"
	"github.com/jiujuan/wukong/pkg/prompt"
)

type Builder struct {
	contextEngine *wkcontext.Engine
	promptEngine  *prompt.Engine
}

func New(contextEngine *wkcontext.Engine, promptEngine *prompt.Engine) *Builder

func (b *Builder) BuildMessages(ctx stdctx.Context, req BuildRequest) (*BuildResult, error)
```

---

## 3. 如果希望 scene 自动映射 template
可以补一个绑定表：

```go
type SceneTemplateBinding struct {
	Scene       string
	TemplateKey string
}
```

然后：

```go
func (b *Builder) BindSceneTemplate(scene string, templateKey string)

func (b *Builder) ResolveTemplate(scene string) (string, bool)
```

这样业务层可以少传一层 `TemplateKey`。

---

# 四、推荐的默认实现关系

## 1. ChatService 未来调用方式

```go
result, err := builder.BuildMessages(ctx, messagebuilder.BuildRequest{
	Scene:       "chat",
	TemplateKey: "chat.session.default",
	Context: wkcontext.BuildRequest{
		Scene:     "chat",
		UserID:    userID,
		SessionID: sessionID,
		Query:     content,
	},
	Variables: map[string]any{
		"query": content,
	},
})
```

然后：

```go
resp, err := llmProvider.Chat(ctx, result.Messages)
```

---

## 2. Planner 调用方式

```go
result, err := builder.BuildMessages(ctx, messagebuilder.BuildRequest{
	Scene:       "planner",
	TemplateKey: "planner.task.default",
	Context: wkcontext.BuildRequest{
		Scene:     "planner",
		TaskID:    task.TaskID,
		SkillName: task.SkillName,
		Query:     taskPrompt,
	},
	Variables: map[string]any{
		"task_id": task.TaskID,
		"skill":   task.SkillName,
	},
})
```

---

## 3. Worker / ReAct 调用方式

```go
result, err := builder.BuildMessages(ctx, messagebuilder.BuildRequest{
	Scene:       "worker",
	TemplateKey: "worker.action.default",
	Context: wkcontext.BuildRequest{
		Scene:     "worker",
		TaskID:    subTask.TaskID,
		SkillName: subTask.SkillName,
		Query:     subTask.Action,
	},
	Variables: map[string]any{
		"action": subTask.Action,
		"params": subTask.Params,
	},
})
```

---

# 五、第一版实现时我建议再加的两个小接口

## 1. PromptEngine 的模板提供者接口
如果以后想从别处加载模板：

```go
type TemplateProvider interface {
	Load() ([]*Template, error)
}
```

---

## 2. ContextEngine 的上下文格式化接口
如果以后不同 scene 想有不同 bundle 输出规则：

```go
type Formatter interface {
	Format(scene string, blocks []ContextBlock) (*ContextBundle, error)
}
```

第一版可以先不做实现，只留未来扩展位。

---

# 六、我对第一版接口的最终建议

如果你现在就要开始写代码，我建议第一版只落这些最小集合：

## `pkg/prompt`
- `Template`
- `MessageTemplate`
- `RenderInput`
- `Engine`
- `Register / Get / Render`

## `pkg/context`
- `BuildRequest`
- `ContextBlock`
- `ContextBundle`
- `Source`
- `Policy`
- `Engine.Build`

## `pkg/messagebuilder`
- `Builder`
- `BuildRequest`
- `BuildResult`
- `BuildMessages`

这样最稳，不会一开始把接口设计得太大。

如果你愿意，我下一步可以继续给你一版：
**“这三套接口对应的第一版最小可运行代码骨架”**。