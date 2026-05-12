# 第三方 Skills 与 Sandbox 安全架构分析

## 1. 背景

当前 Wukong 已经具备本地 Skills 和 Tools 的基础能力：

- `pkg/skills` 负责加载、解析、注册和执行 Skill。
- `pkg/tool` 负责注册和执行宿主提供的工具能力。
- Skill 可以声明自己允许使用哪些 Tool。
- Worker 在执行 Task/SubTask 时，根据 Skill 的工具白名单调用 Tool。

如果后续希望除了使用自己的 Skills，还能使用别人发布在 npm、GitHub 或其他仓库中的 Skills，那么系统会从“本地可控插件”进入“第三方插件生态”。这时最大的变化不是安装方式，而是安全边界。

第三方 Skill 可能带来以下风险：

- 下载来源不可信。
- 版本漂移导致行为不可复现。
- Skill 脚本直接读取本地文件、环境变量或密钥。
- Skill 通过网络外传数据。
- Skill 调用高风险 Tool，例如 `code_exec`、`file_write`、`http_request`。
- npm/GitHub 包存在供应链攻击。
- Skill 执行时消耗大量 CPU、内存或输出日志。

因此，完整设计应当由三部分共同完成：

```text
pkg/skills  = 技能包管理、来源校验、版本锁定、权限声明
pkg/tool    = 宿主能力代理、运行时权限检查、工具调用审计
cmd/sandbox = 第三方代码隔离执行、资源限制、系统级防护
```

Sandbox 很重要，但它不是全部安全体系。它解决的是“代码运行时能碰到什么”，不能替代 Skill 包管理、权限模型和工具授权。

## 2. 总体架构判断

推荐把未来的第三方 Skill 执行链路设计为：

```text
用户请求
  -> Agent/Worker 选择 Skill
  -> pkg/skills 解析 Skill 身份、版本、来源和 manifest
  -> pkg/skills 生成 SkillRuntimePolicy
  -> cmd/sandbox 启动隔离运行环境
  -> Skill 在 sandbox 内执行
  -> Skill 通过 JSON-RPC/stdin-stdout 请求工具调用
  -> pkg/tool 根据 SkillContext 做权限校验
  -> Tool 执行并返回结果
  -> Skill 生成最终结果
```

这里有两层安全边界：

1. `cmd/sandbox` 是系统层隔离。
2. `pkg/tool` 是业务层授权。

只做 sandbox，权限会太粗。例如一个 Skill 可以联网，但到底能不能访问 `api.github.com`、能不能访问内网地址、能不能写 memory，仍然需要业务策略判断。

只做 Tool 授权也不够。因为第三方 Skill 如果直接在宿主进程或普通 shell 中运行，可能绕过 Tool 直接读文件、访问环境变量或发起网络请求。

因此，第三方 Skill 应该同时满足：

- 代码运行在 sandbox 中。
- 能力调用必须经过 Tool Broker。
- 每次 Tool 调用必须携带 Skill 身份和授权上下文。

## 3. pkg/skills 设计

`pkg/skills` 应该从本地 `SKILL.md` 加载器，升级成 Skill 包管理系统。

### 3.1 Skill Manifest

当前 `SKILL.md` 适合给人和模型阅读，但不适合作为长期稳定的机器协议。建议保留 `SKILL.md`，同时新增机器可解析的 manifest，例如 `wukong.skill.json`：

```json
{
  "name": "@alice/code-review",
  "version": "1.2.0",
  "description": "Review Go code changes",
  "runtime": "node",
  "entry": "dist/index.js",
  "params": {},
  "tools": ["file_read", "llm_chat", "memory_read"],
  "permissions": {
    "filesystem": {
      "read": ["workspace"],
      "write": []
    },
    "network": {
      "allow": ["api.github.com"]
    },
    "shell": false,
    "memory": {
      "read": true,
      "write": false
    }
  }
}
```

