# Wukong 当前实现架构分析

> 本文基于 `docs/design_old/wukong_ai_agent_arch_docs.md`、`docs/design_old/wukong_api_arch.md` 两份旧设计文档，以及当前 `wukong` 后端与 `wukong-frontend` 前端代码实现整理。旧设计文档描述的是目标架构，当前代码已经落地了其中的核心主链路：认证、Chat、Task、Manager 调度、WorkerPool 执行、LLM/Tool/Skill、Memory、Stream、Repository/DB、React 前端控制台。

## 1. 总体架构定位

Wukong 当前实现是一个 Go + React 的 AI Agent 任务执行系统。后端以 Gin API 为入口，围绕 `Manager` 做任务编排，`WorkerPool` 做子任务执行，LLM/Tool/Skill 做智能能力与工具能力，PostgreSQL 做持久化，SSE/WebSocket 做实时过程反馈。前端提供登录、对话、任务中心、技能列表和记忆查看等页面。

整体架构可以分为七层：

```text
Frontend React Console
  - Login / Chat / Tasks / Skills / Memory
  - REST API + SSE

Gin API & Handler
  - AuthHandler / ChatHandler / TaskHandler / SkillHandler / MemoryHandler / StreamHandler

Application Service
  - ChatService / TaskService / StreamService / SkillService / MemoryService

Agent Orchestration
  - Manager / Planner / StateMachine / Queue / AsyncDB Writer

Execution Runtime
  - WorkerPool / SubTaskExecutor / ReActExecutor

Capability Layer
  - LLM Provider / Tool Manager / Skills Registry / Memory Manager

Persistence & Infra
  - PostgreSQL / Repository / Config / JWT / Logger
```

## 2. 后端核心模块

### 2.1 API 路由与 Handler 层

入口文件是 `wukong/cmd/server/main.go`，路由组装在 `wukong/internal/route/router.go`。

当前 API 前缀是 `/api/v1`，主要模块如下：

| 模块 | 路由 | 作用 |
| --- | --- | --- |
| Auth | `/auth/login`, `/auth/logout`, `/auth/profile` | 登录、登出、用户信息 |
| Chat | `/chat/session/create`, `/chat/session/list`, `/chat/message/send`, `/chat/message/list`, `/chat/session/delete` | 会话与消息管理 |
| Task | `/task/create`, `/task/list`, `/task/detail`, `/task/cancel`, `/subtask/list` | 任务提交、查询、取消、子任务查看 |
| Skill | `/skill/list` | 技能列表 |
| Memory | `/memory/working/list`, `/memory/long/list` | 工作记忆、长期记忆查询 |
| Stream | `/stream/chat`, `/stream/task`, `/stream/ws/task` | Chat/Task 实时事件订阅 |
| Health | `/healthz` | 健康检查 |

Handler 层负责参数绑定、JWT 用户身份提取、调用 Service，并使用统一响应结构返回数据。

### 2.2 Service 层

Service 层是 API 与核心领域对象之间的适配层：

| Service | 文件 | 作用 |
| --- | --- | --- |
| `ChatService` | `internal/service/chat_service.go` | 创建会话、保存消息、调用 LLM 生成回复、发布 Chat SSE |
| `TaskService` | `internal/service/task_service.go` | 调用 Manager 创建任务、查询任务、取消任务、注入任务指令 |
| `StreamService` | `internal/service/stream_service.go` | 维护 SSE 订阅、事件缓冲、断线补发、可选 DB 持久化 |
| `StreamAppService` | `internal/service/stream_app_service.go` | 封装 Chat/Task 订阅与 WebSocket 命令处理 |
| `SkillService` | `internal/service/skill_service.go` | 聚合 DB 技能元数据与运行时 Skill Registry |
| `MemoryService` | `internal/service/memory_service.go` | 对外暴露记忆查询 |

### 2.3 Manager 调度中枢

`pkg/manager/manager.go` 是任务系统核心。它承担旧设计文档中 Manager 的大部分职责：

