**调用入口**

- `pool.taskHandler` 不是框架自动反射调用的，它是通过 `WithTaskHandler(...)` 在创建 WorkerPool 时注入的函数指针。注入点在 [main.go:L56-L68](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L56-L68)。
- 你的当前实现里，`taskHandler` 只是打印日志后 `return nil`，见 [main.go:L56-L60](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L56-L60)。所以现在“执行”本质是 no-op（不做真实子任务业务）。

**完整执行链路（从 API 到 taskHandler）**

- 创建任务接口 `POST /api/v1/task/create` 进入 [router.go:L82-L85](file:///d:/codeproject/mygo/wukong/wukong/internal/route/router.go#L82-L85) → [task.go:L31-L53](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/task.go#L31-L53)。
- Handler 调 `TaskService.CreateTask` → `Manager.CreateTask`，主任务进入 manager 的主队列，见 [task_service.go:L21-L29](file:///d:/codeproject/mygo/wukong/wukong/internal/service/task_service.go#L21-L29) 和 [manager.go:L186-L223](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L186-L223)。
- `Manager.Start()` 后会启动调度循环，每秒处理主任务，见 [manager.go:L135-L163](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L135-L163) 与 [manager.go:L339-L351](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L339-L351)。
- 主任务被 `planTask` 拆成子任务，再 `dispatchReadySubTasks` 分发到 WorkerPool：见 [manager.go:L391-L411](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L391-L411)、[manager.go:L418-L439](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L418-L439)、[manager.go:L458-L473](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L458-L473)。
- 子任务通过 `workerPool.Submit(...)` 入队后，被 worker goroutine 在循环里 `Pop()` 出来并执行，见 [pool.go:L286-L297](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L286-L297)。
- 真正调用点就在 `executeTask`：`execErr = pool.taskHandler(execCtx, task)`，见 [pool.go:L323-L363](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L323-L363)。

**`taskHandler` 在 `executeTask` 中的执行语义**
- 先做状态机 `PENDING -> RUNNING`，失败则跳过执行，见 [pool.go:L327-L335](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L327-L335)。
- 给 `taskHandler` 包一层超时 `context.WithTimeout(pool.execTimeout)`，默认 5 分钟，见 [pool.go:L341-L343](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L341-L343) 与 [pool.go:L83-L91](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L83-L91)。
- 有 panic recover 保护，`taskHandler` 崩了也不会炸掉 worker goroutine，见 [pool.go:L349-L356](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L349-L356)。
- 执行成功：状态置 `COMPLETED`，触发结果回调；失败：按重试次数指数退避重入队（1s/4s/9s...），超限后 `FAILED`，见 [pool.go:L371-L414](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L371-L414)。
- 业务结果不是返回值带回，而是从 `task.Data` 提取 `GetResult()`，见 [pool.go:L431-L444](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L431-L444)；`SubTask` 已实现该接口，见 [manager.go:L51-L54](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L51-L54)。

**你这个版本里要特别注意的点**
- 当前 `taskHandler` 仅日志+成功返回，所以子任务不会有真实业务产物（`Result` 通常为空）。
- `SetWorkerPool` 注释说“自动注入 resultCb”，但实际没有把回调注入传入的 pool，见 [manager.go:L120-L129](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L120-L129)。而 `main` 创建 pool 时也没显式 `WithResultCallback(...)`，见 [main.go:L62-L70](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L62-L70)。这会导致 Worker 完成后 Manager 收不到子任务回传，DAG 后续推进会受影响。  

如果你愿意，我下一步可以直接给你画一版“v0.2 实际运行时序图（API->Manager->Pool->Callback）”，并顺手把这处 result callback 注入缺口补上。



 修复上面出现的问题：



**已完成修补**

- 已把 Worker 的 `taskHandler` 从 no-op 改成真实执行 LLM 子任务逻辑：  
  - 从 `task.Data` 解析 `*manager.SubTask`  
  - 组装子任务 prompt 并调用 `llmProvider.Chat(...)`  
  - 将模型输出、token 统计、完成时间写回 `subTask.Result`，供 `pool.extractResult` 和 Manager 回调链路使用  
  - 代码位置：[main.go:L59-L93](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L59-L93)、[main.go:L144-L162](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L144-L162)

- 已补齐 `result callback` 自动注入缺失：  
  - `Manager.SetWorkerPool(...)` 现在会对传入的 pool 强制注入 `m.onSubTaskResult`，保证 Worker 完成后能回传给 Manager 继续 DAG 推进  
  - 代码位置：[manager.go:L123-L133](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L123-L133)

- 为了支持运行时注入回调，给 WorkerPool 增加了 setter：  
  - `SetTaskHandler` / `SetResultCallback`  
  - 代码位置：[pool.go:L264-L273](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L264-L273)

**现在的执行链路（修补后）**

- `/task/create` 创建主任务 → Manager 拆解子任务并提交到 WorkerPool  
- Worker `executeTask` 调用你在 `main.go` 注入的 `taskHandler`（现在会调用 LLM）  
- `taskHandler` 写入 `subTask.Result`  
- Worker 成功后触发 `resultCb`  
- Manager `onSubTaskResult` 收到回传，更新子任务状态并继续分发下游 DAG 节点

**验证结果**

- 已执行：`go test ./...`  
  - 失败，原因是仓库已有全局构建问题（与本次改动无关），例如 import path 混用 `wukong/...` 与模块路径、以及 `pkg/database` 的 `pgx.CommandTag` 编译错误。
- 已执行：`go test ./pkg/worker ./pkg/manager`  
  - `pkg/worker` 通过  
  - `pkg/manager` 因现有 `pkg/database` 编译问题失败（同上，非本次改动引入）

如果你愿意，我下一步可以继续把这两个“全局编译阻塞点”（import path 统一 + pgx 类型错误）一并修掉，这样你就可以直接全量 `go test ./...` 通过并完整联调 v0.2。

----



> 能不能把main.go 中59行 - 93行的处理taskHandler的代码以及它关联代码144行-162行的buildSubTaskMessages()方法，把这些代码封装到 pkg/worker/exec_subtask.go 一个新文件里，好处是让执行子任务调用更加容易，也容易修改，第二让 main.go 代码也更加简洁


可以，已经帮你完成了这次重构，`main.go` 已经明显简化。

**改动结果**
- 已把原来 `main.go` 里 taskHandler 和 `buildSubTaskMessages()` 迁移到新文件 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L1-L87)。
- 新增可复用构造函数 `worker.NewLLMSubTaskHandler(provider, logger)`，主程序只负责注入，不再关心子任务执行细节，见 [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L54-L61)。
- 为了避免 `worker -> manager` 包循环依赖，我给 `manager.SubTask` 增加了执行器所需方法（getter/setter），见 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L53-L77)。

