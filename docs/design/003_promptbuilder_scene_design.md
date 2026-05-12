# PromptBuilder Scene Design

## 1. 目标

Wukong 当前已经把三层核心能力拆出来了：

- `pkg/context`：负责上下文收集与装配
- `pkg/prompt`：负责 prompt 模板管理与渲染
- `pkg/promptbuilder`：负责把上下文、模板、场景差异组合成最终发给 LLM 的消息

但目前 `chat / manager / worker` 仍然各自持有一套场景装配代码，导致：

1. 调用入口分散
2. 场景绑定逻辑重复
3. 新场景接入时需要修改多个目录
4. 业务层仍然能直接碰到底层 engine，职责边界不够干净

因此本设计的目标是把 `pkg/promptbuilder` 提升为系统唯一的“场景提示词装配入口”，让业务层只关心：

- 我属于哪个 scene
- 我需要什么上下文
- 我最终要拿到什么 messages

而不再关心：

- 上下文怎么抓
- prompt 模板怎么选
- message 怎么拼

---

## 2. 设计结论

我推荐的方向不是再造一个新的大引擎，而是把 `pkg/promptbuilder` 做成一个稳定的聚合入口：

> `PromptBuilder = ContextEngine + PromptEngine + Scene Assembler`

其中：

- `ContextEngine` 负责“拿什么”
- `PromptEngine` 负责“怎么说”
- `PromptBuilder` 负责“按 scene 怎么组装成可投喂 LLM 的消息”

也就是说：

- `pkg/prompt` 和 `pkg/context` 继续保持基础能力层
- `pkg/promptbuilder` 成为唯一的业务编排层
- `chat / planner / worker` 不再自己 new 一套 builder，而是从统一工厂拿 scene builder

---

## 3. 推荐目录树

建议将 `pkg/promptbuilder` 组织成下面的结构：

```text
pkg/promptbuilder/
  builder.go
  types.go
  factory.go
  registry.go
  options.go
  errors.go
  scenes/
    chat.go
    planner.go
    worker.go
    common.go
  builder_test.go
  factory_test.go
  scenes/
    chat_test.go
    planner_test.go
    worker_test.go
```

如果想更保守一些，也可以先不拆 `registry.go` 和 `options.go`，但**scene 文件必须拆出去**，否则调用入口还是会继续散落在 `internal/service`、`pkg/manager`、`pkg/worker` 里。

---

## 4. 每个文件职责

### 4.1 `types.go`

只放对外公共类型，不放具体实现逻辑。

建议包含：

- `BuildRequest`
- `BuildResult`
- `SceneAssembler`
- `ScenePreset`
- `BuildMode`（可选）

职责：

- 统一 builder 输入输出
- 给场景装配提供稳定的数据契约

---

### 4.2 `builder.go`

这是 `promptbuilder` 的核心运行时。

职责：

- 持有 `ContextEngine`
- 持有 `PromptEngine`
- 根据 scene 解析模板
- 调用 `ContextEngine.Build(...)`
- 调用 `PromptEngine.Render(...)`
- 调用 scene assembler 处理最终消息组装

它应该是业务层真正调用的主入口：

```go
result, err := builder.BuildMessages(ctx, req)
```

---

### 4.3 `factory.go`

负责创建和组装不同场景的 builder。

职责：

- 注册 scene preset
- 初始化 chat/planner/worker 的默认 assembler
- 提供统一的 `ForScene(...)` 或 `NewXXXBuilder(...)`

这样业务层就不会自己到处手工拼：

```go
newChatPromptBuilder(...)
newPlannerPromptBuilder(...)
newWorkerPromptBuilder(...)
```

而是统一走工厂：

```go
factory.ForScene("chat")
factory.ForScene("planner")
factory.ForScene("worker")
```

---

### 4.4 `registry.go`

负责 scene 注册、查询与模板绑定。

职责：

- scene -> template 的绑定
- scene -> assembler 的绑定
- scene -> preset 的绑定

适合放这些能力：

- `BindSceneTemplate`
- `ResolveTemplate`
- `RegisterAssembler`
- `GetAssembler`
- `RegisterPreset`

这样 `Builder` 的核心逻辑会更薄，配置能力会集中。

---

### 4.5 `options.go`

负责构造时的可选配置。

职责：

- 设置默认 scene
- 设置是否允许缺失 context
- 设置是否允许 fallback template
- 设置日志或调试开关

适合使用标准的 option 模式：

```go
type Option func(*Builder)
```

---

### 4.6 `errors.go`

统一管理 promptbuilder 相关错误。

建议包含：

- `ErrBuilderNil`
- `ErrSceneEmpty`
- `ErrTemplateNotFound`
- `ErrContextBuildFailed`
- `ErrPromptRenderFailed`

好处是：

- 上层可以做明确判断
- 单测更容易写
- 后续做降级策略更自然

---

### 4.7 `scenes/common.go`

放所有场景共享的辅助逻辑。

例如：

- scene 名称常量
- 通用变量归一化
- 共用的 message 拼接工具
- 通用 prompt context 结构

---

### 4.8 `scenes/chat.go`

只负责 chat 场景。

职责：

