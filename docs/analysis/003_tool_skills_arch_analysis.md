# Tools 与 Skills 能力架构分析

## 1. 总体判断

`pkg/tool` 和 `pkg/skills` 已经形成了 Wukong 当前工具/技能体系的基础闭环：

- `pkg/skills` 负责定义“能力声明”：某个 Skill 是什么、需要什么参数、允许使用哪些 Tools、如何执行、记忆配置是什么。
- `pkg/tool` 负责定义“原子工具执行”：LLM、搜索、文件读写、HTTP、代码执行、记忆读写等。
- `pkg/worker` 是二者真正发生作用的运行时：根据 task/subtask 选择 skill，再根据 skill 的 tools 白名单调用具体 tool，必要时通过 ReAct 进行多轮工具调用。

一句话总结：

> Skill 是能力策略和权限声明，Tool 是可执行原子能力，Worker 是调度它们的运行时。

## 2. Tools 现有架构

核心实现位于：

- `wukong/pkg/tool/toolkit.go`

### 2.1 核心接口

当前 Tool 的统一接口是：

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}
```

这个接口定义非常轻量，优点是新增工具成本低；缺点是参数和返回值都缺少强约束，调用方只能通过约定理解输入输出。

### 2.2 Tool Manager

`tool.Manager` 负责工具注册、查询、列表、执行和按 Skill 权限执行：

- `Register`
- `Get`
- `List`
- `Execute`
- `ExecuteForSkill`

其中 `ExecuteForSkill` 是当前 Tools 与 Skills 的关键连接点：

```go
func (m *Manager) ExecuteForSkill(ctx context.Context, skillName, toolName string, params map[string]any) (map[string]any, error)
```

它会通过 `skills.Registry.CanUseTool(...)` 判断某个 skill 是否允许调用某个 tool，从而形成基础权限边界。

### 2.3 内置工具

当前内置工具包括：

| Tool | 职责 |
| --- | --- |
| `llm_chat` | 调用 LLM 进行对话推理 |
| `web_search` | 通过 DuckDuckGo 搜索并返回结构化结果 |
| `file_read` | 在限定目录下读取本地文件 |
| `file_write` | 在限定目录下写入本地文件，支持自动日期路径 |
| `http_request` | 发起外部 HTTP 请求 |
| `code_exec` | 执行 python/js/bash/powershell 代码 |
| `memory_read` | 读取记忆内容 |
| `memory_write` | 写入记忆内容 |

### 2.4 现有优点

1. 工具接口统一，新增 Tool 简单。
2. ToolManager 具备集中注册、查询、执行能力。
3. `ExecuteForSkill` 已经接入 Skill 工具白名单。
4. `file_read` / `file_write` 有 `baseDir` 限制，可防止基础路径逃逸。
5. 工具执行有结构化日志，便于排查失败。
6. `MemoryStore` 被抽象出来，后续可以替换成数据库、Redis、向量库等实现。

### 2.5 现有不足

1. 所有工具都集中在 `toolkit.go` 中，文件过大，维护成本会越来越高。
2. 工具参数缺少 schema，只能依赖 `readString` 手工读取。
3. 工具返回值没有统一 schema，Worker、Task Detail、前端展示都只能按约定解析。
4. `code_exec` 目前主要依赖超时控制，还不是安全沙箱。
5. `http_request` 没有域名、IP、内网访问限制，未来存在 SSRF 风险。
6. `web_search` 固定依赖 DuckDuckGo，没有 provider 抽象。
7. Tool 权限粒度目前只到 skill-tool 白名单，还没有参数级、路径级、网络级策略。

## 3. Skills 现有架构

核心实现位于：

- `wukong/pkg/skills/skillsengine.go`

### 3.1 核心模型

当前 Skill 结构如下：

```go
type Skill struct {
    SkillName   string
    Description string
    Version     string
    Enabled     bool
    Params      []Param
    Tools       []string
    Execute     string
    Template    string
    Memory      MemoryConfig
    SourcePath  string
}
```

Skill 的定位不是一个单纯函数，而是一个综合能力声明：

- 参数定义
- 工具权限
- 可执行脚本入口
- prompt template 引用
- memory 策略
- 元数据来源路径

### 3.2 Registry 职责

`skills.Registry` 当前负责：

- 启动加载：`Start`
- 停止监听：`Stop`
- 周期轮询：`watch`
- 从磁盘加载 `SKILL.md`
- 解析 Skill 元数据
- 保存到内存 map
- 同步到数据库 `MetaStore`
- 提供 `List / Get / CanUseTool / ExecuteWithParams`

### 3.3 Skill 来源

当前 Skill 有两类来源：

1. 内置 Skill：
   - `chat`
   - `web_search`
   - `report_gen`

2. 文件系统 Skill：
   - 从 `skills/<skill_name>/SKILL.md` 加载
   - 支持解析 `Description / Params / Tools / Execute / Template / Memory Config`

### 3.4 Skill 执行

`Registry.ExecuteWithParams(...)` 支持执行脚本型 Skill：

```go
func (r *Registry) ExecuteWithParams(ctx context.Context, skillName string, params map[string]any) (map[string]any, error)
```

执行时会：

1. 读取 Skill 元数据
2. 检查 Skill 是否存在和启用
3. 根据 `Execute` 找到脚本文件
4. 把 `SKILL_NAME`、`SKILL_PARAMS` 注入环境变量
5. 按脚本扩展名选择执行器：
   - `.py`
   - `.sh`
   - `.ps1`

### 3.5 现有优点

1. Registry 是线程安全的。
2. 支持热加载，适合插件化开发。
3. `CanUseTool` 已经形成 Skill 到 Tool 的权限边界。
4. Skill 元数据可以同步到数据库，支持前端 `/skills` 页面展示。
5. `ExecuteWithParams` 支持脚本型 Skill 执行。
6. Skill 模型已经预留了 `Template` 和 `MemoryConfig`，可继续接入 PromptEngine 与 Memory 系统。

### 3.6 现有不足

1. `SKILL.md` 解析偏简陋，依赖固定 Markdown section 和正则。
2. 中文标记存在乱码迹象，例如“必填 / 默认值”的解析逻辑显示异常，需要统一编码。
3. `Template` 字段尚未和 `pkg/prompt` 深度打通。
4. Skill 执行只是直接运行脚本，没有真正沙箱、权限白名单、资源限制。
5. Skill 的磁盘元数据与数据库元数据之间的一致性策略还不完整。
6. 缺少输入/输出 schema，任务规划和前端表单都无法强类型生成。

## 4. Tools 与 Skills 的运行流程

Task 执行时，Tools 与 Skills 的典型链路如下：

```text
Task
  -> Manager 规划 SubTask
  -> Worker 执行 SubTask
  -> resolveSkillName / resolveToolName
  -> SkillAwareActionExecutor
  -> 优先 ToolManager.ExecuteForSkill
  -> 如果 skill 有 Execute 脚本，则 Registry.ExecuteWithParams
  -> 否则 fallback 到 LLMActionExecutor / ReActExecutor