1. 接收主任务：`CreateTask` 创建 `Task`，写入 DB，放入内存队列。
2. 状态流转：通过 `TaskStateMachine` 管理 `PENDING -> PLANNING -> RUNNING -> COMPLETED/FAILED/CANCELLED`。
3. 任务规划：通过 `TaskPlanner` 把主任务拆成 `SubTask` DAG。
4. 子任务调度：扫描依赖满足的子任务，提交给 `WorkerPool`。
5. 结果回调：Worker 完成子任务后回调 `onSubTaskResult`。
6. DAG 推进：某个子任务成功后继续分发其下游子任务。
7. 结果聚合：所有子任务终态后，合并结果并更新主任务。
8. 实时事件：状态、规划、工具、输出通过 `StreamService` 推送给前端。
9. 崩溃恢复：启动时从 DB 加载未完成任务重新入队。
10. 异步落库：通过 `asyncdb.Writer` 批量/异步写任务、子任务、执行日志。

### 2.4 Planner 规划器

规划接口在 `pkg/manager/planner.go`：

```go
type TaskPlanner interface {
    Name() string
    PlanSubTasks(ctx context.Context, task *Task) ([]SubTaskDef, error)
}
```

当前有两类规划器：

| 规划器 | 文件 | 行为 |
| --- | --- | --- |
| `TplPlanner` | `pkg/manager/tpl_planner.go` | 模板式规划，作为兜底 |
| `LLMPlanner` | `pkg/manager/llm_planner.go` | 调用 LLM 生成 JSON DAG，失败后降级到模板规划 |

`LLMPlanner` 会向前端发布 `THINK`、`TOOL`、`STATUS` 类规划事件，让任务面板能看到“正在规划”“规划完成”等过程。

### 2.5 Queue 队列

队列实现在 `pkg/queue/quadtree.go`。旧文档称“四叉树队列”，当前代码实现更接近“按优先级分桶 + 每个优先级内按 `ExecuteAt` 维护小顶堆”的内存优先级队列。

核心能力：

1. `Push`：按 `TaskID` 幂等入队，优先级限制在 1-10。
2. `Pop`：从高优先级到低优先级扫描，取出到期任务。
3. `Remove`：取消未执行任务。
4. `SubmitDelay` 相关支持延迟任务与重试退避。

当前队列是内存队列，DB 负责恢复来源，不是运行时持续轮询调度来源。

### 2.6 WorkerPool 与子任务执行

`pkg/worker/pool.go` 实现 Worker 协程池：

1. 启动固定数量 worker goroutine。
2. 从内部队列取 `queue.Task`。
3. 使用状态机管理子任务执行态。
4. 为每个子任务设置超时 context。
5. 捕获 panic。
6. 失败后按平方退避重试。
7. 超过重试次数后回调 Manager 标记失败。
8. 成功后提取 `SubTask.Result` 并回调 Manager。

具体执行逻辑在 `pkg/worker/exec_subtask.go`：

| 执行器 | 作用 |
| --- | --- |
| `LLMActionExecutor` | 直接构造 Prompt 调 LLM 完成子任务 |
| `ToolActionExecutor` | 调用 ToolManager 中的工具 |
| `CompositeActionExecutor` | 先工具，失败后 LLM 兜底 |
| `SkillAwareActionExecutor` | 根据 action/params 决定走 tool、skill 或 fallback |
| `ReActExecutor` | 多轮 Thought -> Tool -> Observation -> Final Answer |

当前 `main.go` 中注入的是 `NewRoutedSubTaskHandlerWithTools`，默认执行器偏向 ReAct，并为 `web_search`、`report_gen` 等 action 提供特殊路由。

### 2.7 LLM Provider

`pkg/llm` 提供统一 LLM 抽象，支持 DeepSeek、豆包、OpenAI 兼容接口、Ollama 等 Provider。`main.go` 根据配置创建默认 Provider，并可开启 ProviderPool：

1. primary 优先调用。
2. fallback1/fallback2 可配置。
3. 支持失败阈值、冷却、重试退避。

LLM 被两条链路使用：

1. ChatService 直接调用 LLM 生成对话回复。
2. Manager 的 LLMPlanner 和 Worker 的 ReAct/LLM executor 调用 LLM 做规划与执行。

### 2.8 Tool 与 Skill

Skill Registry 在 `pkg/skills/skillsengine.go`，启动时读取配置的 `skills.root_dir`，周期性扫描技能目录，并可把技能元数据同步到 `skill_meta`。

