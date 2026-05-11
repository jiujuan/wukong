# 悟空（Wukong）AI Agent多智能体架构设计文档

## 一、架构概述

### 1.1 系统命名

- **英文名**：Wukong

- **中文名**：悟空

- **定位**：纯Go开发、轻量高性能、无第三方中间件依赖、可扩展的分布式多智能体任务执行系统，支持自定义+AI生成（Claude/Codex）Skills插件，具备完善的任务编排、安全执行、动态调度能力。

go mod github.com/jiujuan/wukong

go1.23+

### 1.2 核心设计理念

- **解耦分层**：调度层与执行层完全分离，Manager只做调度决策，Worker只做任务执行

- **插件化**：Skills标准化目录规范，支持热加载、无重启更新，兼容Claude Code等AI生成技能

- **安全可控**：技能沙箱隔离、权限最小化、资源限制，杜绝第三方技能安全风险

- **高性能**：可配置化Worker池、优先级任务队列、协程复用，支持高并发任务处理

- **全链路可追溯**：状态机管控任务全生命周期，任务执行流程可监控、可回溯

### 1.3 整体架构图

```Plain Text

┌─────────────────────────────────────────────────────────────┐
│                      外部请求层                              │
│                用户/API调用/流式客户端（SSE/WebSocket）        │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                      Manager（调度中枢）                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ 任务入口模块 │  │ 任务规划模块 │  │ 状态机引擎模块      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ 调度循环模块 │  │ DAG依赖管理 │  │ 结果聚合模块        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              流式实时反馈模块（核心新增）              │    │
│  └─────────────────────────────────────────────────────┘    │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                      自研四叉树持久化任务队列                  │
│         四叉树优先级调度+启动全量加载+增量更新无DB轮询         │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                  Worker集群（执行单元）                       │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ 可配置Worker池  │  │ 任务拉取模块    │  │ 心跳上报模块 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ 实时过程上报模块 ││  记忆读写调用模块  ││ ReAct执行引擎 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                  Skills插件化系统                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ 技能加载器      │  │ 标准化目录规范  │  │ 沙箱执行引擎 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
│         支持自定义/Claude Code/Codex生成技能+记忆配置         │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                  底层能力支撑系统（三大核心模块）              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Tool原子能力 │  │ LLM抽象层   │  │ Memory记忆抽象层    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 1.4 技术选型和架构

- 后端：go + gin + 配置库koanf+slog+ pgx + go-redis + redis + postgresql

- 前端：vite + react + tailwind css + shadcn ui + typescript
- 架构：模块化、可扩展的架构设计

## 二、核心模块架构设计

### 2.1 Manager模块（调度中枢，非Agent Loop/ReAct）

#### 2.1.1 核心定位

**调度循环（Scheduler Loop）**，多智能体协调器，仅负责全局任务调度、编排、监控，不执行具体业务逻辑，不运行ReAct（ReAct在Worker/Skill内部实现）。

#### 2.1.2 核心职责

1. **任务接收与解析**：接收外部任务请求，生成全局唯一幂等ID，校验任务重复提交，创建流式会话通道

2. **任务规划**：通过模板/LLM动态拆解大任务为子任务，构建DAG依赖图，全量任务信息持久化至DB

3. **状态机管控**：管控主任务+子任务全生命周期状态流转，状态变更同步落库+实时推送

4. **队列调度**：将可执行子任务推入自研持久化队列，支持优先级、延时、重试，任务状态实时同步DB

5. **Worker集群管理**：维护Worker节点心跳、负载感知、任务分发，异常任务自动重调度

6. **结果聚合**：收集子任务执行结果，聚合生成最终任务输出，最终结果持久化归档

7. **异常恢复**：程序崩溃重启后，自动从DB加载未完成、执行中任务，重置状态重新调度，避免任务丢失

8. **幂等管控**：基于全局唯一任务ID做幂等校验，重复提交任务直接返回历史结果，杜绝重复执行

9. **实时反馈管控**：统一管理SSE/WebSocket会话，推送Agent思考过程、Tool调用、中间结果、执行进度

#### 2.1.3 核心子模块

- **任务入口模块**：提供API/内部接口接收任务，校验任务参数，创建流式会话

- **任务规划模块**：实现任务拆解、DAG构建、子任务依赖管理

- **状态机引擎**：定义任务状态流转规则，支持事务性跃迁、钩子函数

- **调度循环模块**：无限循环扫描任务，判断子任务可执行性，分发任务

- **结果聚合模块**：子任务结果收集、数据整合、最终响应生成

- **流式反馈模块**：SSE/WebSocket会话管理、实时消息推送、通道保活与关闭

### 2.2 Worker模块（执行单元）

#### 2.2.1 核心定位

无状态执行单元，水平扩展，主动拉取任务，负责具体Skill执行，支持ReAct思考范式。

#### 2.2.2 核心职责

1. **心跳上报**：定时向Manager上报节点存活状态、负载、支持技能列表

2. **任务拉取**：按优先级轮询自研队列，主动获取子任务

3. **Worker池执行**：可配置化协程池，高并发执行任务，panic安全

4. **技能调用**：加载对应Skill插件，通过沙箱安全执行

5. **状态上报**：实时上报任务执行中、完成、失败状态

6. **结果回传**：将执行结果/异常信息上报Manager

7. **过程实时上报**：执行过程中同步上报Agent思考步骤、Tool调用详情、中间生成内容

#### 2.2.3 可配置Worker池

- **核心配置**：并发数、队列长度、空闲超时、最大重试、监控开关

- **特性**：动态扩缩容（无重启）、优雅启停、任务复用、性能监控

- **安全机制**：panic捕获、资源限制、超时强制终止



### 2.3 自研四叉树持久化任务队列

- **四叉树核心结构**：采用四叉树索引结构实现优先级队列，相比传统堆/链表队列，优先级检索、任务插入、任务删除时间复杂度更低，高并发下任务调度效率提升显著，适配多优先级、海量任务场景的高效分发。

- **内存+DB双存储无轮询设计**：彻底摒弃定时轮询DB模式，杜绝数据库无效IO与性能压力，采用**启动全量预加载+运行增量同步**机制，内存队列承担核心调度，PostgreSQL仅做持久化落盘与断点恢复。

- **任务加载机制**：

  - 服务启动：一次性加载DB中所有Pending、Running、Waiting状态的有效任务至内存四叉树队列，完成初始化

  - 运行阶段：通过事件驱动做增量更新，仅在任务创建、状态变更、重试触发时，同步DB数据至内存队列，无主动轮询DB

  - 崩溃恢复：重启后复用全量预加载逻辑，快速重建内存队列，断点续跑未完成任务

- **核心能力**：四叉树优先级调度（高/中/低）、延时任务、重试队列、任务去重、**任务幂等、断点续跑、崩溃恢复、无DB轮询**

- **对接逻辑**：Manager写入任务同步落库并直接更新内存队列，Worker主动从内存四叉树队列拉取任务，执行后更新任务状态并同步DB，程序重启后全量预加载重建队列，无需轮询扫描。

- **幂等设计**：全局唯一任务ID+任务状态分布式锁，避免重复执行、重复入队，重复提交任务直接命中内存缓存+DB历史记录，自动去重。

- **持久化机制**：任务创建、状态变更、执行结果全量持久化至PostgreSQL，仅做数据落地与恢复备份，不参与日常调度查询，彻底降低DB读写压力。队列崩溃/程序宕机后，重启自动扫描DB未完成任务，重新推入内存队列执行

- **无第三方依赖**：纯Go自研四叉树结构+内存队列，轻量高性能，无外部中间件侵入。

- **四叉树核心结构**：采用四叉树索引结构实现优先级队列，相比传统堆/链表队列，优先级检索、任务插入、任务删除时间复杂度更低，高并发下任务调度效率提升显著，适配多优先级、海量任务场景的高效分发。

- **内存+DB双存储无轮询设计**：彻底摒弃定时轮询DB模式，杜绝数据库无效IO与性能压力，采用**启动全量预加载+运行增量同步**机制，内存队列承担核心调度，PostgreSQL仅做持久化落盘与断点恢复。

  

### 2.4 Skills插件化系统

#### 2.4.1 标准化目录规范（用户指定）

```Plain Text

