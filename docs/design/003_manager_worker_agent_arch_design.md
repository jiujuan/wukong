# 003 Manager 与 Worker Agent 架构设计分析

## 1. 文档目标

本文分析 `pkg/manager` 与 `pkg/worker` 两个目录的程序职责、核心架构、运行流程与当前设计特点，并给出面向后续演进的架构优化建议。

分析目标：

- 明确 `manager` 与 `worker` 的职责边界
- 梳理主任务、子任务、DAG、执行回调、结果聚合的完整链路
- 说明模板规划器、LLM 规划器、子任务执行器、ReAct 执行器各自所在位置
- 找出当前架构中可继续收敛和解耦的点
- 为后续 `PromptEngine / ContextEngine / PromptBuilder` 接入提供结构参考

---

## 2. 目录职责概览

### 2.1 `pkg/manager`

`pkg/manager` 是任务编排与调度中枢，负责：

- 创建主任务 `Task`
- 维护任务生命周期状态
- 调用 Planner 将主任务拆为 DAG 子任务
- 管理子任务缓存与持久化
- 找出可执行的 ready subtask
- 将子任务提交给 `worker.Pool`
- 接收 worker 执行结果回调
- 推进 DAG 下游继续执行
- 聚合所有子任务结果，生成主任务最终结果

可以把它理解为：

**“任务级 orchestration 层”**

### 2.2 `pkg/worker`

`pkg/worker` 是执行层，负责：

- 提供通用 worker pool
- 执行单个子任务
- 封装 LLM 执行、工具执行、ReAct 执行
- 提供重试、超时、结果回调、事件回调能力
- 管理 worker registry / heartbeat

可以把它理解为：

**“子任务级 execution 层”**

---

## 3. 模块结构分析

## 3.1 `pkg/manager` 文件结构

### `manager.go`

核心文件，包含：

- `Task`
- `SubTask`
- `TaskExecLog`
- `Manager`
- 任务生命周期管理
- DAG 调度逻辑
- 结果回调与聚合逻辑
- 持久化适配逻辑

这是 manager 模块的绝对主入口。

### `planner.go`

定义规划器抽象：

- `TaskPlanner`
- `SubTaskDef`
- `TaskTemplate`
- `TemplateStep`
- `WithPlanReporter`

它是 planner 层与 manager 主逻辑之间的协议文件。

### `tpl_planner.go`

模板规划器实现。

作用：

- 根据 `task.SkillName` 选择预定义模板
- 生成固定 DAG 步骤
- 适合作为默认规划器和回退规划器

### `llm_planner.go`

LLM 规划器实现。

作用：

- 使用 LLM 把主任务拆分为 DAG 步骤
- 将 LLM 输出 JSON 转换为 `SubTaskDef`
- 在 LLM 不可用或结果非法时降级到 `TplPlanner`

---

## 3.2 `pkg/worker` 文件结构

### `pool.go`

worker 执行池主实现，包含：

- `Pool`
- `TaskHandler`
- `ResultCallback`
- `TaskEventCallback`
- worker goroutine 运行逻辑
- 超时、重试、取消、回调机制

它是 worker 模块的基础设施层。

### `exec_subtask.go`

子任务执行路由层，包含：

- `PromptBuilder`
- `ActionPromptBuilder`
- `ActionExecutor`
- `LLMActionExecutor`
- `ToolActionExecutor`
- `CompositeActionExecutor`
- `SkillAwareActionExecutor`
- `SubTaskExecutor`

它负责把一个 `SubTask` 的 action 映射到具体执行器。

### `react.go`

ReAct 执行器实现。

作用：

- 基于 LLM 进行 tool / final 多轮推理
- 动态调用工具
- 收集 `react_steps`
- 输出最终结果

### `heartbeat.go`

worker 注册表与心跳信息。

作用：

- 保存可用 worker 元信息
- 维护负载、状态、技能、过期清理

当前主要用于 registry 级管理，不直接参与本地 `Pool` 的任务执行。

---

## 4. 核心数据结构与职责分层

## 4.1 Manager 侧核心结构

### `Task`

表示主任务。

字段重点：

- `TaskID`
- `UserID`
- `SessionID`
- `SkillName`
- `Params`
- `Status`
- `Priority`
- `Result`
- `Error`

### `SubTask`

表示 DAG 节点级子任务。

字段重点：

- `SubTaskID`
- `TaskID`
- `DependsOn`
- `Action`
- `Params`
- `Status`
- `Result`
- `Error`