Tool Manager 在 `pkg/tool/toolkit.go`，把 LLM、Skill Registry、Memory Manager 注入为工具执行上下文。Worker 执行子任务时可以：

1. 根据 skill/action 找工具。
2. 调用 `ExecuteForSkill`。
3. 如果工具不可用，再降级到 LLM 执行。

当前这层已经具备插件化/工具化雏形，但技能沙箱、安全权限、动态隔离等旧文档里的高级能力还属于后续增强方向。

### 2.9 Memory

Memory 相关代码在 `pkg/memory` 和 `internal/service/memory_service.go`。

数据库表包括：

1. `chat_memory`：会话级记忆。
2. `memory_working`：任务级短期工作记忆。
3. `memory_long_term`：长期记忆。
4. `memory_shared`：共享记忆。

当前 `main.go` 使用 `memory.NewManager(nil, nil)` 初始化内存管理器，API 层已有查询入口，ToolManager 也预留了记忆依赖。也就是说，Memory 的结构和接口已经在，深度写入、压缩、RAG 检索等能力还不是主执行链路的强依赖。

### 2.10 Stream 实时反馈

`StreamService` 是当前前后端体验的关键模块。

事件类型：

| 类型 | 含义 |
| --- | --- |
| `THINK` | LLM/Agent 思考、规划过程 |
| `TOOL` | 工具调用或工具错误 |
| `CHUNK` | 可展示文本输出 |
| `STATUS` | 任务状态变更或阶段状态 |
| `FINISH` | 流结束 |

实现特点：

1. 以 `chat:{sessionID}` 和 `task:{taskID}` 作为 stream key。
2. 每条消息带 `seq`。
3. 前端重连时携带 `last_seq`，后端补发历史消息。
4. 有内存 buffer，配置 DB 时也会写入 `stream_message`。
5. Task 支持 SSE 和 WebSocket；Chat 当前使用 SSE。

### 2.11 Repository 与数据库

数据库建表脚本在 `wukong/scripts/schema.sql`。核心表与旧设计基本一致：

| 表 | 作用 |
| --- | --- |
| `users` | 用户认证 |
| `chat_session` | 会话 |
| `chat_message` | 消息 |
| `chat_memory` | 会话记忆 |
| `task_info` | 主任务 |
| `task_sub` | 子任务 DAG |
| `memory_working` | 工作记忆 |
| `memory_long_term` | 长期记忆 |
| `memory_shared` | 共享记忆 |
| `task_exec_log` | 执行日志 |
| `stream_message` | 流式消息 |
| `skill_meta` | 技能元数据 |

Repository 层负责对象与 JSONB 字段之间的转换。Manager 在内存中保留 `taskCache` 与 `subTaskCache`，减少执行过程中反复查 DB。

## 3. 前端核心模块

前端位于 `wukong-frontend`，技术栈是 Vite + React + TypeScript + Tailwind + shadcn 风格基础组件。

### 3.1 API 与 SSE 客户端

`src/lib/api.ts` 封装 REST 请求：

1. `API_BASE` 默认 `http://localhost:8080`。
2. token 存在 `localStorage` 的 `wukong:token`。
3. 请求自动携带 `Authorization: Bearer token`。
4. 兼容后端 snake_case 与前端 camelCase。
5. 封装 auth、chat、task、skill、memory API。

`src/lib/sse.ts` 封装 EventSource：

1. 自动带 `last_seq`。
2. 由于 EventSource 不方便设置 Header，token 通过 `access_token` query 传入。
3. 监听大小写两套事件名。
4. 断线指数退避重连。
5. 收到事件后把最新 seq 写入 localStorage。

### 3.2 应用状态

`src/store/use_app_store.ts` 管理全局业务状态：

1. Chat session 列表。
2. 当前 session。
3. 会话消息。
4. 任务列表。
5. 当前任务。
6. 每个任务的 stream events。

页面直接通过 store 发起加载、追加消息、更新任务状态。

### 3.3 Chat 页面

`src/features/chat/chat_page.tsx` 实现对话界面：

