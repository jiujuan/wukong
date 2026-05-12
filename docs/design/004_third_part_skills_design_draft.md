

# 运行第三方Skills

先做一个**最小可用版本**最稳可跑版本，pkg/skills 能运行 https://agentskills.io/specification 这里定义的Agent Skills。

## **目标**

- 先不做在线安装市场
- 先不做自动拉取 npm/GitHub
- 先允许你把别人下载好的 Skill 包，手动放进固定目录
- `pkg/skills` 负责发现和选择
- `pkg/tool` 负责权限和能力执行
- `cmd/sandbox` 负责隔离运行

## ** 目录约定**

```text
wukong/
  skills/
    local/
      my-skill/
        SKILL.md
        scripts/
        references/
    vendor/
      third-party-skill/
        SKILL.md
        scripts/
        references/
```

你手动下载来的包，只要放进 `skills/vendor/<skill-name>/` 就算安装完成。

## **MVP版 设计**

1. `pkg/skills`
   - 扫描固定目录
   - 读取 `SKILL.md`
   - 建立 skill 索引
   - 根据 task 选择 skill
   - 生成执行上下文

2. `pkg/tool`
   - 维护工具白名单
   - 只允许 skill 调用已授权 tool
   - 每次调用检查 skill 身份和权限

3. `cmd/sandbox`
   - 启动 skill 脚本
   - 限制工作目录
   - 限制环境变量
   - 限制网络/文件/命令
   - 通过 stdin/stdout 或 JSON-RPC 和主进程通信

## **最简单的执行流程**

```text
task
 -> 选中 skill 名称
 -> pkg/skills 从固定目录加载 skill
 -> 读取 SKILL.md + scripts
 -> pkg/skills 生成运行参数
 -> cmd/sandbox 启动 skill
 -> skill 需要文件/网络/记忆/LLM 时
 -> 通过 pkg/tool 调用
 -> pkg/tool 校验权限后执行
```

## **这版先支持的能力**

- 本地目录发现
- 手动放包
- `SKILL.md` 解析
- 脚本执行
- tool 白名单
- sandbox 隔离

## **这版先不做**

- npm 自动安装
- GitHub 自动拉取
- 版本市场
- 签名验证
- 自动更新
- 复杂依赖解析

## **我建议的实现顺序**

1. 先定固定目录和 Skill 包格式
2. `pkg/skills` 增加目录扫描和选择逻辑
3. `cmd/sandbox` 先能跑一个最简单脚本
4. `pkg/tool` 把 `ExecuteForSkill` 变成强校验
5. task 流程里接上“选中 skill -> sandbox 执行”

## **一句话版**
这版就是：**手动把 Skill 包放到固定目录，`skills` 负责发现，`tool` 负责授权，`sandbox` 负责隔离，task 负责选择并触发执行。**