skill-name/                # 目录名=skill名称（小写、下划线分隔）
└── SKILL.md               # 唯一必填核心文件（大小写敏感）
├── scripts/               # 可选：Python/Bash/JS等可执行脚本（Claude Code生成）
├── templates/             # 可选：Prompt/报告/代码模板文件
└── references/            # 可选：参考文档、规范、示例
```

#### 2.4.2 SKILL.md核心格式

```Markdown

# Skill: [skill名称]
## Description
技能功能描述
## Params
- 参数名: 参数类型 (必填/选填，默认值)
## Tools
- 允许调用的Tool列表
## Execute
执行入口（scripts/xxx.py）
## Template
模板文件路径（可选）
## Memory Config（新增记忆配置项）
- memory_type: working/long-term/rag  # 记忆类型
- window_size: 5  # 短期记忆滑动窗口大小
- compress_switch: true  # 是否开启摘要压缩
- rag_collection: skill_report  # 长期记忆RAG集合名称
- expire_time: 24h  # 记忆过期时间
```

#### 2.4.3 核心能力

1. **热加载/无重启**：技能新增、更新、删除无需重启Manager/Worker

2. **多源兼容**：支持自定义Go技能、Claude Code技能、Codex生成技能

3. **安全沙箱**：Python/JS/WASM沙箱隔离，权限白名单，资源限制

4. **技能注册中心**：统一管理技能元信息、版本、权限、状态

5. **记忆能力适配**：支持按技能配置记忆策略，差异化适配短对话、长文本、多任务场景



### 2.5 Tool原子能力系统

#### 2.5.1 核心定位

最小执行单元，无业务逻辑，为Skills提供底层原子能力，多技能共享。

#### 2.5.2 核心Tool类型

- LLM调用Tool、联网搜索Tool、文件读写Tool、HTTP请求Tool、代码执行Tool、记忆读写Tool

#### 2.5.3 权限管控

Skill仅能调用配置的白名单Tool，杜绝未授权能力访问

### 2.6 LLM Provider抽象层

#### 2.6.1 核心定位

统一LLM调用接口，屏蔽不同厂商/本地模型差异，支持无缝切换，同时承担记忆摘要压缩、RAG检索增强能力。

#### 2.6.2 支持厂商

- **云端厂商**：OpenAI、DeepSeek、火山豆包、通义千问等OpenAI兼容厂商

- **本地模型**：Ollama、vLLM等本地部署模型

#### 2.6.3 核心特性

- 统一请求/响应/流式格式、多厂商负载均衡、自动降级、Token统计、记忆摘要压缩、RAG检索增强

### 2.7 Memory记忆抽象层（核心新增）

#### 2.7.1 核心定位

与LLM、Tool平级的底层基础核心模块，抽象统一记忆读写接口，屏蔽底层存储差异，解决长程任务上下文丢失、多轮对话无记忆、多智能体协同无共享信息的问题，支撑复杂行业调研、报告生成、多轮交互场景。

#### 2.7.2 模块依赖与调用逻辑

- **存储依赖**：复用现有PostgreSQL（JSONB字段持久化记忆）+Redis（热数据缓存），无新增中间件

- **调用链路**：Worker执行Skill的ReAct循环时，自动/按需调用Memory模块，完成上下文读写、记忆压缩、RAG检索，无需侵入Skill核心业务代码

- **生命周期绑定**：短期记忆与任务会话绑定，长期记忆与技能/用户维度绑定，支持跨任务、跨Worker复用

#### 2.7.3 分场景记忆设计方案

##### 场景一：多轮对话与单任务短期记忆（Working Memory）

- **适用场景**：单任务执行、多轮交互式对话、短流程Agent任务

- **存储介质**：Redis热缓存（实时读写）+PostgreSQL持久化（归档备份）

- **管理策略**：
- 滑动窗口：固定保留最近N轮完整对话/执行上下文（默认5轮，可通过SKILL.md配置）
- 摘要压缩：超过窗口阈值时，触发本地轻量LLM自动压缩老旧历史，保留核心结论，丢弃冗余细节，降低Token消耗
- 过期清理：任务结束/超时后，自动清理缓存，归档摘要至DB

- **Worker结合方式**：Worker拉取任务后初始化SessionMemory对象，ReAct循环中实时读写，任务结束后归档最终记忆

##### 场景二：长文本处理与行业调研长期记忆（Long-term Memory & RAG）

- **适用场景**：行业调研、深度报告生成、长文本分析、大篇幅内容创作

- **存储介质**：PostgreSQL（存储结构化记忆、报告摘要、调研结论）+Redis（缓存高频检索内容）

- **核心能力**：
- 结构化记忆存储：按主题、任务、技能维度分类存储调研数据、关键结论、参考资料
- RAG检索增强：结合LLM实现语义检索，快速匹配历史同类任务经验、行业数据
- 增量更新：支持长任务执行中逐步写入记忆，断点恢复后自动加载历史上下文
- 跨任务复用：同类技能可复用过往长期记忆，避免重复调研、重复分析

##### 场景三：多智能体协同共享记忆（Multi-Task Shared Memory）

- **适用场景**：多智能体分工协作、跨子任务数据共享、复杂分布式任务编排

- **存储介质**：PostgreSQL（共享记忆库）+Redis（分布式锁+实时同步）

- **核心能力**：
- 共享记忆空间：Manager统一管控，多Worker/子任务可读写指定共享记忆
- 权限隔离：按任务、技能、智能体角色设置记忆读写权限，防止数据错乱
- 实时同步：子任务更新记忆后，增量同步至其他关联执行单元，保证协同一致性
- 冲突解决：基于分布式锁避免多Worker同时写入，保证记忆数据准确性

#### 2.7.4 核心接口与能力

- 统一读写接口：WriteMemory、ReadMemory、UpdateMemory、DeleteMemory、CompressMemory

- RAG检索接口：SemanticSearch、MemoryMatch、ContextEnhance

- 生命周期管理：MemoryExpire、SessionArchive、SharedMemorySync

- 监控能力：记忆存储量、读写次数、压缩频率、检索命中率

### 2.8 状态机引擎

- **纯Go实现**：无第三方依赖，管控任务全生命周期+记忆状态同步

- **状态定义**：Pending→Planning→Running→Waiting→Completed/Failed/Cancelled

- **核心能力**：状态跃迁守卫、进入/退出钩子、事务性跃迁（失败回滚）、记忆状态同步变更



### 2.9 流式交互与实时反馈模块

#### 2.9.1 核心定位

补齐AI Agent实时交互能力，打破传统“提交-等待-返回”批处理模式，实现Agent思考过程、执行链路、中间结果的实时推送，满足用户对Agent执行透明度的核心需求，支持长连接会话与断开重连。

#### 2.9.2 支持通信方式

- **SSE（Server-Sent Events）**：服务端单向推送，适配Web端纯实时展示场景，轻量无开销，兼容浏览器。必要推送结合Event公共模块

- **WebSocket**：全双工通信，支持客户端主动中断任务、发送补充指令，适配交互式Agent场景

#### 2.9.3 实时推送内容

必要实时推送结合Event公共模块进行实时内容推送

- **任务状态流转**：Pending→Planning→Running等全状态实时同步

- **Agent思考过程**：ReAct思考步骤、任务拆解思路、决策逻辑

- **Tool调用链路**：Tool名称、入参、执行耗时、返回结果

- **中间生成内容**：LLM流式输出片段、子任务中间结果、草稿内容

- **执行进度**：总任务完成度、子任务执行数量、剩余预估时间

- **异常与告警**：执行报错、重试提醒、任务终止通知

#### 2.9.4 核心机制

- **会话绑定**：全局任务ID绑定流式会话，支持幂等会话复用

- **断开重连**：客户端断开后，会话暂存，重连后自动补发未推送消息

- **消息持久化**：实时推送消息同步落库，支持历史过程回溯查看

- **通道管控**：任务完成/终止后自动关闭通道，释放资源，避免内存泄漏

### 3.0、对话Chat与任务Task模型

#### 3.1 核心一句话总结

- **对话（Chat）= 交互载体、上下文载体、多轮记忆载体**
  负责“人跟 Agent 说话”，不负责复杂执行。

- **任务（Task）= 执行载体、调度载体、长流程工作单元**
  负责 Agent 真正“干活”，是调度、拆解、执行、重试、持久化的最小单元。

#### 3.2 详细对比（直接可写进文档）

1. 定位区别

对话（Chat）

- 定位：**人机交互入口 + 多轮上下文容器**
- 本质：用户与系统的**会话通道**
- 生命周期：用户主动开启 → 多轮聊天 → 关闭/过期

任务（Task）

- 定位：**系统执行单元 + 调度单元 + 状态机单元**
- 本质：Agent 要完成的**一件具体工作**
- 生命周期：创建 → 规划 → 执行 → 完成/失败/取消

2. 结构区别

对话chat包含：

- session（会话）
- message（消息列表）
- chat_memory（对话记忆、摘要、用户偏好）

任务task包含：

- task_info（主任务）
- task_sub（子任务 DAG）
- memory_working（任务执行上下文）
- stream_message（执行过程流式推送）
- task_exec_log（执行日志）

#### 3.3 与系统模块的关系

对话只关联：

API → Chat模块 → Chat记忆 → 消息推送

任务关联整个核心引擎：

API → Manager → 四叉树队列 → Worker → Skill → Tool → LLM → 各类记忆 → 日志 → 流式推送

**一句话：对话是“面子”，任务是“里子”。**

#### 3.4 对话（Chat）的作用

1. **维护多轮交互上下文**
   让 Agent 知道“刚才聊了什么”。

2. **承载用户意图输入**
   所有用户指令都从对话进入系统。

3. **管理多轮记忆**
   保存用户偏好、历史对话、自动摘要。

4. **提供连续交互体验**
   单轮、多轮聊天、追问、补充信息都靠它。

5. **绑定任务但不控制任务**
   一个对话可以发起多个任务。

####  3.5 任务（Task）的作用

1. **承载真正的执行逻辑**
   搜索、写报告、代码生成、数据处理、调用工具等。

2. **支持调度、优先级、重试、崩溃恢复**
   只有任务能进入队列、被 Worker 执行。

3. **支持 DAG 子任务拆解**
   复杂工作拆成多步执行。

4. **支持状态机管理**
   PENDING → PLANNING → RUNNING → COMPLETED。

5. **支持持久化、断点续跑、长时间执行**
   对话断了、页面关了、服务重启了，任务照样跑。

6. **支持流式过程推送**
   思考过程、工具调用、中间结果。

#### 3.6 分别解决什么场景问题

#### 对话解决的场景问题

1. 多轮聊天场景

- 日常闲聊
- 多轮问答
- 连续追问
- 上下文理解
- 历史对话回顾

2. 轻量交互场景

- 单轮问答
- 简单指令
- 快速回复

3. 会话管理场景

- 会话列表
- 会话标题
- 历史消息回溯
- 多会话切换

4. 不解决：

- 复杂执行、拆解任务、长时间运行、调度、重试、崩溃恢复。

#### 任务解决的场景问题

1. 复杂长时任务场景

- 行业调研报告
- 批量数据处理
- 多步骤工具调用（搜索→分析→总结→写报告）
- 代码项目生成
- 爬虫 + 清洗 + 结构化输出

2. 调度与分布式执行场景

- 优先级调度
- 多 Worker 负载均衡
- 任务排队、限流、削峰

3. 可靠性场景

- 崩溃恢复
- 断点续跑
- 重试机制
- 幂等防重

4. 可观测场景

- 执行日志
- 过程追踪
- 状态监控
- 失败复盘

5. 不解决：

- 连续多轮自然聊天、上下文记忆、会话管理。

####  3.7 最关键的联动关系（你架构里的真实逻辑）

一个对话可以 → 发起多个任务

```
用户对话 → 提取意图 → 创建 Task → 交给 Manager 调度 → Worker 执行 → 结果返回对话
```

一个任务可以 → 属于某个对话

- task_info 中有 session_id 字段
- 任务执行的流式思考、工具调用 → 推送到对应对话展示

但任务可以脱离对话独立存在

- 后台任务
- 定时任务
- API 直接调用任务
- 系统内部触发任务

####  3.8 极简总结（可直接复制进文档）

对话（Chat）

- 负责：**交互、上下文、多轮记忆、会话管理**
- 解决：**人与 Agent 自然连续对话**
- 特点：轻、短、交互型、无复杂执行

任务（Task）

- 负责：**执行、调度、拆解、持久化、重试、恢复**
- 解决：**Agent 复杂长流程可靠工作**
- 特点：重、长、执行型、带状态机

关系

- **对话是入口，任务是执行体**
- **对话管“说”，任务管“做”**
- **一个对话可发起多个任务，一个任务归属于一个对话**

如果你需要，我可以帮你把这段整理成**架构文档里的标准章节（4.2 对话与任务模型设计）**，直接放进 wukong_arch.md。

#### 3.9 任务Task执行全流程（新增记忆交互环节）

1. **任务提交**：用户/API向Manager提交任务请求，生成全局唯一幂等ID，校验重复提交，创建SSE/WebSocket流式会话+记忆会话

2. **意图解析**：Manager解析任务意图，匹配对应Skill，加载Skill配置的记忆策略，推送初始思考提示

3. **任务规划+持久化+记忆初始化**：Manager拆解大任务为子任务，构建DAG依赖图，绑定状态机，主/子任务+初始记忆全量持久化至PostgreSQL，实时推送任务拆解过程

4. **队列入队+增量更新**：Manager将无依赖的可执行子任务，按优先级写入内存四叉树队列，同步增量更新DB，无DB轮询操作

5. **任务拉取+记忆加载**：Worker通过可配置池，主动从内存四叉树队列拉取任务，加载对应记忆数据，执行前锁定任务+记忆状态防止重复执行

6. **技能执行+实时上报+记忆读写**：Worker加载对应Skill，通过沙箱执行ReAct循环，调用Tool/LLM/Memory完成逻辑，同步推送思考过程、Tool调用、LLM中间输出，实时读写更新记忆

7. **状态上报+落库+记忆归档**：Worker实时上报任务执行状态，状态变更同步增量更新DB与内存队列，任务阶段性完成后压缩归档记忆

8. **结果聚合+归档**：Manager收集所有子任务结果+最终记忆，校验DAG依赖，聚合生成最终结果并归档至DB，推送最终完整结果

9. **异常恢复+记忆重载**：程序/队列崩溃重启，Manager全量预加载DB有效任务(Manager自动扫描DB中Pending/Running状态任务)+关联记忆，重建内存四叉树队列，重置状态重新调度执行，推送恢复通知

10. **任务完结+会话关闭**：Manager更新任务状态为完成，关闭流式会话+记忆会话，清理短期记忆，归档长期记忆，返回最终结果给调用方



## 四、系统核心技术优势

1. **纯Go高性能**：全系统基于Go开发，四叉树队列+协程并发，内存占用低，任务调度效率远超传统队列

2. **无强中间件依赖+低DB压力**：核心队列内存实现，无DB轮询，仅做持久化备份，复用现有DB+Redis实现记忆存储，无新增组件

3. **任务不丢失+记忆不中断**：任务+记忆全生命周期DB持久化，支持崩溃重启断点续跑，长任务上下文不丢失

4. **幂等安全**：全局唯一ID+状态锁，重复任务自动去重，避免重复执行导致数据异常

5. **安全隔离**：第三方AI生成技能沙箱运行，记忆权限隔离，资源严格管控

6. **动态扩展**：Worker水平扩容，Skills热加载，记忆能力按需扩展，无停机平滑更新

7. **多AI技能兼容**：原生支持Claude/Codex生成的Skills，零修改适配记忆能力

8. **全生命周期可控**：状态机+DAG+持久化+记忆四重管控，任务可监控、可回溯、可重试、可恢复

9. **实时交互透明**：支持SSE/WebSocket双模式流式推送，Agent执行全流程可视

10. **长程任务支撑**：三层记忆体系彻底解决上下文丢失问题，适配复杂调研、报告生成、多轮交互场景

11. **标准化规范**：Skills目录+记忆配置规范统一，开发/AI生成技能成本极低

## 五、部署与运维

### 5.1 部署架构

- **单节点部署**：Manager+Worker一体化部署，适合小规模场景

- **分布式部署**：Manager单节点（或主备），Worker多节点水平扩展，适合高并发场景

### 5.2 目录结构

pkg目录下如果要用到参数的可以用函数选项模式进行可扩展的设计

```Plain Text