1. 没有当前会话时，发送消息前自动创建会话。
2. 用户提交后，本地先追加 user 消息和空 assistant 消息。
3. 调用 `/api/v1/chat/message/send`。
4. 同时订阅 `/api/v1/stream/chat?sessionId=...`。
5. 收到 `CHUNK` 后做打字机式增量展示。
6. 收到 `FINISH` 后停止打字机并清空临时 buffer。

当前 Chat 是“同步生成 + SSE 推送结果”的轻量链路，不经过 Manager/Worker/Task DAG。

### 3.4 Tasks 页面

`src/features/tasks/tasks_page.tsx` 实现任务中心：

1. 任务列表页加载 `/api/v1/task/list`。
2. 通过侧边 Sheet 填写 skill、priority、prompt 创建任务。
3. 创建后跳转 `/tasks/:taskId`。
4. 详情页调用 `/api/v1/task/detail` 获取主任务与子任务。
5. 使用 ReactFlow 展示子任务 DAG。
6. 订阅 `/api/v1/stream/task?taskId=...`。
7. `STATUS` 更新任务状态轨迹。
8. `THINK`/`TOOL`/`CHUNK`/`FINISH` 展示实时执行面板。
9. `CHUNK` 累积到最终结果区域。
10. `FINISH` 后重新拉取详情，展示持久化后的最终结果。

## 4. Chat 流程

Chat 当前流程如下：

```text
用户在 ChatPage 输入消息
  -> 若无 session，前端调用 createSession
  -> 前端本地追加 user 消息与空 assistant 消息
  -> POST /api/v1/chat/message/send
  -> ChatHandler.SendMessage
  -> ChatService.SendMessage
      1. 校验 session 属于当前用户
      2. 写入 user chat_message
      3. 如果没有 skillName，直接调用 LLMProvider.Chat
      4. 写入 assistant chat_message
      5. PublishChat(CHUNK, reply)
      6. PublishChat(FINISH)
  -> 前端 SSE 收到 CHUNK
  -> 更新最后一条 assistant 消息
  -> 前端 SSE 收到 FINISH
  -> 完成本轮展示
```

重要特点：

1. Chat 不创建 `task_info`。
2. Chat 不进入 Manager/Planner/WorkerPool。
3. ChatService 目前只把单轮用户输入发给 LLM，没有从 `chat_message` 自动组装多轮上下文。
4. `skillName` 传空时走 LLM；传非空时当前逻辑会返回“收到您的消息”式默认回复，不会进入技能任务链路。
5. Chat 的实时流用于前端展示，主要事件是 `CHUNK` 和 `FINISH`。

## 5. Task 流程

Task 是当前 AI Agent 架构主流程。

### 5.1 创建任务

```text
用户在 TasksPage 提交 skillName / priority / prompt
  -> POST /api/v1/task/create
  -> TaskHandler.CreateTask
  -> TaskService.CreateTask
  -> Manager.CreateTask
      1. 生成 task_id
      2. 创建 Task，状态 PENDING
      3. 异步/同步持久化 task_info
      4. 写入 taskCache
      5. 初始化状态机
      6. 推入 Manager 主任务队列
  -> API 返回 task_id
  -> 前端跳转任务详情页并订阅 task SSE
```

### 5.2 Manager 调度与规划

```text
Manager.Start
  -> 启动 WorkerPool
  -> loadPendingTasks 从 DB 恢复未完成任务
  -> 每 1 秒 scheduleLoop

scheduleLoop
  -> Pop 主任务队列
  -> PENDING -> PLANNING
  -> planner.PlanSubTasks
      - 优先 LLMPlanner
      - 失败降级 TplPlanner
  -> 为每个 step 创建 task_sub
  -> PLANNING -> RUNNING
  -> dispatchReadySubTasks
```

规划结果是一个 DAG。每个 `SubTask` 有：

1. `sub_task_id`
2. `task_id`
3. `action`
4. `params`
5. `depends_on`
6. `status`

### 5.3 子任务分发

```text
dispatchReadySubTasks(taskID)
  -> 读取该主任务所有 SubTask
  -> 找到 status=PENDING 且 depends_on 全部 SUCCESS 的节点
  -> submitSubTask
      1. 子任务状态置 RUNNING
      2. 更新 DB 与缓存
      3. 封装 queue.Task
      4. 提交到 WorkerPool
```

