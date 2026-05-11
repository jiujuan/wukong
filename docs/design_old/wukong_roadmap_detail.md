# 悟空（Wukong）多智能体系统 —— 五版本迭代路线图

> **文档版本**：v1.0  
> **创建日期**：2026-04-07  
> **适用范围**：Wukong 项目全体系前后端迭代规划，供研发、产品、架构团队共同参考

---

## 总览

| 版本 | 里程碑名称 | 核心目标 | 预估周期 |
|------|-----------|---------|---------|
| v0.1 | **地基奠定**（Foundation） | 项目骨架、核心数据流跑通、最小可用系统 | 4 周 |
| v0.2 | **任务引擎**（Task Engine） | Manager 调度 + 四叉树队列 + Worker 执行链路 | 6 周 |
| v0.3 | **智能涌现**（Intelligence） | ReAct + Skills 插件化 + Memory 记忆体系 | 6 周 |
| v0.4 | **全链路打通**（Full Stack） | 流式交互 + 前端 UI + API 完整化 | 5 周 |
| v0.5 | **生产就绪**（Production Ready） | 可观测性 + 安全加固 + 性能优化 + 多租户 | 6 周 |

---

## v0.1 ——《地基奠定》（Foundation）

### 版本目标
搭建整个系统的工程骨架，完成数据库建模、核心公共模块、认证体系和最基础的对话链路，确保团队可以在统一规范下并行开发。

### 核心交付清单

#### 1. 工程脚手架
- 初始化 Go 模块 `github.com/jiujuan/wukong`，Go 1.23+
- 建立标准目录结构：`cmd/`、`internal/`、`pkg/`、`route/`、`handler/`、`middleware/`、`config/`
- 集成配置库 **koanf**，支持 `.yaml` 多环境配置（dev/staging/prod）
- 集成 **slog** 结构化日志，统一日志格式与级别
- 接入 **Gin** HTTP 框架，搭建路由骨架（参照 `route/router.go` 模板）
- Docker Compose 本地开发环境（PostgreSQL + Redis）

#### 2. 数据库初始化
- 执行全量建表 DDL（参照架构文档第七章）：
  - 对话链路：`chat_session`、`chat_message`、`chat_memory`
  - 任务链路：`task_info`、`task_sub`
  - 记忆体系：`memory_working`、`memory_long_term`、`memory_shared`
  - 辅助表：`task_exec_log`、`stream_message`、`skill_meta`
- 引入 **pgx** 驱动，封装 DB 连接池管理模块
- 引入 **go-redis**，封装 Redis 客户端与连接池

#### 3. 公共基础模块
- 实现 `pkg/response/response.go`：统一 Success/Fail 返回结构
- 实现分页结构体 `PageReq` / `PageResp`
- 实现请求唯一 ID 中间件 `middleware.RequestID()`
- 实现跨域中间件 `middleware.Cors()`
- 实现全局错误码常量清单（400/401/403/404/429/500/1001~1004）

#### 4. 认证模块（Auth）
- `POST /api/v1/auth/login`：用户名密码登录，返回 JWT access_token（有效期 2h）
- `POST /api/v1/auth/logout`：登出，Redis 黑名单撤销 Token
- `GET /api/v1/auth/profile`：获取当前用户信息
- 实现 JWT 鉴权中间件 `middleware.JWTAuth()`

#### 5. 最小对话模块（Chat，无任务执行）
- `POST /api/v1/chat/session/create`：创建会话
- `GET /api/v1/chat/session/list`：会话分页列表
- `POST /api/v1/chat/message/send`：发送消息（直接 LLM 回复，不走任务队列）
- `GET /api/v1/chat/message/list`：消息分页列表
- 单轮 LLM 调用链路跑通（接入 OpenAI Compatible 接口）

#### 6. LLM 抽象层（基础版）
- 定义统一 `LLMProvider` 接口：`Chat()`、`Stream()`
- 实现 OpenAI Compatible 适配器（支持 DeepSeek、通义千问等）
- 基础 Token 统计能力

### v0.1 验收标准
- [ ] 本地 `docker-compose up` 一键启动全部依赖
- [ ] 登录接口正常返回 JWT Token
- [ ] 发送一条消息，可从 LLM 获得回复并落库
- [ ] 所有接口响应均符合统一结构规范
- [ ] 数据库全量建表 DDL 无报错执行完毕