**现在 main.go 的效果**
- `main.go` 不再有 59-93 的匿名 taskHandler 逻辑，也没有 144-162 的消息构建函数。
- 只保留一行注入：

```go
worker.WithTaskHandler(worker.NewLLMSubTaskHandler(llmProvider, log))
```

对应位置：[main.go:L55-L61](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L55-L61)

**验证**
- 通过：`go test ./pkg/worker`
- 未通过：`go test ./pkg/manager`（是仓库既有问题：`pkg/database` 里 `pgx.CommandTag` 编译错误，与这次重构无关）

如果你愿意，我可以下一步把 `exec_subtask.go` 再拆成“Prompt 构建策略 + Handler 执行器”两层，这样后续接 `web_search/report_gen` 等 action 分派会更清晰。



---

> 把 pkg/worker/exec_subtask.go 再拆成“Prompt 构建策略 + Handler 执行器”两层，这样后续接 web_search/report_gen 等 action 分派会更加清晰
>
> 

**重构已完成**

- 已按你要的方式把 `exec_subtask` 拆成两层：`Prompt 构建策略` + `Handler 执行器`，核心都在 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L1-L130)。
- `main.go` 保持简洁，仍然只注入一行 `worker.NewLLMSubTaskHandler(...)`，见 [main.go:L55-L61](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L55-L61)。

**两层结构**
- **Prompt 构建策略层**
  - `PromptBuilder` 接口：`BuildMessages(ctx, subTask)`，见 [exec_subtask.go:L25-L27](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L25-L27)。
  - `ActionPromptBuilder` 实现 action 分派（已内置 `web_search`、`report_gen`，其他走默认策略），见 [exec_subtask.go:L29-L67](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L29-L67)。
- **执行器层**
  - `SubTaskExecutor` 负责：校验任务数据、调用 PromptBuilder、调用 LLM、写回结果与状态，见 [exec_subtask.go:L69-L125](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L69-L125)。
  - `NewLLMSubTaskHandler` 只是把执行器适配成 `TaskHandler`，见 [exec_subtask.go:L127-L130](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L127-L130)。

**顺手修复**
- 你文件里出现的 `mgr.SetLogger(log.logger)` 会编译失败（`logger` 字段未导出），我已修成 `mgr.SetLogger(log.With())`，见 [main.go:L51-L53](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L51-L53)。