同时它实现了 worker 执行层所需的轻量接口：

- `GetSubTaskID`
- `GetTaskID`
- `GetAction`
- `GetParams`
- `SetResult`
- `SetError`
- `SetUpdatedAt`

这意味着：

**`manager.SubTask` 同时也是 `worker` 的执行载体。**

### `Manager`

`Manager` 聚合了多个职责：

- 主任务队列 `taskQueue`
- 任务状态机 `stateMachine`
- worker 池 `workerPool`
- worker 注册表 `workerRegistry`
- planner
- 持久化仓储
- 异步写入器
- 内存缓存
- stream 事件上报

它本质上是一个“任务编排运行时”。

---

## 4.2 Worker 侧核心结构

### `Pool`

负责：

- 持有队列
- 启动 worker goroutine
- 调用 `TaskHandler`
- 处理执行超时
- 处理重试与重入队
- 回调 manager

### `TaskHandler`

```go
type TaskHandler func(ctx context.Context, task *queue.Task) error
```

它是 pool 与业务执行逻辑的解耦点。

### `SubTaskExecutor`

它是业务执行路由器：

- 根据 `subTask.Action` 选择 executor
- 默认走 LLM / ReAct
- 特定 action 可映射到专用 executor

### `ActionExecutor`

真正的执行策略接口：

- LLMActionExecutor
- ToolActionExecutor
- ReActExecutor
- CompositeActionExecutor
- SkillAwareActionExecutor

这层是当前 worker 目录里最有扩展性的抽象。

---

## 5. 程序运行主流程

下面按实际运行链路梳理一次。

## 5.1 主任务创建

入口：

- `Manager.CreateTask()`

流程：

1. 构造 `Task`
2. 写库
3. 写入 `taskCache`
4. 设置任务状态为 `PENDING`
5. 推入 `Manager.taskQueue`

结果：

- 主任务进入 manager 调度队列，等待后续规划

---

## 5.2 Manager 启动

入口：

- `Manager.Start()`

主要动作：

1. 启动 `workerPool.Start()`
2. 从数据库加载 pending/running/planning/waiting 任务
3. 启动 `scheduleLoop()`
4. 启动 `cleanWorkerLoop()`

说明：

- manager 是常驻调度器
- worker pool 是其下游执行引擎

---

## 5.3 主任务调度循环

入口：

- `scheduleLoop()`
- 每秒触发 `processTasks()`

流程：

1. 从主任务队列 `taskQueue` 取任务
2. 检查当前主任务状态
3. 若状态为 `PENDING`，先转到 `PLANNING`
4. 调用 `planTask()`
5. 规划完成后转到 `RUNNING`
6. 调用 `dispatchReadySubTasks()`

这个过程的关键点是：

**manager 只调度主任务，不直接执行子任务。**

---

## 5.4 任务规划

入口：

- `planTask(qTask)`

内部流程：

1. 从 `qTask.Data` 取出 `*Task`
2. 调用 `m.planner.PlanSubTasks(...)`
3. 得到 `[]SubTaskDef`
4. 逐个调用 `createSubTask(...)`
5. 持久化与缓存子任务

规划器有两类：

### 模板规划器 `TplPlanner`

特点：

- 基于固定模板
- 稳定
- 可预测
- 不依赖 LLM

### LLM 规划器 `LLMPlanner`

特点：

- 基于主任务参数与 skill 动态规划 DAG
- 能做更灵活的拆解
- 当 LLM 不可用或规划结果非法时自动降级模板规划

这一设计已经具备：

- 主规划器 + fallback 规划器
- 规划过程事件上报

这是 manager 目录里比较成熟的一层设计。

---

## 5.5 DAG 子任务分发

入口：

- `dispatchReadySubTasks(taskID)`

流程：

1. 读取该 task 的全部 subtasks
2. 遍历所有 `PENDING` 子任务
3. 用 `canExecute()` 判断依赖是否满足
4. 对 ready subtask 调用 `submitSubTask()`

`canExecute()` 规则：

- 子任务自身必须是 `PENDING`
- `DependsOn` 中所有子任务必须都是 `SUCCESS`

说明：

- 这是一个基于依赖状态扫描的 DAG 推进器
- 当前不是事件驱动图调度器，而是“回调触发 + 全量扫描”

---

## 5.6 子任务提交到 WorkerPool

入口：

- `submitSubTask(st)`