如果某个子任务依赖上游，则只有上游全部成功后才会被提交。

### 5.4 Worker 执行

```text
WorkerPool worker goroutine
  -> 从 WorkerPool 队列 Pop 子任务
  -> PENDING -> RUNNING
  -> 带 timeout 调用 TaskHandler
  -> SubTaskExecutor.Handle
      1. 根据 action 选择执行器
      2. 可走 ReActExecutor / ToolActionExecutor / LLMActionExecutor
      3. 写入 subTask.Result
  -> 成功：RUNNING -> COMPLETED，resultCb(success=true)
  -> 失败：按退避重试
  -> 超过重试：RUNNING -> FAILED，resultCb(success=false)
```

当前默认执行器链路：

1. `ReActExecutor` 让 LLM 输出 JSON：thought/action/tool_name/tool_params/final_answer。
2. 如果 action 是 `tool`，通过 ToolManager 调用工具。
3. 把 Observation 继续喂给 LLM。
4. 得到 final answer 后返回 `output` 和 `react_steps`。
5. Manager 把 `react_steps` 转成 `THINK`/`TOOL` 流事件，把 `output` 转成 `CHUNK`。

### 5.5 结果回调、DAG 推进与聚合

```text
WorkerPool.fireResultCb
  -> Manager.onSubTaskResult
      1. 找到 SubTask
      2. 更新 SUCCESS/FAILED
      3. 持久化 task_sub
      4. emitResultEvents 发布 THINK/TOOL/CHUNK
      5. 如果成功，继续 dispatchReadySubTasks
      6. tryAggregateTask

tryAggregateTask
  -> 如果仍有 PENDING/RUNNING 子任务：等待
  -> 如果任一 FAILED：主任务 RUNNING -> FAILED
  -> 如果全部 SUCCESS/SKIPPED：
      1. aggregateResults
      2. 写入 task.Result
      3. RUNNING -> COMPLETED
      4. 发布 FINISH
```

最终 `task_info.result` 是按子任务 action/sub_task_id 合并后的 map，并附带 `_summary`、`_completed_at`、`_subtask_count`。

### 5.6 Task 实时反馈

Task 过程中会向 `task:{taskID}` stream 发布事件：

| 阶段 | 事件 |
| --- | --- |
| 任务状态变化 | `STATUS {"status":"PLANNING","reason":"scheduling"}` |
| LLM 规划 | `THINK` |
| 规划失败/工具信息 | `TOOL` |
| 子任务执行输出 | `CHUNK` |
| 子任务 ReAct 思考 | `THINK` |
| 子任务工具调用 | `TOOL` |
| 任务终态 | `FINISH` |

前端任务详情页消费这些事件，形成状态轨迹、实时面板和最终结果展示。

## 6. Chat 与 Task 的区别

| 维度 | Chat | Task |
| --- | --- | --- |
| 入口 | `/chat/message/send` | `/task/create` |
| 主要服务 | `ChatService` | `TaskService + Manager` |
| 是否创建任务 | 否 | 是 |
| 是否规划 DAG | 否 | 是 |
| 是否经过 WorkerPool | 否 | 是 |
| LLM 调用位置 | ChatService 直接调用 | Planner 和 Worker 调用 |
| 持久化 | `chat_message` | `task_info`、`task_sub`、`task_exec_log` |
| 实时流 | `chat:{sessionID}` | `task:{taskID}` |
| 前端展示 | 对话气泡 | DAG、状态轨迹、执行事件、最终结果 |
| 适合场景 | 单轮/轻量问答 | 可拆解、可追踪、可重试的复杂任务 |

## 7. 当前实现与旧设计的对应关系