**验证**
- `go test ./pkg/worker` ✅ 通过
- `go test ./cmd/server` ❌ 失败（现有仓库问题：`internal/route/router.go` 里仍有 `wukong/...` 旧导入路径，非本次重构引入）

如果你下一步要接 `web_search/report_gen` 真正执行分支，我可以继续在 `SubTaskExecutor` 里做 action router（不同 action 走不同 Tool/LLM链路），并保留当前 PromptBuilder 作为默认兜底。



---

filePath: d:\codeproject\mygo\wukong\wukong\pkg\worker\exec_subtask.go

> 下一步如果要接 web_search/report_gen 等等真正执行分支，可以继续在 SubTaskExecutor 里做 action router（不同 action 走不同 Tool/LLM/Skills链路），并保留当前 PromptBuilder 作为默认兜底

已按你说的方向继续实现：`SubTaskExecutor` 现在已经有 **action router**，并保留了 **PromptBuilder 默认兜底**。

**本次改造**
- 在 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L119-L121) 新增 `ActionExecutor` 抽象，执行链路从“单一 LLM 调用”升级为“可路由执行器”。
- 新增 `LLMActionExecutor`（通用 LLM 执行器），作为默认执行器，见 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L123-L162)。
- 在 `SubTaskExecutor` 内新增 `actionExecutors` 路由表，按 action 分派执行，见 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L164-L212)。
- 路由已内置两个分支：
  - `web_search` → `WebSearchPromptBuilder`，见 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L71-L93)
  - `report_gen` → `ReportGenPromptBuilder`，见 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L95-L117)
- 兜底逻辑仍保留 `ActionPromptBuilder`，未知 action 继续走默认 PromptBuilder，见 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L170-L180) 和 [exec_subtask.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L189-L192)。

**兼容性**
- `main.go` 现有注入方式保持不变：`worker.NewLLMSubTaskHandler(...)` 仍可直接用，见 [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L55-L61)。
- 你之前改成 `log.With()` 的写法也已兼容并保留，见 [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L52-L60)。

**验证**
- 已执行：`go test ./pkg/worker`，通过。  

如果你要继续下一步，我可以直接把 `web_search/report_gen` 从“Prompt分派”升级成“真实 Tool 执行分派”（例如：`web_search` 先调搜索工具，再让 LLM 做总结；`report_gen` 走模板化报告组装）。

---

> `d:\codeproject\mygo\wukong\wukong\pkg\worker` `d:\codeproject\mygo\wukong\wukong\pkg\manager` `d:\codeproject\mygo\wukong\wukong\pkg\queue` `d:\codeproject\mygo\wukong\wukong\pkg\statemachine` 这4个文件夹里持久化到DB的操作都改成异步落库，manager、work、statemachine、queue这4个文件里的程序各种操作太频繁，几乎都是内存操作，如果加上频繁操作数据库，程序执行性能会下降很多，所以持久化操作改成异步持久化db操作，在pkg/asyncdb，单独新建目录asyncdb独立写异步持久化db操作的操作，或者你能提供更好优化建议
>

**已完成改造**

- 我已经按你的方向实现了独立异步落库模块：新增 [writer.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L1-L129)。
- `Manager` 已接入异步写队列：新增 `asyncWriter` 字段并在构造时初始化，见 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L98-L139)。
- `Manager.Stop()` 会等待异步队列尽量 flush 后退出，见 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L197-L213)。
- `CreateTask/UpdateTask/CreateSubTask/UpdateSubTask` 的持久化都改成异步提交（写快照，避免并发修改污染），见 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L808-L892)。
- 任务主流程里原本直接 `repo.Update...` 的调用已替换为异步落库入口（例如 `submitSubTask`、`onSubTaskResult`、聚合完成/失败写回），见 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L483-L569) 和 [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L628-L670)。

**关于你点名的4个目录**
- `pkg/manager`：已完成异步化改造（真正有 DB 写路径的核心目录）。
- `pkg/worker` / `pkg/queue` / `pkg/statemachine`：当前代码里本身没有直接 DB 持久化调用，主要是内存和调度逻辑。  
  所以现在的实现是把“所有 DB 写入口”集中收敛到 `manager -> asyncdb`，达到你要的“高频内存操作不被 DB 同步写阻塞”的目标。

**验证**
- 已通过：`go test ./pkg/asyncdb ./pkg/worker`
- `pkg/manager` 全量编译仍被你仓库里已有的 `taskdb.go` 旧接口问题阻塞（不是本次异步化引入的新问题）。

