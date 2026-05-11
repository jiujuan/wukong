

## exec_subtask 注释

已经补好了，我在 [exec_subtask.go](D:/codeproject/mygo/wukong/wukong/pkg/worker/exec_subtask.go) 里给这些关键位置加了更详细的注释：

- `executableSubTask`
  - 为什么只定义最小执行协议，不直接绑定 `manager.SubTask`
- `PromptBuilder`
  - 它在执行链里承担的边界角色
- `ActionPromptBuilder`
  - 模板选择逻辑、参数派生逻辑
- `ActionExecutor`
  - 执行策略抽象的意义
- `LLMActionExecutor`
  - 标准 LLM 执行链路
- `SubTaskExecutor`
  - 路由职责、结果写回为什么关键
- `defaultActionExecutors`
  - 有无 toolManager 时的不同策略
- `ToolActionExecutor`
  - 工具直连执行的适用场景
- `CompositeActionExecutor`
  - primary/fallback 的设计目的
- `SkillAwareActionExecutor`
  - skill/tool/fallback 的优先级顺序
- `resolveSkillName / resolveToolName`
  - 为什么要做归一化
- `cloneParams`
  - 为什么执行前要复制 params

我尽量把注释写成“帮助理解设计意图”的类型，没有写成那种每行翻译代码的空注释。

另外我跑了：

```bash
go test ./pkg/worker
```

测试通过。  
如果你愿意，我下一步也可以继续把 [react.go](D:/codeproject/mygo/wukong/wukong/pkg/worker/react.go) 和 [manager.go](D:/codeproject/mygo/wukong/wukong/pkg/manager/manager.go) 的关键流程注释也补成同样风格。