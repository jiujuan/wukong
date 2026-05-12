# 目录草案

建议分开，放在 `pkg` 下两个独立目录里，不要先塞进一个目录。

我建议这样：

```text
wukong/pkg/
  prompt/
    promptengine.go
    builtin.go
    render.go
    promptengine_test.go

  context/
    contextengine.go
    source.go
    policy.go
    scene.go
    contextengine_test.go
```

如果后面要加组合层，再单独补一个轻量目录：

```text
  compose/
    promptbuilder.go
```

或者先不建，先由业务层自己串起来也行。

## 为什么建议分开

因为这两个模块职责不一样：

- `prompt/`
  - 管模板
  - 管渲染
  - 输出 `[]llm.Message`

- `context/`
  - 管上下文来源
  - 管上下文块装配
  - 管裁剪、排序、降级

把它们放一个目录，短期看方便，后面很容易重新耦合成一团。分开后：

- 依赖边界清楚
- 单测更好写
- 可以独立演进
- 以后 `PromptEngine` 单独给 `worker/react` 用也很自然
- `ContextEngine` 单独做调试和快照也方便

## 推荐依赖方向

建议只允许这条方向：

```text
business -> context -> prompt -> llm types
```

更准确一点是：

- `context` 不依赖 `prompt`
- `prompt` 最好也不依赖 `context` 的复杂实现，只依赖一个轻量输入结构
- 业务层负责把 `ContextBundle` 传给 `PromptEngine`

## 第一版更稳的目录结构

如果你想先保守一点，我建议第一版甚至可以更简单：

```text
wukong/pkg/
  prompt/
    promptengine.go
    promptengine_test.go

  context/
    contextengine.go
    contextengine_test.go
```

等功能长出来再拆 `builtin.go / source.go / policy.go`。

## 不建议的方式

不建议先做成：

```text
pkg/ai/
  promptengine.go
  contextengine.go
```

或者：

```text
pkg/engine/
  prompt.go
  context.go
```

因为名字太大、边界太模糊，后面容易继续往里塞 planner、memory、rag、skill runtime，最后变成杂物间。

## 结论

我的建议是：

- `pkg/prompt/`
- `pkg/context/`

两个独立目录，先分开设计和实现。  
组合层先不急着抽目录，等你开始接 `chat/planner/worker` 时，如果重复串联逻辑明显，再加一个 `pkg/compose/` 或 `pkg/promptbuilder/`。

如果你愿意，我下一步可以直接给你一版“推荐的 pkg 目录树 + 每个文件职责说明”。

# 目录建议

可以，下面给你一版适合当前 Wukong 的 `pkg` 目录树建议，以及每个文件的职责边界。

---

# 推荐目录树

```text
wukong/pkg/
  prompt/
    promptengine.go
    builtin.go
    render.go
    types.go
    promptengine_test.go

  context/
    contextengine.go
    registry.go
    source.go
    policy.go
    scene.go
    types.go
    contextengine_test.go

  promptbuilder/
    builder.go
    types.go
    builder_test.go
```

---

# 一、`pkg/prompt/`

职责：  
只负责 prompt 模板管理、变量渲染、消息生成。

## 1. `promptengine.go`
主入口文件。

负责：
- `Engine` 结构体
- `New()`
- `Register()`
- `Get()`
- `Render()`
- `RenderTemplate()`

这是 prompt 模块的门面。

---

## 2. `builtin.go`
内置模板注册。

负责：
- 注册系统内置 prompt
- 内置模板 key 维护
- 提供 `RegisterBuiltins()` 或 `NewDefaultEngine()`

这里放：
- `chat.session.default`
- `planner.task.default`
- `worker.action.default`
- `worker.react.default`

不要把渲染逻辑放这里。

---

## 3. `render.go`
模板渲染实现。

负责：
- `{{var}}` 替换
- 缺失变量检测
- context 字段展开
- 模板字符串转 `llm.Message`

如果后面要支持更多渲染规则，也主要扩这里。

---

## 4. `types.go`
类型定义。

负责：
- `Template`
- `MessageTemplate`
- `RenderInput`
- `RenderOption`
- 错误类型定义

把类型单独放出来，避免 `promptengine.go` 太胖。

---

## 5. `promptengine_test.go`
prompt 模块测试。

重点覆盖：
- 模板注册成功
- 重复 key 报错
- 渲染变量成功
- 缺失变量报错
- 输出消息顺序正确

---

# 二、`pkg/context/`

职责：  
只负责上下文来源管理、上下文块构建、裁剪与组合。

## 1. `contextengine.go`
主入口文件。

负责：
- `Engine`
- `New()`
- `Build()`
- scene 装配流程
- source + policy 执行主链路

这是 context 模块的门面。

---

## 2. `registry.go`
注册中心。

负责：
- source 注册
- policy 注册
- scene 注册
- 查询与校验注册项

