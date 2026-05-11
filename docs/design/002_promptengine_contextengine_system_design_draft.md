很好，这一步就开始像“系统能力”了，不再只是把 prompt 收口，而是在搭一层真正的推理基础设施。

如果 `PromptEngine` 是“怎么说”，那 `ContextEngine` 更像“拿什么来说”。这两个模块最好从一开始就解耦，否则很快会重新缠成业务代码里的 if/else 泥团。

我先给你一个适合 Wukong 当前阶段的架构设计，再把后续 task 拆出来。

---

**核心思路**

把这两层明确拆开：

- `PromptEngine`
  - 管理模板
  - 渲染消息
  - 负责提示词结构化表达

- `ContextEngine`
  - 管理上下文来源、装配、裁剪、排序、降级
  - 负责把“可供 LLM 使用的信息”整理成标准上下文块

两者通过一个清晰的中间层连接：

- `ContextBundle`
  - ContextEngine 输出
- `PromptInput`
  - PromptEngine 输入

也就是说：

`数据源 -> ContextEngine -> ContextBundle -> PromptEngine -> []llm.Message -> LLM`

这条链路一旦稳定，后面 chat、task、planner、react 都能复用。

---

**一、PromptEngine 和 ContextEngine 的职责边界**

**PromptEngine 职责**
- 注册 prompt 模板
- 管理模板版本和 key
- 渲染变量
- 输出 `[]llm.Message`
- 支持把上下文块插入到模板中

它不负责：
- 查数据库
- 拉历史消息
- 算 token
- 排序上下文来源
- 记忆召回

**ContextEngine 职责**
- 定义上下文源
- 从多个来源拉取上下文
- 做优先级排序、窗口截断、去重、降级
- 输出统一结构

它不负责：
- 写提示词文案
- 决定 system prompt 长什么样
- 直接调用 LLM

这样职责才干净。

---

**二、建议的模块关系**

建议目录形态：

- `pkg/prompt/`
  - `promptengine.go`
- `pkg/context/`
  - `contextengine.go`

后面如果长大再拆。

两个模块关系建议这样：

```text
ChatService / Planner / Worker / ReAct
        |
        v
  ContextEngine.Build(...)
        |
        v
   ContextBundle
        |
        v
 PromptEngine.Render(...)
        |
        v
   []llm.Message
        |
        v
       LLM
```

---

**三、ContextEngine 的核心抽象**

建议第一版先有这几个核心类型。

```go
type Engine struct {
    resolver Resolver
    sources  map[string]Source
    policies map[string]Policy
}

type BuildRequest struct {
    Scene      string
    SessionID  string
    TaskID     string
    UserID     string
    SkillName  string
    Query      string
    Variables  map[string]any
}

type ContextBundle struct {
    Scene   string
    Blocks  []ContextBlock
    Meta    map[string]any
}

type ContextBlock struct {
    Name      string
    Type      string
    Priority  int
    Content   string
    Tokens    int
    Source    string
    Timestamp int64
}

type Source interface {
    Name() string
    Load(ctx context.Context, req BuildRequest) ([]ContextBlock, error)
}
```

这个设计很关键：  
`ContextEngine` 不直接输出 prompt 文本，而是输出“上下文块列表”。

---

**四、上下文块 ContextBlock 的意义**

这层非常值钱，因为它能把来源统一掉。

比如：

- 聊天历史
- 会话摘要
- 用户画像
- 当前任务参数
- 子任务执行记录
- 工具调用结果
- 检索到的知识片段
- 系统约束
- 技能说明

都先变成：

```go
ContextBlock{
  Name: "recent_messages",
  Type: "history",
  Priority: 90,
  Content: "...",
  Source: "chat_message",
}
```

这样后面：
- PromptEngine 可以按块插入
- Token 裁剪可以按块做
- 调试日志可以按块打
- 不同场景可以复用同一来源

这就是它比“直接拼字符串”强很多的地方。

---

**五、ContextEngine 的内部处理流程**

建议第一版固定成这个 pipeline：

1. 收集 `Source`
2. 加载上下文块
3. 过滤空块/失败块
4. 去重
5. 按优先级排序
6. 按 policy 截断
7. 输出 `ContextBundle`

也就是：

```text
sources -> load -> normalize -> dedupe -> prioritize -> truncate -> bundle
```

后面你再加复杂能力，也是在这条链上扩展。

---

**六、ContextEngine 的 Source 设计**

建议按来源拆 source，而不是按业务函数拆。

第一批 Source 可以设计成：

- `ChatHistorySource`
  - 读 `chat_message`
- `ChatMemorySource`
  - 读 `chat_memory`
- `TaskStateSource`
  - 读任务、子任务、状态流
- `SkillSpecSource`
  - 读 skill 配置/说明
- `ToolResultSource`
  - 读工具输出
- `StaticRuleSource`
  - 固定系统约束
- `UserProfileSource`
  - 后续扩展
- `KnowledgeRetrieveSource`
  - 后续 RAG 扩展

这样以后 chat 和 task 场景只是选不同 source 组合，而不是写两套 engine。

---

**七、Policy 设计：让 ContextEngine 可组合**

如果 Source 是“拿什么”，那 Policy 是“怎么装”。

建议引入一个简单策略接口：

```go
type Policy interface {
    Name() string
    Apply(ctx context.Context, blocks []ContextBlock, req BuildRequest) ([]ContextBlock, error)
}
```

第一版策略可以只有几个：

