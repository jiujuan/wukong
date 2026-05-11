对应文档 [002_promptengine_contextengine_system_design.md](../design/002_promptengine_contextengine_system_design.md) 中的 9. 分阶段实施计划

## 阶段一已经落地 promptengine

我新增了 [promptengine.go](D:/codeproject/mygo/wukong/wukong/pkg/prompt/promptengine.go) 和 [promptengine_test.go](D:/codeproject/mygo/wukong/wukong/pkg/prompt/promptengine_test.go)，实现了这几项核心能力：

- `Engine / Template / MessageTemplate / RenderInput`
- `New()`
- `Register()`
- `Get()`
- `Render()`
- 基于 `{{var}}` 的简单变量渲染
- 缺失变量检测与 `MissingVariablesError`

这版保持得比较轻，先只做 prompt 注册与渲染，不把 PromptEngine 提前做胖。`RenderInput.Context` 也预留了入口，当前支持简单的 `map[string]any` / `map[string]string` / `string` 展开，方便后面接 `ContextEngine`。

单测也补齐了，覆盖了：

- 模板注册成功
- 重复注册报错
- 渲染成功
- 缺变量报错
- 模板不存在报错

验收结果：
```bash
go test ./pkg/prompt
```
已通过。

下一步最顺的就是第二阶段，把 `pkg/worker/exec_subtask.go` 先接进这个 PromptEngine，把里面散落的 prompt builder 收一层。



builder 收一层。

2 files changed+372-0



wukong/pkg/prompt/promptengine.go

wukong/pkg/prompt/promptengine_test.go



---

## 阶段二已经完成，PromptEngine 首批接入

阶段二已经完成，PromptEngine 首批接入到了这三处低风险模块：

- [exec_subtask.go](D:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go)
- [react.go](D:/codeproject/mygo/wukong/wukong/pkg/worker/react.go)
- [llm_planner.go](D:/codeproject/mygo/wukong/wukong/pkg/manager/llm_planner.go)

我顺手把 [promptengine.go](D:/codeproject/mygo/wukong/wukong/pkg/prompt/promptengine.go) 也补成了可接入版本，新增了：

- `NewDefaultEngine()`
- `MustRegister()`
- `RegisterBuiltins()`
- 内置模板 key：
  - `worker.action.default`
  - `worker.action.web_search`
  - `worker.action.report_gen`
  - `worker.react.default`
  - `planner.task.default`

这次接入后的效果是：

- `exec_subtask` 不再手写 action prompt 字符串，统一走 PromptEngine 渲染
- `react` 的 system/user prompt 改成通过 PromptEngine 生成
- `llm_planner` 的 planner prompt 也改成通过 PromptEngine 生成
- 原来的执行逻辑、回退逻辑、接口返回结构都保持不变

测试和回归我也补齐并跑过了：

1. PromptEngine 单测
2. `pkg/worker` 单测
3. `pkg/manager` 单测
4. 后端 API 回归：
   - `go test ./internal/handler ./internal/route ./internal/service ./internal/repository`
5. 前端构建回归：
   - `npm run build`

验收结果都通过了。

这轮里比较关键的新增测试点有：

- PromptEngine builtin 模板注册测试
- `exec_subtask` 验证 prompt builder 输出仍能正确喂给 LLM
- `react` 验证首轮消息已经来自统一模板
- `llm_planner` 验证发送给 LLM 的 planner messages 是标准 `system + user` 结构，且包含 task/skill 信息

如果继续往下做，下一步最合适的就是阶段三：把 `ContextEngine` 骨架建起来，然后优先接 Chat 链路。





## 阶段三：ContextEngine 骨架

完成了，`ContextEngine` 阶段三骨架已经落到代码里，并且测试通过。

这次新增了两个文件：
- [contextengine.go](D:/codeproject/mygo/wukong/wukong/pkg/context/contextengine.go)
- [contextengine_test.go](D:/codeproject/mygo/wukong/wukong/pkg/context/contextengine_test.go)

实现内容包括：
- `BuildRequest / ContextBlock / ContextBundle / Source / Policy / SceneConfig / Engine`
- `RegisterSource / RegisterPolicy / RegisterScene / GetScene / Build`
- 基础 pipeline：
  - 解析 scene
  - 解析 source / policy
  - 加载上下文块
  - 过滤空内容并补齐默认 source
  - 执行 policy
  - 再次归一化
  - 稳定排序
  - 输出 `Blocks / Named / Text / Meta`

测试也补齐了，覆盖了：
- 注册和读取 scene
- 重复注册报错
- 完整 build pipeline
- source / policy 执行错误
- scene 或依赖缺失场景

验证结果：
```text
go test ./pkg/context
ok  	github.com/jiujuan/wukong/pkg/context	0.641s

go test ./pkg/prompt ./pkg/context
ok  	github.com/jiujuan/wukong/pkg/prompt	(cached)
ok  	github.com/jiujuan/wukong/pkg/context	(cached)
```

这一阶段现在可以算完成。下一步最自然的衔接，就是进入“首批 Source / Policy 实现”，把 `chat_message`、`chat_memory` 这些真实上下文源接进来。