wukong-agent/
├── cmd/                         # 仅存放应用服务启动入口，无通用底层代码
│   ├── manager/                 # Manager服务启动入口（初始化、启动调度循环）
│   │   └── main.go
│   └── worker/                  # Worker服务启动入口（初始化、启动执行池）
│       └── main.go
├── internal/                    # 应用代码
│   ├── route/                   # 路由代码
│   ├── middleware/              # 中间件代码
│   ├── handler/ 
│   ├── service/
│   ├── repository/
│   │    ├── user_repo.go
├── pkg/                         # 全局公共依赖包，全系统复用，无业务耦合
│   ├── logger/                  # 统一日志组件（日志分级、格式化、持久化）
│   ├── config/                  # 配置加载与解析（YAML/环境变量、热更新）
│   ├── db/                      # 数据库操作（基于pgx的PostgreSQL封装，含记忆存储）
│   ├── redis/                   # Redis客户端封装（缓存、分布式锁、记忆热缓存）
│   ├── errors/                  # 统一错误处理（错误码、错误堆栈、国际化）
│   ├── response/                # 统一响应封装（API/内部调用标准响应体）
│   ├── monitor/                 # 全链路监控模块（含记忆读写监控）
│   ├── stream/                  # 流式交互模块（SSE/WebSocket封装）
│   ├── memory/                  # 记忆抽象层（核心新增，三大记忆场景实现）
│   ├── manager/                 # Manager核心通用逻辑（调度、规划、状态机）
│   ├── worker/                  # Worker核心通用逻辑（任务拉取、池化执行）
│   ├── queue/                   # 自研四叉树任务队列（优先级、重试、延时）
│   ├── statemachine/            # 状态机引擎（任务状态流转、事务管控）
│   ├── skills/                  # Skills插件化系统（加载、沙箱、注册中心）
│   ├── tool/                    # Tool原子能力系统（接口、实现、权限管控）
│   ├── uuid/                    # 生成uuid
│   ├── jwt/                     # 包装go-jwt
│   ├── validator/               # 包装go-validator
│   ├── Event/                   # 事件系统
│   └── llm/                     # LLM Provider抽象层（多厂商适配、统一调用）
├── skills/                      # Skills插件存放目录（含记忆配置）
│   └── [skill-name]/            # 单个技能目录，SKILL.md为唯一必填文件
├── configs/                     # 外部配置文件（环境配置、权限配置、LLM参数）
├── go.mod
└── go.sum
```

#### 5.2.1 目录设计说明

- **cmd 目录**：轻量化服务入口，仅负责服务初始化、启动、优雅关停，不编写任何通用底层逻辑、业务核心代码，职责单一，便于部署和启动管控。

- **pkg 目录**：全系统公共核心包，所有可复用、跨模块调用的能力统一收纳，实现**一处编写、多处复用**，彻底解耦业务层与底层通用能力，便于单元测试和版本维护。

- **pkg 细分能力**：

  - 基础设施类：logger、config、db（含任务+记忆持久化）、redis（记忆缓存）、uuid、errors、response、monitor、stream、memory，支撑全系统底层通用、全链路监控、任务持久化、实时交互与记忆管理

  - 核心业务类：manager、worker、queue（四叉树持久化队列）、statemachine、skills、tool、llm，对应架构核心模块，无业务耦合，可独立复用

- **pkg/memory 核心职责**：统一记忆抽象接口，实现短期/长期/共享记忆三大场景，对接DB+Redis存储，支持记忆压缩、RAG检索、会话管理、权限隔离

- **pkg/monitor 核心职责**：全系统指标采集与上报，覆盖Worker池状态、任务执行数/状态、状态机执行流程、技能加载状态、LLM调用量与全流程链路追踪、记忆读写与检索指标，支持指标暴露、数据上报、异常告警

- **pkg/stream 核心职责**：SSE与WebSocket双模式封装、会话管理、实时消息推送、断开重连、消息缓存与补发、通道生命周期管控

- **skills 目录**：独立于代码目录，专门存放标准化技能插件，支持记忆策略配置、热加载、独立更新，不侵入核心代码结构。



### 5.3 运维特性

- **监控指标**：依托pkg/monitor模块，覆盖Worker池运行状态、任务执行总数/成功率/耗时、任务全生命周期状态、状态机执行流转过程、技能加载与运行状态、LLM调用量/耗时/Token消耗、LLM全链路调用过程、**任务持久化状态、幂等校验次数、崩溃恢复任务数、流式会话数、消息推送量**

- **日志审计**：全链路任务执行日志、技能调用日志、异常日志、持久化落库日志、崩溃恢复日志、流式消息推送日志，与监控模块联动溯源

- **灰度更新**：Skills支持灰度发布，结合监控指标实时观测运行状态，异常一键回滚

- **持久化与恢复**：任务全生命周期DB持久化，支持程序/队列崩溃后自动恢复执行，任务幂等去重，无任务丢失风险

- **流式运维**：流式会话监控、连接状态告警、消息积压监控、断开重连成功率统计

## 六、适用场景介绍

- 复杂任务自动化执行（行业调研、报告生成、文章撰写）

- AI生成技能落地运行（Claude/Codex直接生成技能，无需二次开发）

- 分布式多任务并发处理

- 企业级插件化任务执行平台

- 本地+云端LLM混合调用的智能体系统

## 七、数据库表结构设计（PostgreSQL）

**整套系统完整的 PostgreSQL 建表SQL**写到文档最后，包含：
任务表、子任务表、任务执行日志表、记忆表（短期/长期/RAG）、流式消息表、技能元信息表。
全部字段、索引、注释齐全，可直接上线建库使用。

### 7.1 设计原则

- **完全适配架构**：任务幂等、持久化、四叉树队列加载、崩溃恢复、记忆系统、流式反馈
- **技术栈统一**：仅使用 PostgreSQL + JSONB，不引入新增中间件
- **性能优先**：关键字段加索引，优化启动全量加载与日常增量查询，支持启动时**批量加载未完成任务**，任务加载支持增量更新、无轮询设计，优化数据库性能
- **权责清晰**：每张表明确读写主体，保证数据一致性

### 7.2 完整建表 SQL（可直接执行）

#### 7.2.1 核心任务表

```sql
-- 主任务表 (task_info)
DROP TABLE IF EXISTS task_info;
CREATE TABLE task_info (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,        -- 全局幂等ID
    user_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64) NULL,               -- 关联对话会话
    skill_name VARCHAR(64) NOT NULL,           -- 执行技能
    params JSONB NOT NULL DEFAULT '{}'::JSONB,  -- 入参
    status VARCHAR(32) NOT NULL,                -- PENDING, PLANNING, RUNNING, WAITING, COMPLETED, FAILED, CANCELLED
    priority INT NOT NULL DEFAULT 5,            -- 四叉树队列优先级(1~10)
    retry_count INT NOT NULL DEFAULT 0,
    max_retry INT NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE NULL,
    result JSONB NULL,
    error TEXT NULL,
    is_deleted BOOLEAN DEFAULT FALSE,
    CONSTRAINT uk_task_id UNIQUE (task_id)
);