流程：

1. 子任务状态置为 `RUNNING`
2. 持久化状态
3. 更新 cache
4. 构造 `queue.Task`
5. `workerPool.Submit(qTask)`

若提交失败：

1. 子任务标记为 `FAILED`
2. 写库和缓存
3. 触发主任务聚合检查

说明：

- `Manager` 与 `Pool` 的边界在这里非常清楚
- manager 只负责“把 subtask 投给 worker”

---

## 5.7 WorkerPool 执行子任务

入口：

- `Pool.Submit()`
- worker goroutine `executeTask()`

执行流程：

1. 状态从 `PENDING -> RUNNING`
2. 创建带 timeout 的 `execCtx`
3. 调用注入的 `taskHandler`
4. 从 `task.Data` 中提取 result
5. 根据执行结果：
   - 成功：`RUNNING -> COMPLETED`
   - 失败但可重试：重入队
   - 失败且超重试：`RUNNING -> FAILED`
6. 调用 `resultCb`
7. 调用 `taskEventCb`

这里的几个关键能力：

- timeout 控制
- panic recover
- retry with backoff
- callback 回传 manager

这是 worker 层的核心基础设施价值。

---

## 5.8 子任务业务执行

入口：

- `SubTaskExecutor.Handle()`

流程：

1. 从 `queue.Task.Data` 取出 `executableSubTask`
2. 根据 `Action` 选择 executor
3. 执行 executor
4. 将结果写回 `subTask.SetResult(...)`
5. 清空 error 并更新时间

当前 action 执行链包含：

- 默认 LLM 执行
- web_search / report_gen 专用逻辑
- ReAct 执行
- Tool executor
- Skill-aware executor

也就是说，worker 这一层本质上是：

**“Action Router + Executor Runtime”**

---

## 5.9 ReAct 执行流程

入口：

- `ReActExecutor.Execute()`

流程：

1. 提取 skill、tool hint、allowed tools
2. 构造 system prompt + user prompt
3. 进入最多 `maxIterations` 次循环
4. 每轮 LLM 返回：
   - `tool`
   - `final`
   - 或普通文本 fallback
5. 若是 `tool`：
   - 调 `toolManager.ExecuteForSkill(...)`
   - observation 回写到消息列表
6. 若是 `final`：
   - 输出最终结果

这是一个典型的 ReAct loop。

优点：

- 工具调用链封装完整
- `react_steps` 可回传上层
- 与普通 LLMActionExecutor 分离

---

## 5.10 执行结果回流到 Manager

入口：

- `Manager.onSubTaskResult(...)`

流程：

1. 根据 `subTaskID` 找到缓存中的 `SubTask`
2. 更新 subtask 状态：
   - success -> `SUCCESS`
   - failed -> `FAILED`
3. 持久化与更新缓存
4. 上报执行事件
5. 若成功则继续 `dispatchReadySubTasks(taskID)`
6. 调用 `tryAggregateTask(taskID)`

这是 manager 与 worker 的关键交界点：

**worker 只负责完成一个 subtask，manager 负责决定 DAG 下一步。**

---

## 5.11 主任务结果聚合

入口：

- `tryAggregateTask(taskID)`

逻辑：

1. 读取 task 下全部 subtasks
2. 检查是否全部进入终态
3. 若有失败 subtask：
   - 主任务标记 `FAILED`
4. 若全部成功：
   - 聚合每个 subtask 的 `Result`
   - 写入主任务 `Result`
   - 主任务标记 `COMPLETED`

聚合策略：

- 默认按 `Action` 作为 key
- 若 action 重复，则用 `subTaskID` 防止覆盖
- 额外生成：
  - `_summary`
  - `_completed_at`
  - `_subtask_count`

这部分相当于：

**DAG 执行闭环的最终收口点**

---

## 6. 当前架构的优点

## 6.1 分层已经有雏形

虽然代码还比较集中，但实际上已经形成了三层：

- Manager：编排层
- Pool：执行基础设施层
- Executor：动作执行层

这是很好的基础。

## 6.2 Planner 有 fallback 机制

`LLMPlanner -> TplPlanner` 的降级策略很实用，说明架构已经考虑了不稳定外部依赖。

## 6.3 Worker Pool 基础设施比较完整

具备：

- timeout
- retry
- callback
- cancellation
- state transition

这使得执行层具备比较好的工程稳定性。

## 6.4 ReAct 与普通执行器已分离