---

## v0.2 ——《任务引擎》（Task Engine）

### 版本目标
实现 Wukong 最核心的分布式任务执行链路：Manager 调度中枢 + 自研四叉树持久化队列 + Worker 执行单元，打通"任务提交 → 调度 → 执行 → 状态回流"全流程。

### 核心交付清单

#### 1. 自研四叉树持久化任务队列
- 实现纯 Go 四叉树（Quad-tree）数据结构，支持优先级索引
- 支持任务插入、弹出、删除、优先级检索
- 实现内存队列 + PostgreSQL 双存储：
  - 服务启动：全量预加载 `PENDING/PLANNING/RUNNING/WAITING` 状态任务
  - 运行阶段：事件驱动增量更新，禁止主动轮询 DB
  - 崩溃恢复：重启后全量预加载，自动重建队列
- 实现任务幂等去重（全局唯一 task_id + 内存 Map 二重校验）
- 实现延时任务、重试队列能力

#### 2. Manager 调度中枢
- **任务入口模块**：`POST /api/v1/task/create` 接收任务，生成全局唯一幂等 ID，创建 task_info 落库
- **任务规划模块**（基础版）：支持固定模板拆解大任务为子任务，生成 task_sub DAG，持久化至 DB
- **状态机引擎**：
  - 状态流转：`PENDING → PLANNING → RUNNING → WAITING → COMPLETED / FAILED / CANCELLED`
  - 实现状态跃迁守卫、进入/退出钩子、事务性跃迁（失败回滚）
  - 状态变更同步落库
- **调度循环模块**：持续扫描可执行子任务，判断 DAG 依赖满足条件，推入四叉树队列
- **Worker 集群管理**：维护 Worker 心跳表（Redis），感知节点存活与负载
- **结果聚合模块**：收集子任务执行结果，更新主任务状态，归档最终输出

#### 3. Worker 执行单元
- 实现可配置化 Worker 协程池（并发数、队列长度、空闲超时、最大重试均可配置）
- 实现主动拉取：Worker 按优先级轮询四叉树队列，获取子任务
- 实现心跳上报：定时向 Manager 上报存活、负载、支持技能列表（Redis TTL 机制）
- 实现状态上报：执行开始/完成/失败状态实时回传 Manager
- 实现 panic 安全捕获，任务执行崩溃自动标记 FAILED 并触发重试
- 实现 Worker 优雅启停

#### 4. 任务相关 API
- `GET /api/v1/task/list`：任务分页列表（支持状态过滤）
- `GET /api/v1/task/detail`：任务详情（含子任务列表）
- `POST /api/v1/task/cancel`：取消任务（状态机跃迁至 CANCELLED）
- `GET /api/v1/subtask/list`：子任务分页列表

#### 5. 对话 → 任务关联
- 发送消息时，若识别为复杂指令，自动创建 Task 并关联 `session_id`
- chat_message 中写入 `task_id` 外键

### v0.2 验收标准
- [ ] 提交一个任务，可在 DB 观察到状态从 PENDING 流转到 COMPLETED
- [ ] 重启服务后，未完成任务自动重建进入队列继续执行
- [ ] 并发提交 100 个任务，Worker 池正确并发处理，无数据竞争
- [ ] 重复提交同一 task_id，幂等去重，返回历史结果
- [ ] Worker 心跳断开后，Manager 能感知并将其任务重调度

---

## v0.3 ——《智能涌现》（Intelligence）

### 版本目标
为 Worker 注入真正的 AI 能力：ReAct 推理引擎 + Skills 插件化执行 + Memory 记忆体系三大核心智能模块全部上线，系统从"任务调度器"升级为"真正的 AI Agent"。

### 核心交付清单

#### 1. Skills 插件化系统
- 实现技能标准目录规范解析（`SKILL.md` 必填核心文件）
- 实现技能加载器：扫描指定技能目录，解析元信息，注册至 `skill_meta` 表
- 实现热加载/无重启更新：文件变更监听，自动重载技能配置
- 实现技能沙箱执行引擎：Python/Bash 脚本安全沙箱（权限白名单 + 资源限制 + 超时强制终止）
- 内置 3 个基础技能：`chat`（对话）、`web_search`（联网搜索）、`report_gen`（报告生成）
- `GET /api/v1/skill/list`：技能列表接口（含启用状态、版本信息）
- 实现技能 Tool 白名单权限管控（Skill 仅能调用配置的 Tool 列表）