CREATE INDEX idx_task_status ON task_info(status);
CREATE INDEX idx_task_user ON task_info(user_id);
CREATE INDEX idx_task_session ON task_info(session_id);
CREATE INDEX idx_task_priority ON task_info(priority);
COMMENT ON TABLE task_info IS '主任务表，核心调度载体';

-- 子任务DAG表 (task_sub)
DROP TABLE IF EXISTS task_sub;
CREATE TABLE task_sub (
    id BIGSERIAL PRIMARY KEY,
    sub_task_id VARCHAR(64) NOT NULL UNIQUE,
    task_id VARCHAR(64) NOT NULL,
    depends_on JSONB NOT NULL DEFAULT '[]'::JSONB,  -- 依赖的sub_task_id列表
    action VARCHAR(128) NOT NULL,                   -- 执行动作/步骤
    params JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL,                    -- PENDING, RUNNING, SUCCESS, FAILED, SKIPPED
    worker_id VARCHAR(64) NULL,                    -- 执行节点ID
    result JSONB NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_sub_task_id UNIQUE (sub_task_id)
);

CREATE INDEX idx_sub_task_id ON task_sub(task_id);
CREATE INDEX idx_sub_status ON task_sub(status);
COMMENT ON TABLE task_sub IS '子任务表，维护DAG依赖关系';
```

#### 7.2.2 对话相关表

```sql
-- 对话会话表 (chat_session)
DROP TABLE IF EXISTS chat_session;
CREATE TABLE chat_session (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,       -- 多轮对话唯一标识
    user_id VARCHAR(64) NOT NULL,
    title VARCHAR(256) NULL,                      -- 会话标题(自动生成)
    scene VARCHAR(32) NOT NULL DEFAULT 'CHAT',    -- CHAT / TASK / AGENT
    status VARCHAR(32) NOT NULL DEFAULT 'OPEN',   -- OPEN / CLOSED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE NULL,
    CONSTRAINT uk_session_id UNIQUE (session_id)
);