`SKILL.md` 负责说明能力、用法、提示词和示例；`wukong.skill.json` 负责描述包身份、版本、入口、运行时、权限和工具依赖。

### 3.2 Skill 身份模型

Skill 不能只用 `SkillName` 标识。第三方生态下，同名 Skill 可能来自不同来源、不同作者和不同版本。

建议定义：

```go
type SkillID struct {
    Namespace string
    Name      string
    Version   string
    Source    string
}
```

示例：

```text
local:summary@dev
github:alice/review@8af31c
npm:@wukong/translate@1.3.0
```

运行时所有日志、审计、Tool 授权都应该使用完整 Skill 身份，而不是只使用名称。

### 3.3 Source Provider

建议为不同来源定义统一接口：

```go
type SourceProvider interface {
    Resolve(ctx context.Context, ref SkillRef) (*ResolvedSkill, error)
    Fetch(ctx context.Context, resolved *ResolvedSkill, dst string) error
    Verify(ctx context.Context, resolved *ResolvedSkill, path string) error
}
```

可以逐步实现：

- `LocalProvider`：加载本地 Skill 目录。
- `GitHubProvider`：支持 `github:owner/repo/path@tag` 或 commit SHA。
- `NpmProvider`：支持 `npm:@scope/pkg@version`。

GitHub 来源应当解析到确定 commit SHA。npm 来源应当解析到确定版本和 tarball integrity。运行时不应该依赖可变 tag 或 semver range。

### 3.4 Installer

安装流程建议：

```text
解析来源 ref
  -> Resolve 到确定版本
  -> 下载到 staging 目录
  -> 校验 manifest
  -> 校验入口文件
  -> 校验权限声明
  -> 计算 checksum
  -> 原子移动到正式安装目录
  -> 写入 skills.lock
  -> 触发 Registry reload
```

推荐目录结构：

```text
skills/
  local/
  vendor/
    github/
      owner/
        repo/
          skill-name/
            commit-sha/
    npm/
      scope/
        package/
          version/
skills.lock
```

`skills.lock` 用于保证可复现：

```json
{
  "skills": [
    {
      "name": "@alice/code-review",
      "version": "1.2.0",
      "sourceType": "npm",
      "sourceRef": "npm:@alice/code-review@1.2.0",
      "resolvedRef": "https://registry.npmjs.org/@alice/code-review/-/code-review-1.2.0.tgz",
      "checksum": "sha256:...",
      "enabled": true
    }
  ]
}
```

### 3.5 Registry

Registry 应从“扫描目录得到 Skill map”升级为“已安装 Skill 的运行时注册表”。

建议提供能力：

- `Install(ref)`
- `Enable(skillID)`
- `Disable(skillID)`
- `Remove(skillID)`
- `Resolve(name)`
- `GetManifest(skillID)`
- `CanUseTool(skillID, toolName)`
- `GrantedPermissions(skillID)`
- `BuildRuntimePolicy(skillID)`

Registry 的职责是回答：

- 这个 Skill 是否存在？
- 这个版本是否启用？
- 它从哪里安装？
- 它声明了哪些 Tool？
- 它被授予了哪些权限？
- 它应该用什么 sandbox policy 执行？

## 4. pkg/tool 设计

`pkg/tool` 应该保持为宿主能力层，不建议一开始就允许第三方动态注册 Go Tool。第三方 Skill 想读文件、联网、调用 LLM 或写记忆，都应该通过宿主 Tool Broker。

### 4.1 强制 Skill 身份校验

`ExecuteForSkill` 必须强制要求 Skill 存在。第三方生态下，未知 Skill 不应该绕过权限检查。

建议逻辑：

```go
func (m *Manager) ExecuteForSkill(ctx context.Context, skillID skills.SkillID, toolName string, params map[string]any) (map[string]any, error) {
    skill, ok := m.skillsRegistry.Get(skillID)
    if !ok {
        return nil, ErrUnknownSkill
    }
    if !m.skillsRegistry.CanUseTool(skillID, toolName) {
        return nil, ErrToolNotAllowed
    }
    return m.executeWithPolicy(ctx, skill, toolName, params)
}
```