#### 2. Tool 原子能力系统
- 定义标准 `Tool` 接口：`Name()`、`Description()`、`Execute(params)`
- 实现以下原子 Tool：
  - **LLM 调用 Tool**：封装 LLM Provider，支持流式/非流式
  - **联网搜索 Tool**：接入搜索 API（DuckDuckGo / Bing / Serper）
  - **文件读写 Tool**：本地文件系统安全读写
  - **HTTP 请求 Tool**：通用外部 API 调用
  - **代码执行 Tool**：沙箱执行 Python/JS 代码片段
  - **记忆读写 Tool**：对接 Memory 抽象层

#### 3. ReAct 执行引擎（Worker 内部）
- 实现 ReAct 循环：`Thought → Action → Observation → Thought...`
- 支持最大迭代轮次限制（防死循环）
- 支持中间过程实时上报（思考步骤、Tool 调用、观察结果）
- 支持 LLM Function Calling 格式的工具调用
- 支持 ReAct 执行过程写入 `task_exec_log`

#### 4. Memory 记忆抽象层
- 定义统一记忆接口：`WriteMemory`、`ReadMemory`、`UpdateMemory`、`DeleteMemory`、`CompressMemory`
- **短期记忆（Working Memory）**：
  - Redis 热缓存实时读写（滑动窗口，默认 5 轮，SKILL.md 可配）
  - 超窗口阈值自动触发 LLM 摘要压缩
  - 任务结束后归档摘要至 `memory_working` 表
- **长期记忆（Long-term Memory）**：
  - PostgreSQL 结构化存储（按主题/技能/用户维度分类）
  - RAG 检索接口：`SemanticSearch`、`MemoryMatch`（基于 embedding_vector 字段）
  - 跨任务记忆复用：同类技能自动关联历史记忆
- **共享记忆（Shared Memory）**：
  - 多 Worker 共享记忆空间（PostgreSQL + Redis 分布式锁）
  - 按任务/技能/角色的读写权限隔离
  - 增量同步与冲突解决
- 实现按 SKILL.md 配置差异化记忆策略（memory_type / window_size / compress_switch）
- 记忆生命周期管理：`MemoryExpire`、`SessionArchive`、`SharedMemorySync`

#### 5. LLM 抽象层扩展
- 新增适配器：Ollama（本地部署）、火山豆包
- 实现多厂商负载均衡与自动降级
- 实现 Token 消耗统计落库（按任务维度）
- 实现记忆摘要压缩专用轻量 LLM 调用链路
- `GET /api/v1/memory/working/list`：短期记忆查询
- `GET /api/v1/memory/long/list`：长期记忆查询

### v0.3 验收标准
- [ ] 通过对话触发"撰写行业调研报告"任务，Worker 使用 ReAct + 联网搜索 + 报告生成 Skill 完成报告
- [ ] 超过滑动窗口阈值后，长上下文自动压缩，Token 消耗下降可观察
- [ ] 多个子任务并发执行时，共享记忆无数据竞争
- [ ] 新增一个 Skill 目录后，无需重启即可在 `/skill/list` 中看到并被调用
- [ ] ReAct 循环日志完整写入 `task_exec_log`，可追溯全链路思考过程

---

## v0.4 ——《全链路打通》（Full Stack）

### 版本目标
打通用户侧完整体验：流式实时交互上线、前端 Web UI 完整交付、API 规范全面对齐，让整个系统从内部可用升级为真正面向用户的完整产品。

### 核心交付清单

#### 1. 流式交互与实时反馈模块
- **SSE（Server-Sent Events）**：
  - `GET /api/v1/stream/chat?sessionId=xxx`：流式对话推送
  - `GET /api/v1/stream/task?taskId=xxx`：任务执行过程推送
  - 实现消息类型分层推送：`THINK`、`TOOL`、`CHUNK`、`STATUS`、`FINISH`
- **WebSocket**（可选增强）：
  - 支持客户端主动发送中断指令
  - 支持补充指令注入执行中任务
- **断开重连机制**：
  - 客户端断开后会话暂存，重连后自动补发 `seq` 大于客户端最后收到序号的消息
  - 依赖 `stream_message` 表的 seq 字段实现