CREATE INDEX idx_chat_session_user ON chat_session(user_id);
CREATE INDEX idx_chat_session_status ON chat_session(status);
COMMENT ON TABLE chat_session IS '多轮对话会话总表';

-- 对话消息表 (chat_message)
DROP TABLE IF EXISTS chat_message;
CREATE TABLE chat_message (
    id BIGSERIAL PRIMARY KEY,
    msg_id VARCHAR(64) NOT NULL UNIQUE,
    session_id VARCHAR(64) NOT NULL,             -- 关联会话
    user_id VARCHAR(64) NOT NULL,
    role VARCHAR(32) NOT NULL,                   -- user / assistant / system / tool
    content TEXT NOT NULL,                       -- 消息原文
    content_type VARCHAR(32) DEFAULT 'TEXT',     -- TEXT / MARKDOWN / JSON
    task_id VARCHAR(64) NULL,                    -- 关联任务ID
    thought TEXT NULL,                           -- Agent思考过程
    tool_call JSONB NULL,                        -- Tool调用信息
    tool_result JSONB NULL,                      -- Tool执行结果
    seq INT NOT NULL DEFAULT 0,                  -- 消息序号，保证顺序
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_msg_id UNIQUE (msg_id)
);

CREATE INDEX idx_chat_msg_session ON chat_message(session_id);
CREATE INDEX idx_chat_msg_user ON chat_message(user_id);
CREATE INDEX idx_chat_msg_task ON chat_message(task_id);
CREATE INDEX idx_chat_msg_seq ON chat_message(session_id, seq);
COMMENT ON TABLE chat_message IS '单轮/多轮消息明细表';