这样可以避免伪造 skillName 直接执行宿主 Tool。

### 4.2 Tool Capability

每个 Tool 应声明自己的能力类型和风险等级：

```go
type Capability string

const (
    CapabilityFileRead  Capability = "filesystem.read"
    CapabilityFileWrite Capability = "filesystem.write"
    CapabilityHTTP      Capability = "network.http"
    CapabilityShell     Capability = "shell.exec"
    CapabilityLLM       Capability = "llm.call"
    CapabilityMemory    Capability = "memory.access"
)
```

Tool 接口可以逐步演进为：

```go
type Tool interface {
    Name() string
    Description() string
    ParameterSchema() map[string]any
    Capability() Capability
    RiskLevel() RiskLevel
    Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}
```

### 4.3 SkillContext

Tool 执行时应传入 Skill 上下文：

```go
type SkillContext struct {
    SkillID     skills.SkillID
    SourceType  string
    Version     string
    TrustLevel  string
    Permissions skills.Permissions
    Workspace   string
    SessionID   string
}
```

每个 Tool 根据 SkillContext 做进一步限制：

- `file_read` 检查读路径是否在授权 root 下。
- `file_write` 检查写路径是否在 Skill workspace 下。
- `http_request` 检查域名白名单和内网访问限制。
- `memory_write` 检查是否允许写入当前用户/session namespace。
- `code_exec` 检查是否允许 shell 或命令执行。

### 4.4 Tool Broker

第三方 Skill 不应该直接调用 Go 函数。建议由 sandbox runner 提供 JSON-RPC/stdin-stdout 协议：

```json
{
  "method": "tool.call",
  "params": {
    "tool": "file_read",
    "args": {
      "path": "README.md"
    }
  }
}
```

宿主收到请求后：

```text
解析请求
  -> 读取 SkillContext
  -> Registry 检查 Tool 白名单
  -> ToolManager 检查 capability 权限
  -> 具体 Tool 做参数级策略校验
  -> 执行
  -> 返回结构化结果
```

这可以确保第三方 Skill 的所有宿主能力都经过统一审计。

## 5. cmd/sandbox 独立程序设计

Sandbox 应该作为独立程序存在，例如：

```text
cmd/sandbox/
  main.go

pkg/sandbox/
  runner.go
  protocol.go
  policy.go
  filesystem.go
  network.go
  process.go
```

它的职责不是理解业务，而是提供受限执行环境。

### 5.1 SandboxRequest

```go
type SandboxRequest struct {
    SkillID     string
    Entry       string
    Runtime     string
    Params      map[string]any
    Policy      SandboxPolicy
    Workspace   string
    TimeoutSec  int
}
```

### 5.2 SandboxPolicy

```go
type SandboxPolicy struct {
    FileSystem  FileSystemPolicy
    Network     NetworkPolicy
    Process     ProcessPolicy
    Env         map[string]string
    MaxMemoryMB int
    MaxOutputKB int
}

type FileSystemPolicy struct {
    ReadOnlyRoots []string
    WritableRoots []string
}

type NetworkPolicy struct {
    Enabled      bool
    AllowedHosts []string
}

type ProcessPolicy struct {
    AllowShell      bool
    AllowedCommands []string
}
```

### 5.3 Sandbox 能解决的问题

Sandbox 可以明显缓解：

- 第三方 Skill 任意读取本地文件。
- 第三方 Skill 读取环境变量或密钥。
- 第三方 Skill 任意写工作区。
- 第三方 Skill 任意执行 shell 命令。
- 第三方 Skill 任意联网外传数据。
- 第三方 Skill 占满 CPU、内存或输出。
- 第三方 npm/GitHub 包被攻击后的影响范围。