- **通道管控**：任务完成/终止后自动关闭 SSE 通道，释放资源
- **Manager 推送管理**：统一管理所有 SSE 会话，支持任务 ID 广播

#### 2. 任务规划模块升级（LLM 动态规划）
- 支持通过 LLM 动态分析用户意图，自动拆解任务为子任务 DAG
- 支持任务规划结果流式推送（规划思路实时展示给用户）
- 规划失败自动降级至模板规划

#### 3. 前端 Web UI（Vite + React + TailwindCSS + shadcn/ui + TypeScript）
- **布局框架**：左侧导航（会话列表）+ 右侧主内容区
- **对话页面**：
  - 多轮对话消息渲染（用户/AI 消息气泡）
  - Markdown 渲染（代码高亮、表格、列表）
  - SSE 流式输出实时渲染（打字机效果）
  - 会话新建/切换/删除
- **任务执行页面**：
  - 任务状态看板（PENDING/RUNNING/COMPLETED/FAILED 分类展示）
  - 任务执行过程实时面板（THINK/TOOL/CHUNK 分层展示）
  - 子任务 DAG 可视化展示
  - 任务取消操作
- **记忆面板**（侧边抽屉）：
  - 当前任务短期记忆展示
  - 长期记忆列表浏览
- **技能列表页**：展示可用技能、状态、版本、记忆策略配置
- **全局 Toast 通知**：API 错误、任务状态变更友好提示

#### 4. API 完整化与规范对齐
- 补齐所有文档中定义的接口（参照 API 架构文档第五章）
- 全面对齐统一响应结构、分页参数、字段命名规范
- 实现全局请求限流中间件（60 次/分钟）
- 实现关键操作敏感字段脱敏（密码、Token 等不入响应体）
- 完善接口级错误码（1001~1004 全部生效）
- 编写 API 接口文档（Swagger / OpenAPI 3.0 自动生成）

#### 5. 对话记忆完善
- `chat_memory` 对话摘要自动写入：超过 20 轮自动摘要压缩
- 用户偏好提取与写入（用于后续对话个性化增强）
- 历史消息分页加载优化（懒加载 + 虚拟列表）

### v0.4 验收标准
- [ ] 用户在前端发起任务，全程可通过 SSE 实时看到 THINK/TOOL/CHUNK 推送
- [ ] 断网重连后，未推送的消息自动补发，前端无感知
- [ ] 前端 UI 完整运行，对话、任务、技能、记忆四大核心页面可用
- [ ] API 文档自动生成，所有接口有入参/出参示例
- [ ] 对话超 20 轮后，自动摘要压缩，继续对话上下文不丢失

---

## v0.5 ——《生产就绪》（Production Ready）

### 版本目标
面向真实生产部署：全面提升系统可观测性、安全性、稳定性和性能，支持多租户/多用户隔离，完善运维能力，使系统具备商业化交付条件。

### 核心交付清单

#### 1. 可观测性体系
- **结构化日志**：slog 全链路日志，按 task_id/session_id/worker_id 关联追踪
- **指标监控（Metrics）**：
  - 接入 Prometheus：任务吞吐量、队列积压深度、Worker 利用率、LLM 调用延迟、Token 消耗
  - 提供 `/metrics` HTTP 端点
  - 预置 Grafana Dashboard 模板
- **分布式追踪（Tracing）**：
  - 接入 OpenTelemetry：任务创建 → 调度 → Worker 执行 → Tool 调用全链路 Trace
  - 集成 Jaeger 或 Tempo 可视化查看
- **健康检查**：`GET /healthz`、`GET /readyz` 端点
- **任务执行回溯**：支持通过 `task_exec_log` 可视化回放 Agent 执行全过程

#### 2. 安全加固
- **鉴权增强**：
  - JWT Refresh Token 机制（access_token 2h + refresh_token 7d）
  - 多设备登录管控（Redis 登录会话表）
- **API 安全**：
  - 请求签名验证（对外开放 API Key 模式）
  - SQL 注入防护（pgx 参数化查询全覆盖）
  - XSS/CSRF 防护（前端 CSP + 后端校验）
- **技能沙箱加固**：
  - Python/Bash 沙箱资源硬限制（CPU 10%、内存 256MB、执行时间 60s）
  - 网络访问白名单（沙箱内禁止访问内网）
  - 文件系统隔离（tmpfs 临时文件系统）