-- 对话记忆表 (chat_memory)
DROP TABLE IF EXISTS chat_memory;
CREATE TABLE chat_memory (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    recent_messages JSONB NOT NULL DEFAULT '[]'::JSONB,  -- 滑动窗口原始消息
    summary TEXT NULL,                                    -- 对话压缩摘要
    user_profile JSONB NULL,                             -- 用户画像
    preference JSONB NULL,                               -- 用户偏好
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_chat_mem_session UNIQUE (session_id)
);

CREATE INDEX idx_chat_mem_user ON chat_memory(user_id);
COMMENT ON TABLE chat_memory IS '多轮对话全局记忆，跨任务复用';
```

#### 7.2.3 记忆系统表

```sql
-- 短期工作记忆 (memory_working)
DROP TABLE IF EXISTS memory_working;
CREATE TABLE memory_working (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    full_history JSONB NOT NULL DEFAULT '[]'::JSONB,  -- 任务上下文历史
    summary TEXT NULL,                                 -- 压缩摘要
    window_size INT NOT NULL DEFAULT 5,
    compress_flag BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expire_at TIMESTAMP WITH TIME ZONE NULL,
    CONSTRAINT mem_working_task UNIQUE (task_id)
);

CREATE INDEX idx_mem_work_user ON memory_working(user_id);
COMMENT ON TABLE memory_working IS '任务级短期记忆，任务结束归档';