**更优优化建议（下一步）**
- 现在是“异步队列 + 多 worker”第一版，已经能显著降同步写延迟。
- 下一步建议做两个增强（我可以继续直接改）：
  - **去重合并**：对 `update_task/update_subtask` 按 `task_id/sub_task_id` 做 coalescing，短时间多次状态更新只落最后一次。
  - **批量写入**：对同类写操作做 micro-batch（比如每 20ms 或满 N 条批量 flush）进一步降 DB 往返成本。

如果你同意，我下一步直接把这两个增强也落到 `pkg/asyncdb`。

---



> 好的进一步改进，去重合并和批量写入优化程序


filePath: d:\codeproject\mygo\wukong\wukong\pkg\manager\manager.go
          
**已完成优化（去重合并 + 批量调度）**

- 我已经把异步落库升级为“**去重合并 + 微批处理**”模型，核心在 [writer.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L12-L214)。
- `Job` 新增 `Key` 字段，用于同一实体更新去重（后写覆盖前写），见 [writer.go:L12-L16](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L12-L16)。
- `Writer` 新增 `flushInterval`、`maxBatchSize`，支持按时间窗口和批大小触发 flush，见 [writer.go:L20-L34](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L20-L34)。
- dispatcher 会把短窗口内任务聚合成 batch，再由 worker 执行，见 [writer.go:L148-L214](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L148-L214)。
- 生命周期并发安全也补齐了，`Submit/Stop` 避免关闭通道竞态，见 [writer.go:L108-L146](file:///d:/codeproject/mygo/wukong/wukong/pkg/asyncdb/writer.go#L108-L146)。

**Manager 接入变更**

- `NewManager` 里已启用批量参数：`flush=20ms`、`batch=128`，见 [manager.go:L119-L127](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L119-L127)。
- `persistUpdateTask`、`persistUpdateSubTask` 已加去重键：
  - `update_task:<task_id>`
  - `update_subtask:<sub_task_id>`
  见 [manager.go:L821-L860](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L821-L860)。
- create 类写入仍走普通异步，不做去重，避免误丢事件（语义正确）。

**验证**

- `go test ./pkg/asyncdb ./pkg/worker` ✅
- `go test ./pkg/manager -run TestDoesNotExist` ✅（可编译通过）

如果你愿意，下一步我可以继续做“**真正 SQL 批量写**”（同类 update 合并成单条 `CASE WHEN` 或批量 upsert），那会比当前“调度层批处理”再进一步。

---

​            

已按你给的 v0.3 第一个大清单，落地了 **Skills 插件化系统** 的第一版可运行实现（含目录规范解析、热加载、注册中心、沙箱执行、skill_meta 持久化、技能列表 API）。

**本次实现**
- 新增 Skills 核心模块 [system.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/skills/system.go#L20-L520)，包含：
  - `SKILL.md` 解析：`# Skill`、`Description`、`Params`、`Tools`、`Execute`、`Template`、`Memory Config`
  - 目录扫描加载：`skills/<skill-name>/SKILL.md`
  - 热加载：按 `poll_interval` 周期重扫并自动刷新
  - 注册中心：线程安全 `List/Get`
  - 工具白名单校验：`CanUseTool(skill, tool)`
  - 脚本沙箱执行入口：支持 `.py/.sh/.ps1`，并带执行超时
  - 内置 3 个基础技能：`chat`、`web_search`、`report_gen`
- 新增 skill_meta 批量 upsert 仓储 [skill_repo.go](file:///d:/codeproject/mygo/wukong/wukong/internal/repository/skill_repo.go#L13-L72)：
  - `BatchUpsertSkills` 单条 SQL 批量 `ON CONFLICT (skill_name)` 更新
- 新增技能服务层 [skill_service.go](file:///d:/codeproject/mygo/wukong/wukong/internal/service/skill_service.go#L5-L18)
- 新增技能接口处理器 [skill.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/skill.go#L9-L23)
- 路由接入 `/api/v1/skill/list` [router.go](file:///d:/codeproject/mygo/wukong/wukong/internal/route/router.go#L27-L110)
- 启动流程接入 Skills 注册中心 + 可选 DB 持久化 [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L54-L138)
  - 启动时自动加载技能并热重载
  - 若数据库连接失败，自动降级为仅内存注册（服务仍可运行）
- 新增配置默认值 [config.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/config/config.go#L83-L111)：
  - `skills.root_dir`
  - `skills.poll_interval_sec`
  - `skills.exec_timeout_sec`

**和你 roadmap 对齐情况**
- 已覆盖：
  - Skills 标准目录 + `SKILL.md` 解析
  - 热加载（无重启）
  - 技能注册中心
  - 技能列表 API
  - Tool 白名单校验能力
  - Python/Bash/PowerShell 脚本执行入口（沙箱基础版：超时+入口限制）
  - 默认 3 个基础技能
- 当前这版仍属于 v0.3 第一阶段“可用骨架”，后续可继续补强：
  - 更严格沙箱隔离（CPU/内存/网络/文件系统白名单）
  - 变更事件级监听（当前是轮询热加载）
  - 技能启停管理 API（enable/disable/update）

**验证结果**
- `go test ./...` 通过
- `go vet ./...` 通过
- 代码诊断无新增错误

如果你继续，我下一步可以直接把 **Skill 执行链路接入 Worker 的 action router**（按 `skill_name/action` 自动选 Skill + Tool 白名单执行），把 v0.3 的 Skills 从“可管理”推进到“可执行闭环”。



---

​            


filePath: d:\codeproject\mygo\wukong\wukong\cmd\server\main.go
          
已完成：v0.3 的 **ReAct 执行引擎（Worker 内部）** 已做成独立文件并接入 worker 执行链路。

**实现内容**
- 新增独立 ReAct 引擎文件 [react.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/react.go#L1-L213)。
- 核心能力已覆盖 roadmap 要求：
  - `Thought -> Action -> Observation` 循环
  - 最大迭代次数限制（`maxIterations=6`）
  - 中间过程记录（`react_steps`）
  - 工具调用与 Observation 回灌 LLM
  - 工具调用白名单（基于 skill 允许的 tools）
  - 最终结果聚合输出（含 token usage）

**关键代码点**
- ReAct 执行器入口：`NewReActExecutor` / `Execute`  
  [react.go:L43-L181](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/react.go#L43-L181)
- ReAct 输出协议（JSON function-calling 风格）解析：  
  [react.go:L202-L213](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/react.go#L202-L213)
- 系统提示词约束（强制 JSON、tool/final 两种 action）：  
  [react.go:L194-L200](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/react.go#L194-L200)

**Worker 接入**
- Worker 的 `SubTaskExecutorWithTools` 默认执行器已切到 ReAct：  
  [exec_subtask.go:L191-L201](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L191-L201)
- `web_search/report_gen` action 路由也指向 ReAct 执行器：  
  [exec_subtask.go:L194-L196](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go#L194-L196)
- 启动流程已调用带 tools+skills 的 handler：  
  [main.go:L99-L105](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L99-L105)

**为“按 skill_name/action 自动路由”补齐**
- 规划器给子任务参数注入了 `skill_name`、`action`，供 ReAct 路由：  
  [planner.go:L252-L253](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/planner.go#L252-L253)

**技能脚本执行增强（闭环支撑）**
- Skills 引擎支持 `ExecuteWithParams`，可把参数通过 `SKILL_PARAMS` 传给脚本：  
  [skillsengine.go:L199-L255](file:///d:/codeproject/mygo/wukong/wukong/pkg/skills/skillsengine.go#L199-L255)
- 脚本执行支持注入环境变量：  
  [skillsengine.go:L531-L552](file:///d:/codeproject/mygo/wukong/wukong/pkg/skills/skillsengine.go#L531-L552)

**验证**
- 已执行并通过：
  - `go test ./...`
  - `go vet ./...`

如果你继续下一步，我可以把 `react_steps` 的每轮 `thought/action/observation` 持久化到 `task_exec_log`，这样就能满足 roadmap 里“可追溯全链路思考过程”的落库要求。

---


已完成，多厂商 LLM 抽象层已改造完成，并保持现有调用方式兼容（`llm.New(...).Chat/StreamChat` 不变）。

**已完成改造**
- 原单文件实现已拆分为多 Provider 架构，核心入口在 [provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/provider.go#L10-L150)。
- 原 `llm.go` 逻辑已迁移并替换为 DeepSeek 适配器文件 [deepseek_provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/deepseek_provider.go#L1-L15)。
- 新增 OpenAPI 通用适配器 [openapi_provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/openapi_provider.go#L14-L128)。
- 新增 Ollama 本地部署适配器 [ollama_provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/ollama_provider.go#L14-L117)。
- 新增豆包适配器 [doubao_provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/doubao_provider.go#L1-L15)。
- 公共消息/响应结构体抽到 [types.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/types.go#L1-L34)。

**架构与能力**
- 新增 `ProviderType` 枚举：`deepseek / ollama / openapi / doubao`：见 [provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/provider.go#L10-L17)。
- 新增 `WithProviderType(...)` 选项和适配器工厂：见 [provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/provider.go#L65-L150)。
- Provider 默认值按厂商自动注入（base_url/model）：见 [provider.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/provider.go#L126-L137)。
- OpenAPI/DeepSeek/豆包走统一 OpenAI-Compatible Chat/Stream 流程。
- Ollama 走 `/api/chat`，并将响应适配回统一 `ChatResponse`。

**配置与启动接入**
- 配置新增 `llm.provider`，默认 `deepseek`：见 [config.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/config/config.go#L104-L108)。
- 启动时读取 provider 并注入 `llm.WithProviderType(...)`：见 [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L49-L56)。

**测试与验证**
- 扩展了 `pkg/llm` 单测，增加 provider 类型与默认值校验：见 [llm_test.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/llm/llm_test.go#L11-L50)。
- 已通过：
  - `go test ./pkg/llm`
  - `go test ./...`
  - `go vet ./...`

如果你接下来要做 v0.3 里的“多厂商负载均衡 + 自动降级”，我可以直接在这个架构上加 `provider_pool`（优先级、熔断、重试回退）并接入现有 `worker/react` 调用链。

----

​            


已完成 v0.3 的 **Memory 记忆抽象层**，并按你要求放在 `pkg/memory` 下拆分为独立文件。

**已实现文件**
- 统一接口与模型：[mem_interface.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/mem_interface.go#L8-L61)
- 短期记忆（Working Memory）：[short_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/short_mem.go#L9-L135)
- 长期记忆（Long-term Memory）：[long_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/long_mem.go#L11-L96)
- 记忆管理与生命周期（Manager）：[manager_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/manager_mem.go#L12-L463)

**功能覆盖**
- **统一记忆接口**
  - `WriteMemory / ReadMemory / UpdateMemory / DeleteMemory`
  - `CompressMemory / SessionArchive / SharedMemorySync / MemoryExpire`
  - 见 [mem_interface.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/mem_interface.go#L52-L61)
- **短期记忆（Working）**
  - 按 `task_id` 维护对话历史、窗口大小、摘要、过期时间
  - 支持 append、summary 设置、过期清理
  - 见 [short_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/short_mem.go#L20-L120)
- **长期记忆（Long-term）**
  - 按 `memory_id` 存储 `user_id/skill_name/topic/content/source_task_id`
  - 支持检索 `Search(user, skill, keyword, limit)`
  - 见 [long_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/long_mem.go#L22-L88)
- **生命周期管理**
  - `CompressMemory`：按窗口自动压缩最近历史为摘要
  - `SessionArchive`：把短期会话归档到长期记忆
  - `MemoryExpire`：清理 working/shared 过期数据
  - 见 [manager_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/manager_mem.go#L88-L176)

**与现有链路接入**
- 已把 memory manager 注入到 Tool 层，使 `memory_read/memory_write` 走统一记忆管理：
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L135-L143)

**单元测试**
- 新增记忆模块测试：[manager_mem_test.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/manager_mem_test.go#L10-L99)
  - 覆盖 working 写读与压缩
  - 覆盖归档到 long-term
  - 覆盖 shared 同步与过期清理

**验证结果**
- `go test ./pkg/memory` 通过
- `go test ./...` 通过
- `go vet ./...` 通过

如果你继续，我可以下一步把 `memory_long_term` 对接真实 PostgreSQL（含 topic/skill/user 维度索引检索）并把 `SessionArchive` 落库到 `scripts/schema.sql` 的表结构。

---


已实现这两个接口，并已经接入路由可直接调用。

**变更内容**
- 新增 Memory Handler，提供两个查询接口实现：
  - [memory.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/memory.go#L23-L38) `ListWorking`
  - [memory.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/memory.go#L46-L61) `ListLong`
- 新增 Memory Service，封装对 memory manager 的查询调用：
  - [memory_service.go](file:///d:/codeproject/mygo/wukong/wukong/internal/service/memory_service.go#L9-L29)
- 路由层接入两个新 API：
  - [router.go](file:///d:/codeproject/mygo/wukong/wukong/internal/route/router.go#L106-L110)
- 启动装配注入 `memoryManager` 到路由构造：
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L169-L170)
- memory 存储层补齐列表能力（支持过滤+limit+排序）：
  - Working 列表：[short_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/short_mem.go#L123-L148)
  - Long 列表：[long_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/long_mem.go#L91-L123)
  - Manager 暴露列表方法：[manager_mem.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/manager_mem.go#L186-L192)

**接口说明**
- `GET /api/v1/memory/working/list`
  - Query: `task_id`(可选), `limit`(可选，默认20，最大200)
  - 返回当前登录用户（JWT UserID）下的短期记忆列表
- `GET /api/v1/memory/long/list`
  - Query: `skill_name`(可选), `keyword`(可选), `limit`(可选，默认20，最大200)
  - 返回当前登录用户下的长期记忆列表（按创建时间倒序）

**测试与验证**
- 新增测试覆盖列表能力与排序过滤：
  - [manager_mem_test.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/memory/manager_mem_test.go#L101-L140)
- 已执行并通过：
  - `gofmt -w ...`
  - `go test ./...`
  - `go vet ./...`

如果你要，我下一步可以继续把这两个 list 接口升级为“数据库真实查询版”（对接 `memory_working` / `memory_long_term` 表，而不是仅内存态）。

---


​          
已按 `wukong_roadmap.md` v0.4 第 1 项把“流式交互与实时反馈模块”落地到后端，并接入了你要求的 `pkg/websocket/websocket.go` 封装。

**实现结果**
- 新增 SSE 接口：
  - `GET /api/v1/stream/chat?sessionId=xxx&last_seq=0`
  - `GET /api/v1/stream/task?taskId=xxx&last_seq=0`
- 新增 WebSocket 接口（可选增强）：
  - `GET /api/v1/stream/ws/task?taskId=xxx&last_seq=0`
  - 支持客户端发送：
    - `{"action":"interrupt","task_id":"task_xxx"}`
    - `{"action":"inject","task_id":"task_xxx","content":"补充指令"}`
- 消息类型分层已支持并实际推送：`THINK`、`TOOL`、`CHUNK`、`STATUS`、`FINISH`
- 支持断线重连补发：基于 `last_seq` + `stream_message.seq` 查询补发
- 通道管控：收到 `FINISH` 自动关闭对应任务/会话流通道
- Manager 统一推送管理：主任务状态变化、子任务结果会自动广播到 task stream

**关键代码**
- SSE + WebSocket handler（含重连补发、心跳、中断/注入命令）
  - [stream.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/stream.go#L28-L206)
- 流式服务（发布订阅、seq 管理、FINISH 自动关流、内存+DB回放）
  - [stream_service.go](file:///d:/codeproject/mygo/wukong/wukong/internal/service/stream_service.go#L13-L187)
- `stream_message` 持久化仓储（Append + ListAfterSeq）
  - [stream_repo.go](file:///d:/codeproject/mygo/wukong/wukong/internal/repository/stream_repo.go#L19-L74)
- 路由接入（新增 `/api/v1/stream/*`）
  - [router.go](file:///d:/codeproject/mygo/wukong/wukong/internal/route/router.go#L114-L119)
- Manager 推送接入（STATUS/FINISH/THINK/TOOL/CHUNK）
  - [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L99-L101)
  - [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L299-L327)
  - [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L642-L673)
  - [manager.go](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L335-L357)
- Chat 链路接入 chat stream 推送（CHUNK + FINISH）
  - [chat.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/chat.go#L19-L20)
  - [chat.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/chat.go#L125-L128)
- 启动装配（StreamService/StreamRepository 注入 Manager 与 Router）
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L105-L107)
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L126-L127)
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L150-L153)
  - [main.go](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L174-L175)

**验证**
- 已通过：
  - `gofmt -w ...`
  - `go test ./...`
  - `go vet ./...`

如果你下一步要，我可以继续把“断线重连补发”从 `last_seq` 参数扩展为同时支持 `Last-Event-ID` 头（SSE 标准做法）。

---

toolName: view_files
            

已完成，断线重连补发现在同时支持：

- Query 参数：`last_seq`
- Header：`Last-Event-ID`

并且保持向后兼容。

**具体改动**
- 在 [stream.go](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/stream.go) 中把三处读取序号改为统一入口 `resolveLastSeq(c)`：
  - `ChatSSE`
  - `TaskSSE`
  - `TaskWebSocket`
- 新增解析逻辑：
  - 先尝试 `last_seq`
  - 若无效/为空，再尝试 `Last-Event-ID`
  - 都没有则回退 `0`
- 新增函数位置可见：
  - [resolveLastSeq / parseSeqValue](file:///d:/codeproject/mygo/wukong/wukong/internal/handler/stream.go#L172-L195)

**行为说明**
- 优先级：`last_seq` > `Last-Event-ID`
- `Last-Event-ID` 仅接受非负整数（和原 `seq` 语义一致）
- 这样浏览器/代理自动带的 SSE 重连头也能直接触发补发，不再依赖必须传 query 参数

**验证**
- `gofmt -w internal/handler/stream.go` 通过
- `go test ./...` 通过
- `go vet ./...` 通过

----

这两个文件是 同一条流式能力链路里的不同层级 ，可以理解为：

- stream_service.go = 基础流引擎层 （底层能力）
- stream_app_service.go = 应用编排层 （业务语义）
功能区别

- stream_service.go 负责“流本身”：
  
  - 定义消息类型常量（ THINK/TOOL/CHUNK/STATUS/FINISH ）
  - 发布消息（ PublishChat/PublishTask ）
  - 订阅消息（ SubscribeChat/SubscribeTask ）
  - 回放断线补发（ listAfterSeq -> repo ListAfterSeq ）
  - 内存缓冲、订阅者管理、FINISH 自动关流
     代码位置： stream_service.go
- stream_app_service.go 负责“把业务动作映射到流”：
  
  - 对外给 handler 提供更业务化入口（订阅 chat/task）
  - 处理 ws 指令语义（ interrupt / inject ）
  - 调用 TaskService 做任务取消/注入
  - 注入时再调用 StreamService.PublishTask 发 THINK 事件
     代码位置： stream_app_service.go
     为什么要分两层

- stream_service.go 更通用，可被 manager、task、chat 等模块复用。
- stream_app_service.go 把“业务规则”集中，避免 handler 里写业务逻辑。
- 分层后链路清晰： handler -> stream_app_service -> stream_service -> stream_repo 。
一句话总结：
 stream_service.go 管“怎么流”； stream_app_service.go 管“什么业务要流、什么时候流”

---


已修复，这次问题的核心根因其实是两层：

- **Manager 初始化时传的是 `nil` 仓储**，导致任务/子任务/执行日志都不会落库。
- **`task_exec_log` 原先完全没有写入链路**，即使任务执行了也不会入表。

另外我也把 `task_sub` 的更新改成了 Upsert，避免“先更新后创建”时丢数据。

**已修复内容**
- 接通 Manager 的数据库仓储注入，不再是 `nil`：
  - [main.go:L106-L108](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L106-L108)
  - [main.go:L124-L127](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L124-L127)
  - [main.go:L151-L153](file:///d:/codeproject/mygo/wukong/wukong/cmd/server/main.go#L151-L153)
- 在 Manager 增加执行日志模型与仓储接口，并在关键流程落 `task_exec_log`：
  - 类型/接口扩展：[manager.go:L55-L62](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L55-L62), [manager.go:L99](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L99)
  - 关键流程埋点（任务创建、状态变化、子任务创建/提交、结果回调）：[manager.go:L304](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L304), [manager.go:L349](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L349), [manager.go:L445](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L445), [manager.go:L624-L637](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L624-L637), [manager.go:L690-L692](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L690-L692)
  - 日志异步持久化函数：[manager.go:L1046-L1074](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L1046-L1074)
- 在 Worker Pool 增加任务事件回调，Manager 可接收运行过程事件并写入 `task_exec_log`：
  - 回调类型/注入点：[pool.go:L28](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L28), [pool.go:L145-L146](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L145-L146), [pool.go:L298-L301](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L298-L301)
  - 运行事件触发（开始/失败/重试/最终失败/完成）：[pool.go:L369-L455](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L369-L455), [pool.go:L486-L496](file:///d:/codeproject/mygo/wukong/wukong/pkg/worker/pool.go#L486-L496)
  - Manager 自动绑定该回调：[manager.go:L161](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L161), [manager.go:L194-L199](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L194-L199), [manager.go:L739-L747](file:///d:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go#L739-L747)
- `task_sub` 更新改为 Upsert，防止更新时目标行不存在导致“看起来没写进去”：
  - [task_repo.go:L317-L351](file:///d:/codeproject/mygo/wukong/wukong/internal/repository/task_repo.go#L317-L351)
- 新增 `task_exec_log` 仓储写入：
  - [task_repo.go:L353-L360](file:///d:/codeproject/mygo/wukong/wukong/internal/repository/task_repo.go#L353-L360)

**验证结果**
- 已执行 `gofmt`：通过
- 已执行 `go test ./pkg/worker/... ./pkg/manager/... ./internal/repository/... ./cmd/server/...`：通过
- 已执行 `go vet ./pkg/worker/... ./pkg/manager/... ./internal/repository/... ./cmd/server/...`：通过

如果你现在重跑一轮任务，`task_sub` 应该会持续有数据，`task_exec_log` 会看到从提交到完成/失败的关键过程日志。
