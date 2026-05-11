悟空（Wukong）多智能体系统

> 基于Go语言的轻量高性能分布式多智能体任务执行系统

## 版本功能

### v0.1 版本 - 地基奠定
- [x] 工程脚手架（Go模块初始化、标准目录结构）
- [x] 配置管理（koanf，支持YAML多环境配置）
- [x] 日志模块（slog结构化日志）
- [x] HTTP框架（Gin）
- [x] 数据库建表SQL（PostgreSQL）
- [x] Redis客户端封装
- [x] 统一响应结构（response）
- [x] 分页结构体（PageReq/PageResp）
- [x] 请求ID中间件、跨域中间件、JWT认证中间件
- [x] 统一错误码
- [x] UUID生成工具、JWT工具
- [x] LLM抽象层（基础版，OpenAI兼容）
- [x] 认证API（登录、登出、用户信息）
- [x] 对话API（会话和消息）

### v0.2 版本 - 任务引擎
- [x] 四叉树任务队列（pkg/queue）
  - 优先级调度（1-10级）
  - 延时任务
  - 幂等去重
  - 内存+队列双存储
- [x] 状态机引擎（pkg/statemachine）
  - 状态转换规则
  - 进入/退出钩子
  - 状态历史记录
- [x] Worker执行单元（pkg/worker）
  - 可配置协程池
  - 心跳上报
  - 任务拉取和执行
  - panic安全捕获
- [x] Manager调度中枢（pkg/manager）
  - 任务入口模块
  - 任务规划模块
  - 调度循环
  - 崩溃恢复
- [x] 任务API（创建、列表、详情、取消）
- [x] 子任务API

## 项目结构

```
wukong/
├── cmd/server/           # 服务入口
├── internal/             # 应用代码
│   ├── handler/         # 处理器（auth, chat, task）
│   ├── middleware/      # 中间件（cors, auth）
│   ├── model/           # 数据模型
│   ├── repository/      # 数据仓库
│   ├── route/           # 路由
│   └── service/         # 服务层
├── pkg/                  # 公共模块（函数选项模式）
│   ├── config/          # 配置管理
│   ├── db/             # 数据库封装
│   ├── errors/         # 统一错误
│   ├── jwt/            # JWT工具
│   ├── logger/         # 日志
│   ├── llm/            # LLM抽象层
│   ├── manager/         # Manager调度中枢
│   ├── queue/          # 四叉树任务队列
│   ├── redis/          # Redis封装
│   ├── response/       # 响应封装
│   ├── statemachine/   # 状态机引擎
│   ├── uuid/           # UUID生成
│   ├── validator/      # 验证器
│   └── worker/         # Worker执行单元
├── configs/             # 配置文件
├── scripts/             # 脚本
└── skills/             # 技能插件
```

## API接口

### 认证模块
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/auth/login | 登录 |
| POST | /api/v1/auth/logout | 登出 |
| GET | /api/v1/auth/profile | 用户信息 |

### 对话模块
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/chat/session/create | 创建会话 |
| GET | /api/v1/chat/session/list | 会话列表 |
| POST | /api/v1/chat/message/send | 发送消息 |
| GET | /api/v1/chat/message/list | 消息列表 |

### 任务模块
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/task/create | 创建任务 |
| GET | /api/v1/task/list | 任务列表 |
| GET | /api/v1/task/detail | 任务详情 |
| POST | /api/v1/task/cancel | 取消任务 |
| GET | /api/v1/subtask/list | 子任务列表 |

## 快速开始

### 1. 启动依赖服务

```bash
docker-compose up -d
```

### 2. 初始化数据库

```bash
# 连接数据库执行 scripts/schema.sql
```

### 3. 运行服务

```bash
go mod tidy
go run cmd/server/main.go
```

### 4. 测试接口

```bash
# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 创建任务
curl -X POST http://localhost:8080/api/v1/task/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"skill_name":"chat","params":{"input":"hello"},"priority":5}'

# 查询任务列表
curl http://localhost:8080/api/v1/task/list \
  -H "Authorization: Bearer <token>"
```

## 设计特点

- **函数选项模式**：pkg目录使用函数选项模式，便于配置和扩展
- **四叉树队列**：优先级任务调度，高效分发
- **状态机引擎**：规范任务状态流转
- **Worker池**：可配置协程池，高并发执行
- **Manager调度**：统一调度中枢，崩溃恢复

## 任务状态流转

```
PENDING → PLANNING → RUNNING → WAITING → COMPLETED
                    ↘ FAILED ↗
                       ↓
                  PENDING (重试)
```