-- 长期经验记忆 (memory_long_term)
DROP TABLE IF EXISTS memory_long_term;
CREATE TABLE memory_long_term (
    id BIGSERIAL PRIMARY KEY,
    memory_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    skill_name VARCHAR(64) NOT NULL,    -- 归属技能
    topic VARCHAR(256) NOT NULL,        -- 记忆主题
    content TEXT NOT NULL,              -- 核心内容/结论
    embedding_vector vector(1536) NULL, -- 向量化检索(可选)
    source_task_id VARCHAR(64) NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_long_term_id UNIQUE (memory_id)
);

CREATE INDEX idx_mem_long_user ON memory_long_term(user_id);
CREATE INDEX idx_mem_long_skill ON memory_long_term(skill_name);
COMMENT ON TABLE memory_long_term IS '行业/经验类长期记忆，支持RAG检索';

-- 共享记忆 (memory_shared)
DROP TABLE IF EXISTS memory_shared;
CREATE TABLE memory_shared (
    id BIGSERIAL PRIMARY KEY,
    share_key VARCHAR(128) NOT NULL UNIQUE,  -- 共享标识
    data JSONB NOT NULL DEFAULT '{}'::JSONB,
    owner_task_id VARCHAR(64) NOT NULL,
    read_only BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_shared_key UNIQUE (share_key)
);