```

ReAct 执行链路如下：

```text
Worker ReActExecutor
  -> PromptBuilder 构造 worker/react prompt
  -> LLM 返回 action/tool/final_answer
  -> ToolManager.ExecuteForSkill 校验 skill tool 权限
  -> 执行 tool
  -> observation 回填给 LLM
  -> 直到 final 或超过最大轮数
```

这个设计已经具备 agentic workflow 的基础形态：

- Skill 决定能力边界
- Tool 提供可执行动作
- ReAct 决定动态调用顺序
- Worker 负责执行闭环

## 5. 架构问题总结

### 5.1 Tool 粒度已经形成，但治理能力不足

Tool 已经是清晰的执行单元，但缺少：

- 参数 schema
- 结果 schema
- 风险等级
- 权限策略
- 审计字段
- provider 抽象

如果后续工具数量增长，仅靠 `map[string]any` 会让调用链越来越脆。

### 5.2 Skill 模型方向正确，但 manifest 不够稳定

Skill 当前通过 Markdown 解析，适合人读，但不适合作为长期机器协议。

未来 Skill 应逐步演进为：

- Markdown 用于说明
- YAML / JSON / frontmatter 用于机器解析

### 5.3 安全能力需要前置升级

目前最需要重点关注的高风险能力：

- `code_exec`
- Skill 脚本执行
- `http_request`
- `file_read`
- `file_write`

这些能力如果未来开放给用户或外部生成的 Skill，必须加入更强的沙箱与策略控制。

### 5.4 Prompt / Skill / Tool 还没有完全闭环

当前 `Skill.Template` 已经存在，但和 `pkg/prompt`、`pkg/promptbuilder` 的关系还不够强。

理想关系应是：

```text
Skill Manifest
  -> Params / Tools / Template / Memory
  -> ContextEngine 获取上下文
  -> PromptEngine 渲染模板
  -> PromptBuilder 生成 LLM messages
  -> Worker/ReAct 调用 Tools
```

## 6. 未来功能演进方向

### 6.1 拆分目录结构

建议将 `pkg/tool/toolkit.go` 拆成：

```text
pkg/tool/
  manager.go
  types.go
  options.go
  memory_store.go
  path.go
  tools/
    llm.go
    web_search.go
    file_read.go
    file_write.go
    http.go
    code_exec.go
    memory.go