`ReActExecutor` 独立存在，而不是直接塞进统一 handler 里，这为后续扩展 agent 风格执行器提供了空间。

## 6.5 主任务与子任务模型清晰

`Task -> SubTask -> Result Aggregation` 的链路结构明确，比较适合继续升级为更强的 agent runtime。

---

## 7. 当前架构的主要问题

## 7.1 Manager 过重

`Manager` 当前聚合了过多职责：

- 调度
- 状态管理
- 规划
- DAG 推进
- 结果聚合
- 缓存
- 持久化
- stream 事件
- worker registry 清理

这意味着：

- 文件过大
- 单元测试粒度偏粗
- 修改一处逻辑容易牵动多处

## 7.2 Manager 与 SubTask 执行载体耦合较深

`manager.SubTask` 直接实现了 worker 执行接口，这种方式短期高效，但长期会导致：

- manager 数据模型与 worker 运行时模型混在一起
- 执行层难以独立演化
- 后续如果有 remote worker / sandbox worker，会更难抽象

## 7.3 DAG 推进是“扫描式”而不是“图模型驱动”

当前 `dispatchReadySubTasks()` 每次都遍历全部 subtasks 判断依赖满足情况。

问题：

- subtasks 多时效率一般
- 没有显式 graph runtime
- 无法很自然支持更复杂的条件边、跳过规则、分支聚合

## 7.4 Planner 输出协议偏弱

当前 `SubTaskDef` 比较轻，但缺少更强的调度元信息，例如：

- timeout
- retry policy
- preferred executor
- expected output schema
- context requirements

这会限制后面将 planner 输出直接映射到更丰富的执行策略。

## 7.5 Prompt / Context 仍散落在执行器内部

目前：

- `llm_planner.go`
- `exec_subtask.go`
- `react.go`

都各自直接拼 prompt。  
这会导致后续接入 `PromptEngine / ContextEngine` 时需要多点替换。

## 7.6 状态机分散

当前有两套状态变化语义：

- manager 主任务状态
- worker pool 内部状态

但它们之间没有统一的 task runtime state model 文档或抽象层，后续维护时容易出现边界不一致。

## 7.7 WorkerRegistry 与本地 Pool 的关系偏弱

`WorkerRegistry` 当前主要是独立元数据注册结构，还没有真正参与：

- worker 选择
- skill 路由
- 调度决策

所以它目前更像“预留能力”，还没有真正成为调度系统的一部分。

---

## 8. 架构优化建议

下面按优先级给建议。

## 8.1 将 Manager 拆为多个子组件

建议把 `Manager` 内部能力拆成更清晰的组件：

### 建议拆分

- `TaskScheduler`
  - 主任务调度循环
- `TaskPlannerRuntime`
  - task -> subtasks
- `DAGDispatcher`
  - ready subtask 选择与提交
- `TaskAggregator`
  - 主任务结果聚合
- `TaskStoreCache`
  - cache + repo 封装

这样 `Manager` 最终只做 orchestrator facade。

### 收益

- 文件规模下降
- 可测试性提升
- 状态推进逻辑更清楚

---

## 8.2 引入独立的 ExecutionTask / RuntimeSubTask 模型

建议不要长期让 `manager.SubTask` 直接充当 worker 执行载体。

可以增加一层运行时对象：

```go
type RuntimeSubTask struct {
    SubTaskID string
    TaskID    string
    Action    string
    Params    map[string]any
    Metadata  map[string]any
}
```

Manager 负责：

- 持久化模型

Worker 负责：

- 运行时模型

这样更利于：

- remote worker
- sandbox executor
- 多执行后端

---

## 8.3 抽象 DAG Runtime

建议把 DAG 相关逻辑从 `Manager` 中抽成独立组件：

```go
type DAGRuntime interface {
    Ready(subtasks []*SubTask) []*SubTask
    OnComplete(subTaskID string)
    IsFinished(taskID string) bool
}
```

第一版不用复杂化，先把：

- 依赖判断
- ready 节点选择
- finished 判断

从 manager 中挪出来即可。

### 收益

- DAG 推进逻辑独立
- 后续支持条件节点、branch、join 更自然

---

## 8.4 统一 Planner 输出协议

扩展 `SubTaskDef`：

```go
type SubTaskDef struct {
    SubTaskID        string
    TaskID           string
    Action           string
    Params           map[string]any
    DependsOn        []string
    ExecutorType     string
    TimeoutSeconds   int
    MaxRetry         int
    ContextProfile   string
    ExpectedSchema   string
}
```