CREATE INDEX idx_mem_shared_owner ON memory_shared(owner_task_id);
COMMENT ON TABLE memory_shared IS '多智能体/子任务共享记忆空间';
```

#### 7.2.4 辅助支撑表

```sql
-- 任务执行日志 (task_exec_log)
DROP TABLE IF EXISTS task_exec_log;
CREATE TABLE task_exec_log (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    sub_task_id VARCHAR(64) NULL,
    log_type VARCHAR(32) NOT NULL,    -- INFO, WARN, ERROR, THINK, TOOL_CALL, LLM
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_log_task ON task_exec_log(task_id);
CREATE INDEX idx_log_sub_task ON task_exec_log(sub_task_id);
COMMENT ON TABLE task_exec_log IS '任务全链路执行日志，可追溯';

-- 流式消息 (stream_message)
DROP TABLE IF EXISTS stream_message;
CREATE TABLE stream_message (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    msg_type VARCHAR(32) NOT NULL,    -- THINK, TOOL, CHUNK, STATUS, FINISH
    content TEXT NOT NULL,
    seq INT NOT NULL DEFAULT 0,        -- 消息序号，重连补发
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_stream_task ON stream_message(task_id);
CREATE INDEX idx_stream_seq ON stream_message(task_id, seq);
COMMENT ON TABLE stream_message IS '流式实时消息，支持SSE/WebSocket';

-- 技能元信息 (skill_meta)
DROP TABLE IF EXISTS skill_meta;
CREATE TABLE skill_meta (
    id BIGSERIAL PRIMARY KEY,
    skill_name VARCHAR(64) NOT NULL UNIQUE,
    description TEXT NULL,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    enabled BOOLEAN DEFAULT TRUE,
    memory_type VARCHAR(32) DEFAULT 'working',  -- 记忆类型配置
    memory_window INT DEFAULT 5,                -- 滑动窗口大小
    memory_compress BOOLEAN DEFAULT TRUE,       -- 是否自动压缩
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_skill_name UNIQUE (skill_name)
);

COMMENT ON TABLE skill_meta IS '技能插件元信息，配置记忆策略';
```

### 7.3 ER 图（实体关系图）

```
+---------------------+       1:N        +---------------------+
|    chat_session     |<------------------|    chat_message     |
+---------------------+                  +---------------------+
| session_id (PK)     |                  | msg_id (PK)         |
| user_id             |                  | session_id (FK)     |
| title               |                  | user_id             |
| scene               |                  | role                |
| status              |                  | content             |
| created_at          |                  | task_id (FK)        |
+---------------------+                  | seq                 |
       ^                                   +---------------------+
       | 1:1                               ^
       |                                   |
+---------------------+                   |
|    chat_memory      |                   |
+---------------------+                   |
| session_id (PK,FK)  |                   |
| user_id             |                   |
| summary             |                   |
+---------------------+                   |
       ^                                  |
       | 1:N                              |
+---------------------+       N:1        +---------------------+
|    task_info        |<------------------|    task_sub         |
+---------------------+                  +---------------------+
| task_id (PK)        |                  | sub_task_id (PK)    |
| user_id             |                  | task_id (FK)        |
| session_id (FK)     |                  | depends_on          |
| skill_name          |                  | action              |
| status              |                  | status              |
| priority            |                  | worker_id           |
+---------------------+                  +---------------------+
       ^
       | 1:N
       |
+---------------------+       N:1        +---------------------+
|    memory_working   |<------------------|    memory_long_term |
+---------------------+                  +---------------------+
| task_id (PK)        |                  | memory_id (PK)      |
| user_id             |                  | user_id             |
| summary             |                  | skill_name          |
+---------------------+                  | topic               |
       ^                                  +---------------------+
       |
       | 1:N
       |
+---------------------+       N:1        +---------------------+
|    memory_shared    |<------------------|    task_exec_log    |
+---------------------+                  +---------------------+
| share_key (PK)      |                  | id (PK)             |
| data                |                  | task_id (FK)        |
| owner_task_id       |                  | sub_task_id (FK)    |
+---------------------+                  | log_type            |
       ^                                  +---------------------+
       |
       | 1:N
       |
+---------------------+       N:1        +---------------------+
|    stream_message   |<------------------|    skill_meta       |
+---------------------+                  +---------------------+
| id (PK)             |                  | skill_name (PK)     |
| task_id (FK)        |                  | description         |
| msg_type            |                  | enabled             |
| content             |                  | memory_type         |
+---------------------+                  +---------------------+
```

### 7.4 表关系详细说明

1.  **对话链路**：`chat_session` 是多轮对话的根节点，一对多关联 `chat_message`（存储单轮消息）；同时 `chat_session` 与 `chat_memory` 一对一关联，存储全局对话记忆与摘要。
2.  **任务与对话关联**：`task_info` 通过 `session_id` 关联对话会话，实现任务与对话的绑定，支持在任务执行中延续多轮对话上下文。
3.  **任务与子任务**：`task_info` 一对多关联 `task_sub`，子任务通过 `depends_on` 维护DAG依赖关系，由Manager统一调度。
4.  **记忆与任务/对话关联**：
    - `memory_working` 与 `task_info` 一对一，存储任务执行的短期上下文
    - `memory_long_term` 与 `task_info` 多对一，存储任务沉淀的长期经验
    - `memory_shared` 为共享记忆，被多个子任务/智能体读写
5.  **日志与消息关联**：`task_exec_log` 记录任务全链路日志，关联 `task_info`/`task_sub`；`stream_message` 为流式交互消息，关联 `task_info`，支持断开重连补发。

### 7.5 读写主体（Manager/Worker/API）说明

#### 7.5.1 写操作（写入数据）

| 表名 | 写入主体 | 写入时机 |
|------|----------|----------|
| `task_info` | API / Manager | 任务提交、状态变更、优先级更新 |
| `task_sub` | Manager | 任务规划拆解DAG、子任务创建 |
| `chat_session` | API | 新建多轮对话、关闭会话 |
| `chat_message` | API / Worker | 用户发消息、Agent回复、Tool调用/结果返回 |
| `chat_memory` | Worker | 多轮对话滑动窗口更新、摘要压缩、用户画像更新 |
| `memory_working` | Worker | 任务执行上下文读写、阶段性压缩摘要 |
| `memory_long_term` | Worker | 任务完成后沉淀核心结论、RAG检索结果存储 |
| `memory_shared` | Worker/Manager | 子任务写入共享数据、Manager同步共享状态 |
| `task_exec_log` | Worker | 执行过程实时写日志（思考、Tool调用、异常） |
| `stream_message` | Worker/Manager | 流式反馈实时写、重连补发写 |
| `skill_meta` | 运维/管理API | 技能注册、更新、启用禁用 |

#### 7.5.2 读操作（读取数据）

| 表名 | 读取主体 | 读取时机 |
|------|----------|----------|
| `task_info` | Manager / 四叉树队列 | 服务启动加载未完成任务、日常调度查询任务状态、幂等校验 |
| `task_sub` | Manager / Worker | 子任务调度、依赖检查、执行状态查询、DAG重建 |
| `chat_session` | API / Worker | 加载会话历史、判断会话状态、获取会话标题 |
| `chat_message` | API / Worker | 加载多轮对话历史、消息回溯、重连补发消息、流式消息渲染 |
| `chat_memory` | Worker | 加载多轮对话上下文、摘要、用户偏好用于增强回答 |
| `memory_working` | Worker | 任务执行加载上下文、崩溃恢复续跑、任务结束归档查询 |
| `memory_long_term` | Worker / LLM检索模块 | 技能执行时查询历史经验、RAG语义检索、同类任务经验复用 |
| `memory_shared` | Worker / Manager | 子任务间读取共享数据、多Agent协同数据同步 |
| `task_exec_log` | API / Manager / Worker | 任务链路追溯、问题排查、执行过程复盘 |
| `stream_message` | API / Worker | 客户端重连补发消息、历史执行过程回放 |
| `skill_meta` | Manager / Worker | 技能加载、获取记忆策略、校验技能是否启用 |

### 7.6 启动加载任务的关键SQL（给四叉树队列用）

```sql
-- 服务启动时一次性加载所有需要恢复的任务
SELECT * FROM task_info
WHERE status IN ('PENDING','PLANNING','RUNNING','WAITING')
AND is_deleted = FALSE
ORDER BY priority DESC;
```

```sql
-- 加载对应子任务DAG
SELECT * FROM task_sub
WHERE task_id = $1
ORDER BY id;
```

```sql
-- 加载短期记忆（崩溃恢复上下文）
SELECT * FROM memory_working WHERE task_id = $1;
```