比如：
- `RegisterSource()`
- `RegisterPolicy()`
- `RegisterScene()`

这样 `contextengine.go` 不会塞满注册逻辑。

---

## 3. `source.go`
上下文源接口与基础实现约定。

负责：
- `Source` 接口
- source 基础工具函数
- 通用 source helper

后续具体 source 可以继续拆子文件，比如：
- `source_chat.go`
- `source_task.go`
- `source_skill.go`

第一版先不急着拆。

---

## 4. `policy.go`
上下文策略接口与通用策略。

负责：
- `Policy` 接口
- 去重策略
- 排序策略
- 截断策略
- token budget 策略

后续如果策略变多，可以再拆：
- `policy_dedupe.go`
- `policy_budget.go`

---

## 5. `scene.go`
场景配置。

负责：
- `SceneConfig`
- scene 默认配置
- 各场景 source/policy 组合

这里定义：
- `chat`
- `planner`
- `worker`
- `react`

这样业务层只传 scene，不要自己拼 source 列表。

---

## 6. `types.go`
context 模块类型定义。

负责：
- `BuildRequest`
- `ContextBundle`
- `ContextBlock`

这个文件很重要，因为它会成为上下文层的公共模型。

---

## 7. `contextengine_test.go`
context 模块测试。

重点覆盖：
- source 装配正确
- policy 顺序正确
- 空上下文降级正常
- scene 配置正确
- 输出 bundle 可预期

---

# 三、`pkg/promptbuilder/`

这个目录不是第一步必须建，但我建议预留。  
它的作用是把 `ContextEngine + PromptEngine` 串起来，给业务层一个更干净的统一入口。

职责：  
负责从 scene/request 直接构建最终 `[]llm.Message`。

---

## 1. `builder.go`
主入口。

负责：
- 组合 `context.Engine`
- 组合 `prompt.Engine`
- 提供统一的 `BuildMessages()` 方法

示意：

```go
func (b *Builder) BuildMessages(ctx context.Context, req BuildRequest) ([]llm.Message, error)
```

适合给：
- `ChatService`
- `Planner`
- `Worker`
- `ReActExecutor`

直接使用。

---

## 2. `types.go`
组合层类型。

负责：
- `BuildRequest`
- `BuildResult`
- `SceneTemplateBinding`

比如：
- scene 用哪个 template
- 变量从哪里来
- context 怎么映射进 prompt

---

## 3. `builder_test.go`
组合层测试。

重点覆盖：
- context 产出是否正确传入 prompt
- scene/template 是否绑定正确
- 最终消息结构是否正确

---

# 四、推荐的演进顺序

## 第一阶段
先建最小结构：

```text
pkg/
  prompt/
    promptengine.go
    promptengine_test.go

  context/
    contextengine.go
    contextengine_test.go
```

先把骨架立住。

---

## 第二阶段
功能长出来后再补：

```text
pkg/
  prompt/
    builtin.go
    render.go
    types.go

  context/
    registry.go
    source.go
    policy.go
    scene.go
    types.go
```

---

## 第三阶段
当业务里出现明显重复的“先 build context，再 render prompt”代码时，再加：

```text
pkg/promptbuilder/
```

---

# 五、依赖关系建议

建议保持这个方向：

```text
context  -> only own types and source deps
prompt   -> llm types + own render types
promptbuilder -> context + prompt
business -> promptbuilder or context+prompt
```

更直白一点：

- `context` 不依赖 `prompt`
- `prompt` 不依赖 `context` 的复杂实现
- `promptbuilder` 才负责组合
- `chat/task/planner/worker` 尽量不要自己手搓拼接

---

# 六、各目录适合放什么，不适合放什么

## `pkg/prompt/` 适合放
- prompt 模板
- prompt 渲染
- 模板 key 常量
- prompt 注册

## `pkg/prompt/` 不适合放
- 查数据库
- 历史消息查询
- memory 检索
- token 裁剪策略

---

## `pkg/context/` 适合放
- context source
- context policy
- context bundle
- scene 装配

## `pkg/context/` 不适合放
- prompt 文案
- system prompt 模板
- llm 请求发送

---

## `pkg/promptbuilder/` 适合放
- 场景到模板绑定
- context 到 prompt 的桥接
- 最终 message 构建

## `pkg/promptbuilder/` 不适合放
- 复杂业务逻辑
- 任务执行状态机
- memory 持久化

---

# 七、最终建议

如果按“当前项目阶段 + 后续可扩展性”综合看，我最推荐的是：

```text
wukong/pkg/
  prompt/
  context/
  promptbuilder/   # 可后置
```

也就是：

- `prompt` 和 `context` 必须独立目录
- `promptbuilder` 作为组合层，先可选，后再加

这样最稳，也最不容易把边界做乱。

如果你愿意，我下一步可以继续给你一版：
**“这 3 个目录下每个核心结构体的 Go 接口草案”**。