- chat 历史消息装配
- chat memory 装配
- system/user/assistant 消息归位
- 多轮上下文的最后一层拼装

这里不应该再出现 repository 直接查询逻辑，查询动作应由 `ContextEngine` 的 source 完成，`chat.go` 只关心最终怎么拼。

---

### 4.9 `scenes/planner.go`

只负责 planner 场景。

职责：

- 主任务描述装配
- task state / skill spec 上下文注入
- planner prompt 模板变量整理
- LLM 输出前的最终消息归整

---

### 4.10 `scenes/worker.go`

只负责 worker 场景。

职责：

- subtask 基础字段整理
- action / params / skill 上下文注入
- tool / react / llm 执行链的 prompt 准备
- worker 专用的 message assembly

---

## 5. 推荐的调用方式

### 5.1 对业务层暴露一个统一入口

业务层只拿 builder，不碰 scene 细节：

```go
result, err := promptbuilder.NewFactory(...)
  .ForScene("chat")
  .BuildMessages(ctx, req)
```

或者：

```go
builder := promptbuilder.NewChatBuilder(...)
result, err := builder.BuildMessages(ctx, req)
```

两种都可以，但我更推荐第一种，因为它更利于后续扩展新场景。

---

### 5.2 对业务层隐藏底层装配

像下面这些代码，后续应该尽量消失：

- `newChatPromptBuilder(...)`
- `newPlannerPromptBuilder(...)`
- `newWorkerPromptBuilder(...)`
- 在 service / manager / worker 中重复注册 scene template

这些逻辑应该收敛到 `pkg/promptbuilder` 内部。

---

## 6. 核心接口草案

### 6.1 `BuildRequest`

```go
type BuildRequest struct {
	Scene       string
	TemplateKey string
	Context     context.BuildRequest
	Variables   map[string]any
	Meta        map[string]any
}
```

说明：

- `Scene` 决定走哪个场景装配逻辑
- `TemplateKey` 支持显式指定模板
- `Context` 交给 `ContextEngine`
- `Variables` 交给 `PromptEngine`
- `Meta` 预留调试与扩展字段

---

### 6.2 `BuildResult`

```go
type BuildResult struct {
	Scene          string
	TemplateKey    string
	Messages       []llm.Message
	ContextBundle  *context.ContextBundle
	PromptMessages []llm.Message
	Meta           map[string]any
}
```

说明：

- `Messages` 是最终交给 LLM 的消息
- `PromptMessages` 是纯模板渲染结果，便于调试
- `ContextBundle` 便于上层观察上下文内容

---

### 6.3 `SceneAssembler`

```go
type SceneAssembler interface {
	Name() string
	BuildPromptInput(req BuildRequest, bundle *context.ContextBundle) prompt.RenderInput
	Assemble(req BuildRequest, bundle *context.ContextBundle, promptMessages []llm.Message) ([]llm.Message, error)
}
```

说明：

- `BuildPromptInput` 决定模板变量怎么映射
- `Assemble` 决定最终消息如何插入 system / history / context

---

### 6.4 `Factory`

```go
type Factory struct {
	// ...
}

func NewFactory(opts ...Option) *Factory
func (f *Factory) RegisterScene(scene string, preset ScenePreset) error
func (f *Factory) ForScene(scene string) (*Builder, error)
func (f *Factory) MustForScene(scene string) *Builder
```

---

### 6.5 `Builder`

```go
type Builder struct {
	// ...
}

func (b *Builder) BuildMessages(ctx context.Context, req BuildRequest) (*BuildResult, error)
func (b *Builder) BindSceneTemplate(scene, templateKey string)
func (b *Builder) RegisterAssembler(scene string, assembler SceneAssembler)
func (b *Builder) ResolveTemplate(scene string) (string, bool)
```

---

## 7. 推荐的 scene preset

建议先内置这三个场景：

1. `chat`
2. `planner`
3. `worker`

每个 scene preset 负责：

- 绑定默认模板 key
- 注册场景 assembler
- 定义默认上下文源组合

以后如果要扩展 `memory`、`task_detail`、`tool_call`、`react`，只需要新增 preset，不需要改基础引擎。

---

## 8. 现有代码的迁移建议

### 第一阶段

保留现有 `newChatPromptBuilder / newPlannerPromptBuilder / newWorkerPromptBuilder`，只是把它们改成内部适配层，减少对外散布。

### 第二阶段

增加 `promptbuilder.Factory`，把 scene preset 收进去，业务层只拿 factory 出来的 builder。

### 第三阶段

删除 `internal/service`、`pkg/manager`、`pkg/worker` 中重复的 builder 装配逻辑，只保留业务调用。

---

## 9. 最终判断

如果问“`pkg/promptbuilder` 该怎么设计才最稳”，我的答案是：

> 它应该是 Wukong 的统一 prompt scene orchestration 层，而不是一堆散乱 helper 的集合。

这样做之后：

- `prompt` 更纯
- `context` 更纯
- `promptbuilder` 变成唯一入口
- chat / planner / worker 的调用会变薄
- 新场景接入成本会明显下降

这也是后续把系统继续往“可扩展 Agent 平台”推进时，最值得做的一次收口。