- `PrioritySortPolicy`
- `RecentWindowPolicy`
- `TokenBudgetPolicy`
- `DedupePolicy`

这样不同场景可以组合：

**Chat 场景**
- memory summary
- recent history
- current user query
- token budget small/medium

**Task 场景**
- task goal
- task params
- subtask history
- tool outputs
- execution constraints

**Planner 场景**
- user request
- task template
- skill metadata
- planning rules

这就是“独立又可组合”的关键。

---

**八、PromptEngine 怎么消费 ContextBundle**

这里建议不要让 PromptEngine 直接知道每个 source。

它只关心统一输入，例如：

```go
type PromptInput struct {
    Variables map[string]any
    Context   *ContextBundle
}
```

然后模板中支持约定好的上下文字段：

- `{{context_text}}`
- `{{context.recent_messages}}`
- `{{context.memory_summary}}`

更稳的做法是先让 ContextEngine 输出两个层次：

```go
type ContextBundle struct {
    Blocks []ContextBlock
    Text   string
    Named  map[string]string
    Meta   map[string]any
}
```

这样 PromptEngine 第一版可以很简单：

- 直接用 `bundle.Text`
- 或用 `bundle.Named["recent_messages"]`

后面再升级模板能力。

---

**九、两者组合方式**

建议三种组合层级：

**1. 独立使用**
- 只有 PromptEngine：适合固定 prompt
- 只有 ContextEngine：适合调试上下文装配

**2. 显式组合**
业务层自己调：

```go
bundle := contextEngine.Build(...)
msgs := promptEngine.Render("chat.session.default", PromptInput{
    Variables: vars,
    Context: bundle,
})
```

**3. 封装 Facade**
后面可以再做一层：

```go
type ComposeEngine struct {
    prompt  *prompt.Engine
    context *context.Engine
}
```

对外提供：

```go
BuildMessages(ctx, scene, req) ([]llm.Message, error)
```

但这层不要第一版就做太重。先让两个引擎自己站稳。

---

**十、场景化架构建议**

建议先定义几个 scene，方便统一：

- `chat`
- `task`
- `planner`
- `react`
- `tool_call`

然后：

- ContextEngine 根据 scene 选择 source + policy
- PromptEngine 根据 scene 选择 template key

例如：

**chat**
- sources: memory + history
- template: `chat.session.default`

**task**
- sources: task state + subtask trace + skill config
- template: `worker.action.default`

**planner**
- sources: task request + template + skill info
- template: `planner.task.default`

这会让系统越来越像平台，而不是一堆散函数。

---

**十一、推荐的第一版 ContextEngine 能力边界**

第一版别贪大，做这些就够：

- Source 注册
- 按 scene 选择 source
- 加载上下文块
- 简单优先级排序
- 简单窗口截断
- 输出 bundle
- 支持失败降级

先不做：

- 精准 token tokenizer
- 多租户 prompt 覆盖
- 复杂表达式模板
- 向量检索编排
- 自动摘要链
- 自反思上下文回写

这些等链路跑起来再说。

---

**十二、建议的第一版落地顺序**

**阶段 1：PromptEngine 落地**
- 模板注册
- 变量渲染
- 输出消息

**阶段 2：ContextEngine 骨架**
- `BuildRequest`
- `ContextBlock`
- `ContextBundle`
- `Source`
- `Engine.Build`

**阶段 3：先接 Chat**
- `ChatHistorySource`
- `ChatMemorySource`
- Chat scene policy
- `chat_service.go` 改成通过 ContextEngine + PromptEngine 组装消息

**阶段 4：再接 Planner / Worker**
- `planner.task.default`
- `worker.action.default`
- `react.default`

这样路径最自然。

---

**十三、Task 规划**

**阶段 A：架构骨架**
1. 新建 `pkg/prompt/promptengine.go`
2. 新建 `pkg/context/contextengine.go`
3. 定义核心类型和接口
4. 加基础单元测试

**阶段 B：PromptEngine 完成**
1. 注册模板
2. 渲染变量
3. 输出 `[]llm.Message`
4. 增加 builtin 模板注册能力

**阶段 C：ContextEngine 完成**
1. 注册 source
2. `BuildRequest`
3. `ContextBundle`
4. 简单 policy pipeline
5. 支持按 scene 装配

**阶段 D：先接入 Chat**
1. 实现 `ChatHistorySource`
2. 实现 `ChatMemorySource`
3. 定义 `chat` scene
4. 改 `ChatService` 使用新链路

**阶段 E：扩展到 Task / Planner**
1. 实现 `TaskStateSource`
2. 实现 `SkillSpecSource`
3. 改 `llm_planner.go`
4. 改 `exec_subtask.go` / `react.go`

---

**十四、我建议的最终形态**

长远看，这两个模块最好变成：

- `PromptEngine`
  - “模板中心”

- `ContextEngine`
  - “上下文装配中心”

- `ComposeEngine` 或业务层 facade
  - “消息构建入口”

对应一句话：

**PromptEngine 决定表达结构，ContextEngine 决定信息供给。**

这个分层一旦站住，Wukong 后面做多轮对话、任务执行、ReAct、记忆、RAG、技能插件，都会轻很多。

---

**我建议的下一步**
下一轮最适合先产出一版正式设计文档，内容包括：

1. `PromptEngine` 架构
2. `ContextEngine` 架构
3. 两者组合流程图
4. 接入 Chat/Task/Planner 的分阶段计划
5. 核心接口定义草案

然后再开始写代码，会稳很多。