### 5.4 Sandbox 不能单独解决的问题

Sandbox 不能替代：

- 包来源校验。
- 版本锁定。
- checksum/integrity 校验。
- Skill 权限声明。
- Tool 运行时授权。
- Tool 调用审计。
- LLM 上下文泄露治理。
- 合法网络请求中的数据外传风险。

因此，Sandbox 必须和 `pkg/skills`、`pkg/tool` 一起工作。

## 6. npm Skills 设计

npm Skill 包建议结构：

```text
package.json
wukong.skill.json
SKILL.md
dist/index.js
```

安装命令示例：

```bash
wukong skill install npm:@scope/skill-name@1.2.0
```

安全策略：

- 不直接执行 `npm install` 生命周期脚本。
- 通过 registry 下载 tarball。
- 校验 npm integrity。
- 解压到 staging。
- 校验 `wukong.skill.json`。
- 校验 entry 文件存在。
- 写入 `skills.lock`。
- 运行时由 sandbox 执行 entry。

这样可以避免 `postinstall` 等生命周期脚本带来的供应链风险。

## 7. GitHub Skills 设计

GitHub Skill 安装命令示例：

```bash
wukong skill install github:owner/repo@v1.0.0
wukong skill install github:owner/repo/path/to/skill@commitSHA
```

安全策略：

- tag 必须解析为 commit SHA。
- lockfile 记录 resolved commit SHA。
- 下载 archive 或 clone 到 staging。
- 校验 manifest 和入口文件。
- 计算 checksum。
- 安装后运行时只使用已锁定版本。

不建议长期依赖可变 tag。真正可复现的单位应该是 commit SHA + checksum。

## 8. 推荐落地路线

### 8.1 第一阶段：安全骨架

优先完成：

- 增加 `wukong.skill.json`。
- 增加 `SkillID`、`SkillSource`、`SkillInstaller`。
- 增加 `skills.lock`。
- 改造 `ExecuteForSkill`，禁止 unknown skill 绕过权限。
- Tool 增加 capability 和 risk level。
- 实现基础 `SkillContext`。
- 支持 local + GitHub commit 安装。

### 8.2 第二阶段：Sandbox Runner

完成：

- 新增 `cmd/sandbox`。
- Skill 执行统一通过 sandbox runner。
- 主进程和 sandbox 通过 JSON-RPC/stdin-stdout 通信。
- 默认禁用网络。
- 默认禁用 shell。
- 默认只暴露最小环境变量。
- 限制超时、输出大小和工作目录。

### 8.3 第三阶段：npm 支持

完成：

- npm package resolve。
- tarball 下载。
- integrity 校验。
- 禁止 lifecycle scripts。
- manifest 校验。
- lockfile 记录 resolved tarball 和 checksum。

### 8.4 第四阶段：权限和审计增强

完成：

- 文件路径级权限。
- 网络域名白名单。
- memory namespace 隔离。
- 高风险 Tool 二次授权。
- Skill install/update/run 审计日志。
- Tool call 审计日志。

### 8.5 第五阶段：生态能力

完成：

- Skill marketplace/index。
- 作者、签名、评分、下载量。
- 自动更新检查。
- trust level。
- 前端 Skill 安装、启用、禁用、升级和授权页面。

## 9. 结论

增加一个 sandbox 独立程序是必要的，它能显著降低第三方 Skill 带来的运行时风险。但它不能单独解决所有安全问题。

完整设计应该是：

```text
pkg/skills
  负责 Skill 是谁、从哪来、哪个版本、是否启用、声明了什么权限。

pkg/tool
  负责宿主能力是否允许被调用，以及每一次调用是否符合策略。

cmd/sandbox
  负责第三方代码在受限环境中运行，限制文件、网络、进程和资源。
```

三者配合后，Wukong 的 Skill 体系才能从“本地脚本插件”升级为“可安装、可治理、可审计、可隔离的第三方能力生态”。