| 旧设计能力 | 当前状态 | 说明 |
| --- | --- | --- |
| Manager 调度中枢 | 已实现 | 任务创建、规划、调度、聚合、恢复均已落地 |
| Worker 执行单元 | 已实现 | 当前是进程内 goroutine worker pool，不是独立分布式节点 |
| 四叉树持久化队列 | 部分实现 | 运行时是内存优先级队列，DB 用于恢复；代码并非真正四叉树结构 |
| DAG 子任务 | 已实现 | `task_sub.depends_on` + Manager 依赖判断 |
| 状态机 | 已实现 | 主任务和 WorkerPool 均使用状态机 |
| LLM 规划 | 已实现 | `LLMPlanner`，失败降级模板 |
| ReAct 执行 | 已实现 | Worker 中 `ReActExecutor` |
| Tool/Skill 插件 | 部分实现 | Registry、ToolManager、技能元数据已具备，安全沙箱等还未完整落地 |
| Memory | 部分实现 | 表与 Manager 存在，主链路尚未深度依赖 |
| SSE/WebSocket | 已实现 | Chat/Task SSE，Task WebSocket 命令入口 |
| PostgreSQL 持久化 | 已实现 | Repository + schema.sql |
| 多 Worker 节点心跳 | 部分实现 | 有 WorkerRegistry 与清理逻辑，当前主流程仍是本进程内池 |
| 前端控制台 | 已实现 | Chat、Task、Skill、Memory 页面已落地 |

## 8. 当前架构的核心设计取舍

1. **API 与 Agent Runtime 解耦**  
   API 层只做请求入口，复杂任务进入 Manager。这样 Chat 可以轻量同步，Task 可以异步执行和追踪。

2. **主任务与子任务分离**  
   `task_info` 表示用户目标，`task_sub` 表示可执行步骤。Manager 只调度可执行节点，Worker 只关心单个子任务。

3. **规划和执行分离**  
   Planner 负责“怎么拆”，Worker 负责“怎么做”。LLM 可以同时参与规划和执行，但入口不同。

4. **内存调度 + DB 恢复**  
   调度热路径主要在内存中完成，DB 作为持久化和恢复来源，避免每轮调度都轮询数据库。

5. **事件流是第一等能力**  
   Manager/Worker 执行过程不断发布事件，前端不是只等最终结果，而是看到状态、思考、工具调用和输出。

6. **LLM/Tool/Skill 可插拔**  
   Worker 执行时根据 action 和 params 路由到不同执行器，为后续扩展技能系统、工具系统、沙箱系统留下接口。

## 9. 一次完整 Task 示例

以用户提交 `skillName=report_gen`、`prompt=生成某主题报告` 为例：

```text
1. 前端 TasksPage 调 POST /api/v1/task/create
2. TaskService 调 Manager.CreateTask
3. Manager 创建 task_***，状态 PENDING，入主任务队列
4. 前端跳转 /tasks/task_***，订阅 /stream/task
5. Manager scheduleLoop 取出任务
6. 状态 PENDING -> PLANNING，前端看到 STATUS
7. LLMPlanner 生成若干 step，例如 web_search -> report_gen
8. Manager 创建多个 task_sub，状态 PLANNING -> RUNNING
9. Manager 找到无依赖子任务，提交 WorkerPool
10. WorkerPool 执行 web_search 子任务
11. ReActExecutor 调 LLM 决策工具调用，ToolManager 执行 web_search
12. 子任务 SUCCESS，Manager 发布 THINK/TOOL/CHUNK
13. Manager 解锁依赖它的 report_gen 子任务
14. WorkerPool 执行 report_gen
15. 所有子任务完成后 Manager 聚合结果
16. 主任务 RUNNING -> COMPLETED
17. StreamService 发布 FINISH
18. 前端重新拉 task detail，展示最终结果与 DAG
```

## 10. 后续可演进点

1. **Chat 与 Task 融合**：支持 Chat 中指定 skill 后自动创建 Task，并把 task_id 关联回 chat_message。
2. **多轮上下文**：ChatService 调 LLM 前可读取历史 `chat_message` 和 `chat_memory`。
3. **真正分布式 Worker**：把当前进程内 WorkerPool 演进为多节点 Worker，通过心跳、租约、抢占避免重复执行。
4. **队列持久化增强**：明确当前内存队列与旧文档“四叉树持久化队列”的差异，补齐可恢复的延迟任务、重试任务状态。
5. **Memory 深度接入**：Worker/ReAct 执行前读取 working/long memory，执行后沉淀长期经验。
6. **Skill 沙箱与权限**：补充技能运行隔离、工具白名单、资源限制、审计。
7. **流式 LLM 真增量**：当前 Chat 是拿到完整 LLM 结果后推 `CHUNK`，可以升级为模型 token 级增量流。

