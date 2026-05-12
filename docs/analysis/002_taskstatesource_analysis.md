002_promptengine_contextengine_system_design.md 中的第9小节，阶段五任务， 

### 阶段五：Task / Planner 场景接入 

目标： - 让任务执行与规划能力统一进入同一套架构 

任务： 

1.实现 `TaskStateSource` 
2.实现 `SkillSpecSource` 
3.接入 `worker` 场景 
4.接入 `planner` 场景 
5.增加前端、后端单元测试 
6.回归测试 7.前后端集成测试



这个阶段五任务的目的，核心就是一句话：

把 `Task` 和 `Planner` 这两条原来比较分散的“上下文拼装 + prompt 组织”逻辑，也拉进和 `Chat` 一样的统一架构里。

具体作用可以分三层看。

**对程序架构的作用**

第一，是把“提示词怎么写”和“上下文从哪里来”彻底拆开。

- `PromptEngine` 管“怎么说”
- `ContextEngine` 管“拿什么来一起说”

这样 `planner`、`worker`、后面的 `react`、`tool_call` 就不会各自偷偷拼字符串、各自读一套上下文，架构会明显更整齐。

第二，是把 `Task` / `Planner` 也纳入 scene 化设计。

这一步之后，系统里会更清楚地分成这些场景：

- `chat`
- `planner`
- `worker`

每个 scene 都有自己的 source 组合和 prompt 模板，但走的是同一条装配流水线。这样后面要加 `react`、`task_trace`、`tool_result`、`rag`，就不是到处打补丁，而是往统一入口加 source / policy。

第三，是为后续“上下文能力升级”做地基。

比如后面你要做这些能力：

- 任务执行时带入主任务状态、前序子任务结果
- planner 带入技能说明、记忆、历史执行痕迹
- token budget 裁剪
- source 优先级控制
- 可观测性和调试

如果没有阶段五，后面这些能力会散在 `worker`、`planner`、`manager` 里，越来越难维护。

**对功能的作用**

阶段五最直接会带来两个功能提升。

1. `planner` 规划更“知情”

以前 `LLMPlanner` 主要拿 `task_id / skill / params` 去规划。  
接入后，它可以统一拿到：

- 当前任务状态
- 当前任务参数摘要
- 技能说明
- 技能允许的工具、参数、记忆配置

这样 LLM 在拆任务时会更稳，不容易“瞎规划”或者忽略 skill 边界。

2. `worker` 执行更“有上下文”

以前 worker 大多是把 subtask 的 action 和 params 直接丢给 prompt。  
接入后，它可以统一带上：

- 主任务状态
- 当前子任务状态
- action / params
- 技能规格
- 可能的前序结果摘要

这样执行链路会更像“基于当前任务现场做事”，而不是“拿一条孤立指令做事”。

**对这个项目长期演进的作用**

这一步其实是把 Wukong 从“能跑的多模块系统”，往“可持续演进的 Agent 平台”推了一步。

因为后面很多高级能力都依赖这个统一入口：

- 多步任务上下文继承
- 子任务结果回灌
- 任务记忆 / 技能记忆
- 统一 prompt 调优
- 不同 scene 的 A/B prompt 实验
- 调试某次 LLM 调用到底看到了哪些上下文

没有阶段五，这些能力都能做，但会越来越乱。  
有了阶段五，你后面是在搭积木，不是在拆墙补墙。

如果你愿意，我下一条可以继续直接给你一版：

“阶段五完成后，`planner -> manager -> worker -> tool/react` 的完整上下文流转图”。