这样 planner 就不只是“拆几步”，而是“定义怎么执行这些步”。

---

## 8.5 将 PromptEngine / ContextEngine 接入 Planner 与 Worker

优先顺序建议：

1. `pkg/worker/exec_subtask.go`
2. `pkg/worker/react.go`
3. `pkg/manager/llm_planner.go`

接入方式：

- planner 使用 `PromptEngine + ContextEngine`
- worker action executor 使用 `PromptEngine`
- ReAct executor 使用 `PromptEngine + ContextEngine`

这样后续：

- prompt 统一管理
- context 统一装配
- chat / task / planner / worker 可以逐步共用能力

---

## 8.6 引入 PromptBuilder 作为 manager / worker 的共同消息构建入口

推荐新增：

- `pkg/promptbuilder/`

由它统一完成：

- scene -> context build
- template key -> prompt render
- output -> `[]llm.Message`

这样：

- planner 不再自己拼 system/user prompt
- worker 不再自己拼 action prompt
- react 也走统一入口

---

## 8.7 统一 Task Runtime 事件模型

建议定义标准事件类型，而不是散落字符串：

```go
type EventType string

const (
    EventStatus EventType = "STATUS"
    EventThink  EventType = "THINK"
    EventTool   EventType = "TOOL"
    EventChunk  EventType = "CHUNK"
    EventFinish EventType = "FINISH"
    EventError  EventType = "ERROR"
)
```

并定义统一 payload 结构：

```go
type TaskEvent struct {
    TaskID    string
    SubTaskID string
    Type      EventType
    Content   string
    Meta      map[string]any
    CreatedAt time.Time
}
```

这样 stream、log、UI、debug 都会更统一。

---

## 8.8 让 WorkerRegistry 真正进入调度链

当前 `WorkerRegistry` 还比较游离，建议未来二选一：

### 方向 A：删掉

如果只保留单机本地 pool，就不需要 registry 这层复杂度。

### 方向 B：做实

如果未来要支持分布式 worker，就让 registry 真正参与：

- skill 路由
- capacity-aware 调度
- remote worker 选择

当前建议：

**短期保留，但不要继续在本地单机路径上增加耦合。**

---

## 8.9 持久化适配进一步收口

当前 `Manager` 直接感知：

- repo
- async writer
- batch upsert repo

建议后面抽一层：

```go
type TaskPersistence interface {
    SaveTask(ctx context.Context, task *Task) error
    SaveSubTask(ctx context.Context, subtask *SubTask) error
    SaveTaskLog(ctx context.Context, log *TaskExecLog) error
}
```

这样 manager 不需要同时知道同步写、异步写、批量写三种策略。

---

## 9. 推荐的后续重构顺序

### 第一阶段：低风险收敛

1. 为 planner / worker / react 接入 `PromptEngine`
2. 提炼统一事件类型
3. 抽出 `TaskAggregator`

### 第二阶段：中等改造

1. 抽出 `DAGDispatcher`
2. 抽出 `TaskPersistence`
3. 扩展 `SubTaskDef`

### 第三阶段：结构升级

1. 接入 `ContextEngine`
2. 增加 `PromptBuilder`
3. 引入 runtime task model

### 第四阶段：远期能力

1. 评估 distributed worker
2. 决定 `WorkerRegistry` 是做实还是收缩
3. 演进为更完整的 agent runtime

---

## 10. 结论

`pkg/manager` 与 `pkg/worker` 目前已经形成了一个清晰的 Agent Runtime 雏形：

- `manager` 负责主任务编排与 DAG 推进
- `worker` 负责子任务执行与结果回流
- planner 负责 task -> subtask DAG
- executor 负责 action -> concrete execution
- aggregator 负责 subtask -> final task result

从运行路径上看，当前架构已经具备：

- 主任务调度
- DAG 子任务执行
- LLM / Tool / ReAct 执行能力
- 失败重试与回调闭环

最大的改进空间不在“补功能”，而在“继续把职责拆清楚”：

- 让 manager 更像 orchestrator
- 让 worker 更像 execution runtime
- 让 prompt/context 成为统一基础设施
- 让 planner / executor 输出协议更稳定

如果沿着这个方向继续收敛，Wukong 后续完全可以从“任务调度器 + LLM 执行器”演进成一套更完整的 agent orchestration runtime。

