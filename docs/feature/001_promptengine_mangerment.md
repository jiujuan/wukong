# prompt engine 模块

统一管理项目的prompt和执行prompt。

## 梳理项目prompt现状

我先把现状梳理一下，方便我们规划 `pkg/prompt/promptengine.go` 不会和现有实现打架。

现在项目里的 prompt 还是分散的，主要有几类：

- `internal/service/chat_service.go`
  - Chat 多轮对话的 system prompt 和 memory/history 拼装
- `pkg/manager/llm_planner.go`
  - 任务规划器 prompt
- `pkg/worker/exec_subtask.go`
  - 不同 action/skill 的子任务 prompt builder
- `pkg/worker/react.go`
  - ReAct 工具调用 prompt
- `pkg/tool/toolkit.go`
  - `llm_chat` 这种工具级 prompt

也就是说，项目已经有“prompt 构建逻辑”，但还没有统一的“prompt 引擎”。

## **建议目标**
`promptengine.go` 不要一上来做成复杂平台，而是先解决这 4 件事：

1. 统一 prompt 的注册和命名
2. 统一变量渲染和消息组装
3. 统一 system/user/assistant message 的生成出口
4. 给后续版本化、覆盖、自定义留接口

---

## **规划方案**

### **1. 先定义 PromptEngine 的职责边界**
第一版只管“提示词模板管理与消息构建”，不直接耦合 LLM 调用。

职责建议包括：

- 注册 prompt 模板
- 根据 prompt key 取模板
- 注入变量渲染文本
- 生成 `[]llm.Message`
- 支持 system/user 多段消息模板
- 支持默认模板和自定义覆盖

不放进第一版的：

- prompt 持久化到数据库
- 在线热更新
- A/B 实验
- token 预算裁剪
- 复杂 DSL

---

### **2. 设计核心数据结构**
建议在 `pkg/prompt/promptengine.go` 里先有这些核心类型：

- `Engine`
- `Template`
- `MessageTemplate`
- `RenderInput`

一个比较稳的方向是：

```go
type Engine struct {
    mu        sync.RWMutex
    templates map[string]*Template
}

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

type RenderInput map[string]any
```

这样它天然适配你们现在大量使用的 `[]llm.Message`。

---

### **3. 第一版先支持最简单可靠的渲染能力**
先只支持：

- `{{var}}` 变量替换
- 缺失变量时报错或按选项决定是否忽略
- 输出 `[]llm.Message`

不要第一版就上 Go `text/template` 的全部能力，容易把 prompt 层变成脚本层。

建议先封装成：

- `Register(t *Template) error`
- `Get(key string) (*Template, bool)`
- `Render(key string, input RenderInput) ([]llm.Message, error)`

如果后面要增强，再扩成：

- `RenderTemplate(t *Template, input RenderInput)`
- `MustRegister(...)`

---

### **4. 给项目里的几类 prompt 建统一命名**
建议先约定 key 命名，不然以后会乱：

- `chat.session.default`
- `chat.memory.summary`
- `planner.task.default`
- `worker.action.default`
- `worker.action.web_search`
- `worker.action.report_gen`
- `worker.react.default`
- `tool.llm_chat.default`

这样后面替换现有散落字符串时，不会失控。

---

### **5. 第一批接入顺序**
我建议不要全项目同时替换，按风险从低到高来：

1. `pkg/worker/exec_subtask.go`
   - 已经有 builder 抽象，最适合先接入
2. `pkg/worker/react.go`
   - prompt 比较独立
3. `pkg/manager/llm_planner.go`
   - 规划器 prompt 也比较集中
4. `internal/service/chat_service.go`
   - 这个和 memory/history 组装耦合更深，放最后

这样落地更稳。

---

## **Task 规划**

1. 在 `pkg/prompt/` 下设计 `promptengine.go`
   - 定义 `Engine`、`Template`、`MessageTemplate`、`RenderInput`
   - 实现注册、查询、渲染基础能力

2. 补一个默认模板注册入口
   - 比如 `NewDefaultEngine()` 或 `RegisterBuiltins()`
   - 放入 chat/planner/worker/react 的基础模板 key

3. 增加单元测试
   - 注册成功/重复注册
   - 变量渲染成功
   - 缺失变量报错
   - 输出消息顺序正确

4. 先接入 `pkg/worker/exec_subtask.go`
   - 用 prompt engine 替换硬编码 system/user prompt

5. 再接入 `pkg/worker/react.go`
   - 把 ReAct system prompt 纳入统一模板管理

6. 最后评估是否继续接入 planner/chat
   - planner 优先
   - chat 最后做

---

## **我建议的第一版文件形态**
可以先只做一个文件：

- `wukong/pkg/prompt/promptengine.go`

后面如果长大，再拆：

- `engine.go`
- `render.go`
- `builtin.go`
- `promptengine_test.go`

第一版先不拆，方便快速成型。

---

## **这次规划后的实现建议**
如果下一步直接做，我建议本轮只完成：

- `promptengine.go`
- `promptengine_test.go`
- `pkg/worker/exec_subtask.go` 的首个接入

这是最合适的第一刀，范围小，收益明显，也能验证设计是不是顺手。