- **敏感数据保护**：DB 字段级加密（用户 Token、API Key），返回数据脱敏

#### 3. 性能优化
- **四叉树队列优化**：大规模任务（10k+）场景压测调优，内存占用基线化
- **数据库优化**：
  - 慢查询分析与索引补充
  - 大表（task_exec_log、stream_message）归档分区策略
  - 读写分离适配（pgx 连接池主从）
- **Redis 缓存优化**：
  - Working Memory 热数据 TTL 策略精细化
  - 防缓存击穿（Singleflight 模式）
- **LLM 调用优化**：
  - 请求合并（Batch 模式，非流式场景）
  - 失败自动降级+重试退避策略
  - 流式输出背压控制（避免前端消费慢导致 buffer 积压）
- **并发优化**：Worker 协程池动态扩缩容（无重启），根据队列深度自动调节并发数

#### 4. 多租户与用户体系
- 用户注册/管理完整接口
- 租户（Organization）隔离：数据按 `user_id`/`org_id` 完全隔离
- 角色权限管理：Admin / Developer / Viewer 三级角色
- 用量配额管理：按用户/租户限制任务数、Token 消耗上限
- 管理后台 API：用户管理、技能管理、系统监控（仅 Admin 可见）

#### 5. 部署与运维
- **Docker 镜像**：提供完整 Dockerfile，多阶段构建，镜像体积 < 50MB
- **Docker Compose（生产版）**：含 PostgreSQL（主从）、Redis（Sentinel）、Prometheus、Grafana
- **Kubernetes（可选）**：提供 Helm Chart 模板，支持 HPA 自动扩缩 Worker
- **优雅停机**：接收 SIGTERM 后等待执行中任务完成（最长 30s），再退出
- **配置热更新**：技能配置、LLM Provider 参数支持运行时更新，无需重启
- **数据备份方案**：PostgreSQL WAL 归档 + 定时快照脚本

#### 6. 开发者体验
- 完整的本地开发文档（`CONTRIBUTING.md`）
- Makefile 封装常用命令（`make run`、`make test`、`make build`、`make migrate`）
- 单元测试覆盖率 ≥ 60%（核心模块：状态机、四叉树队列、记忆模块）
- E2E 测试：覆盖"提交任务 → 执行完成 → 流式推送"完整链路

### v0.5 验收标准
- [ ] Grafana 面板实时显示任务吞吐量、队列积压、Worker 利用率
- [ ] 1000 并发任务压测，系统稳定运行，无内存泄漏
- [ ] 多用户数据完全隔离，A 用户无法访问 B 用户的任务和记忆
- [ ] `docker-compose up` 一键启动含监控的完整生产环境
- [ ] 核心模块单元测试覆盖率 ≥ 60%
- [ ] 安全扫描无高危漏洞

---

## 附录：跨版本依赖关系

```
v0.1 地基奠定
  ↓ 工程骨架 + DB + 认证 + 基础对话
v0.2 任务引擎
  ↓ 四叉树队列 + Manager 调度 + Worker 执行
v0.3 智能涌现
  ↓ Skills 插件 + ReAct + Memory 记忆
v0.4 全链路打通
  ↓ SSE 流式 + 前端 UI + API 完整化
v0.5 生产就绪
  ↓ 可观测性 + 安全 + 性能 + 多租户
```

## 附录：各版本关键技术风险提示

| 版本 | 风险点 | 应对建议 |
|------|--------|---------|
| v0.2 | 四叉树队列并发安全 | 优先编写充分的并发单元测试，使用 `-race` 检测 |
| v0.2 | 崩溃恢复全量预加载性能 | 超大任务量场景需测试预加载耗时，必要时分批加载 |
| v0.3 | ReAct 死循环风险 | 强制最大迭代次数限制 + 执行超时强制终止 |
| v0.3 | 沙箱逃逸安全 | Python 沙箱使用 seccomp/namespace 级别隔离 |
| v0.4 | SSE 通道内存泄漏 | 任务完成/超时必须关闭 SSE 通道，定期扫描僵尸连接 |
| v0.5 | LLM Provider 不稳定 | 多 Provider 负载均衡 + 熔断 + 降级策略前置设计 |

---

*本文档随版本迭代持续更新，每个版本发布前需对验收标准逐项 Review。*