```

建议将 `pkg/skills/skillsengine.go` 拆成：

```text
pkg/skills/
  registry.go
  types.go
  parser.go
  executor.go
  builtin.go
  watcher.go
  store.go
```

拆分目标不是制造更多文件，而是让职责更清楚：

- manager 只管注册和执行分发
- tool 实现独立维护
- parser 和 executor 分开
- builtin 与磁盘加载分开

### 6.2 给 Tool 增加 Schema

建议将 Tool 接口演进为：

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() ToolSchema
    OutputSchema() ToolSchema
    RiskLevel() RiskLevel
    Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}
```

收益：

- Planner 能知道怎么调用工具
- ReAct 能生成更准确的 tool params
- 前端能自动生成工具表单
- 后端能做参数校验
- 安全策略能按风险等级控制

### 6.3 强化 Skill Manifest

建议保留 `SKILL.md` 给人阅读，同时增加 `skill.yaml` 或 Markdown frontmatter：

```yaml
name: report_gen
version: 1.0.0
enabled: true
tools:
  - llm_chat
  - file_write
params:
  - name: topic
    type: string
    required: true
memory:
  type: long_term
  window: 20
```

这样机器解析不再依赖 Markdown 段落格式，也能更稳定地支持前端编辑和数据库同步。

### 6.4 接入 PromptEngine

Skill 的 `Template` 字段应逐步变成 PromptEngine 模板来源：

```text
Skill.Template
  -> PromptEngine.Register
  -> PromptBuilder.ForScene(skill_name)
  -> Worker / ReAct 使用统一消息构建
```

这样 Skill 不只是“脚本插件”，还可以成为：

> prompt + tools + context + memory + executor 的完整 Agent 能力单元。

### 6.5 做安全沙箱

重点增强：

- 工作目录隔离
- 文件读写白名单
- 网络访问白名单/黑名单
- 禁止访问内网地址
- 超时、内存、CPU 限制
- 执行日志与审计
- 高风险 tool 二次确认或策略拦截

特别是 `code_exec` 和 Skill 脚本执行，不能长期停留在“直接 exec + timeout”的阶段。

### 6.6 统一 Tool / Skill 执行结果

建议定义标准结果模型：

```go
type ToolResult struct {
    ToolName  string
    Status    string
    Output    any
    Error     string
    Metadata  map[string]any
}
```

Skill 也可以定义类似结果：

```go
type SkillResult struct {
    SkillName string
    Status    string
    Output    any
    Error     string
    ToolCalls []ToolResult
    Metadata  map[string]any
}
```

收益：

- Worker 更容易聚合结果
- Task Detail 更容易展示执行过程
- Stream 事件更稳定
- 前端不再依赖散乱字段

### 6.7 Skill Registry 化

当前 Skill 来自本地目录，未来可以演进为多来源：

- 本地 Skill
- 数据库 Skill
- Git Skill
- 远程 Skill Registry
- 用户自定义 Skill
- AI 生成 Skill

但所有来源最终都应统一注册进 `skills.Registry`，并统一经过权限、schema、版本、启停状态校验。

## 7. 推荐优先级

### 7.1 短期优先

1. 拆分 `pkg/tool` 和 `pkg/skills` 文件结构。
2. 给 Tool 增加参数 schema。
3. 修复 Skill parser 中的中文编码和“必填 / 默认值”解析问题。
4. 把 `Skill.Template` 接入 PromptEngine。
5. 给 `http_request / code_exec / file_read / file_write` 增加更严格安全策略。

### 7.2 中期优先

1. Skill manifest 从纯 Markdown 解析升级到 YAML/frontmatter。
2. Tool result 标准化。
3. ReAct tool call 协议标准化。
4. Skill 执行沙箱化。
5. 工具调用审计事件入库。

### 7.3 长期方向

1. Skill marketplace。
2. 多租户 Skill 权限。
3. 可视化 Skill 编排。
4. Tool / Skill 调用审计与安全评分。
5. 基于 Skill Schema 自动生成前端配置表单和 Planner tool-use 提示词。

## 8. 结论

Wukong 当前 Tools 与 Skills 体系已经具备基础 agent 执行能力：

- Skill 描述能力和权限
- Tool 执行原子动作
- Worker 负责运行时调度
- ReAct 支持多轮动态工具调用

下一阶段最重要的不是继续堆更多工具，而是补齐治理能力：

- schema
- manifest
- sandbox
- policy
- audit
- prompt/context 接入

当这些能力补齐后，Wukong 的 Skill 就可以从“本地脚本插件”升级为真正的“可治理、可组合、可观察的 Agent 能力单元”。

