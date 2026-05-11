全体系前后端 API 架构，**统一规范、统一字段、统一结构、统一分页、统一鉴权、统一错误**，直接复制保存成 `.md` 即可用。

---

# 悟空（Wukong）多智能体系统 API 架构设计文档
# wukong_api_arch.md

## 一、API 设计总规范
### 1.1 协议
- 采用 **HTTPS**
- 接口风格：**RESTful**
- 数据格式：**JSON**
- 字符集：**UTF-8**

### 1.2 域名规范
```
https://api.wukong-agent.com
```

### 1.3 版本规范
```
/api/v1/xxx
```

### 1.4 公共请求头（所有接口必须携带）
| 头名称 | 说明 | 是否必须 |
|-------|------|----------|
| Authorization | Bearer {token} | 是 |
| Content-Type | application/json | 是 |
| Request-ID | 请求唯一ID（便于追踪） | 否 |
| User-Agent | 客户端标识 | 否 |

### 1.5 Token 规范
- 登录成功返回 **access_token**
- 放在 Header：
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

---

## 二、统一响应结构（全局唯一标准）
### 2.1 成功响应
```json
{
  "code": 200,
  "msg": "success",
  "request_id": "f83e2c91-7b4d-4f0a-9c3e-6d718294a1b2",
  "data": {
    // 业务数据
  }
}
```

### 2.2 失败响应
```json
{
  "code": 400,
  "msg": "参数错误",
  "request_id": "f83e2c91-7b4d-4f0a-9c3e-6d718294a1b2",
  "data": null
}
```

### 2.3 状态码规范
- 200 成功
- 400 参数错误
- 401 未授权/Token过期
- 403 无权限
- 404 资源不存在
- 429 请求限流
- 500 服务异常

---

## 三、统一列表 & 分页结构
### 3.1 分页 URL 设计
```
GET /api/v1/xxx/list?page=1&size=10&sort=created_at&order=desc
```

### 3.2 分页请求参数
| 参数 | 说明 | 默认 |
|------|------|------|
| page | 页码 | 1 |
| size | 每页条数 | 10 |
| sort | 排序字段 | created_at |
| order | 排序方向 | desc |

### 3.3 分页返回结构
```json
{
  "code": 200,
  "msg": "success",
  "request_id": "xxx",
  "data": {
    "list": [
      { ... }
    ],
    "total": 128,
    "page": 1,
    "size": 10,
    "pages": 13
  }
}
```

---

## 四、统一字段命名规范
- 小写下划线：`user_id`, `task_id`, `session_id`
- 时间：`created_at`, `updated_at`
- 状态：`status`
- ID：统一 `xxxId`

---

## 五、模块与 URL 规划
### 5.1 认证模块（Auth）
```
POST   /api/v1/auth/login        登录
POST   /api/v1/auth/logout       登出
GET    /api/v1/auth/profile      获取用户信息
```

### 5.2 对话模块（Chat）
```
POST   /api/v1/chat/session/create    创建会话
GET    /api/v1/chat/session/list      会话列表
POST   /api/v1/chat/message/send      发送消息
GET    /api/v1/chat/message/list      消息列表
```

### 5.3 任务模块（Task）
```
POST   /api/v1/task/create           创建任务
GET    /api/v1/task/list             任务列表
GET    /api/v1/task/detail           任务详情
POST   /api/v1/task/cancel           取消任务
```

### 5.4 子任务模块（SubTask）
```
GET    /api/v1/subtask/list          子任务列表
```

### 5.5 流式消息（Stream）
```
GET    /api/v1/stream/chat           SSE 流式对话
GET    /api/v1/stream/task           SSE 任务执行流
```

### 5.6 记忆模块（Memory）
```
GET    /api/v1/memory/working/list   短期记忆
GET    /api/v1/memory/long/list      长期记忆
```

### 5.7 技能模块（Skill）
```
GET    /api/v1/skill/list            技能列表
```

---

## 六、接口统一入参 & 出参示例（可直接给前端用）
### 6.1 登录
**请求**
```json
{
  "username": "admin",
  "password": "123456"
}
```
**返回**
```json
{
  "code": 200,
  "msg": "success",
  "request_id": "xxx",
  "data": {
    "access_token": "eyJhbGciOiJ...",
    "expire": 7200
  }
}
```

### 6.2 发送对话消息
**请求**
```json
{
  "session_id": "sess_123",
  "content": "你好",
  "skill_name": "chat"
}
```
**返回**
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "msg_id": "msg_456",
    "task_id": "task_789"
  }
}
```

### 6.3 任务列表（分页）
**URL**
```
GET /api/v1/task/list?page=1&size=10
```
**返回**
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "list": [
      {
        "task_id": "task_1",
        "title": "行业报告生成",
        "status": "RUNNING",
        "created_at": "2026-04-06 12:00:00"
      }
    ],
    "total": 25,
    "page": 1,
    "size": 10,
    "pages": 3
  }
}
```

---

## 七、WebSocket / SSE 流式接口规范
### 7.1 SSE 流式对话
```
GET /api/v1/stream/chat?sessionId=sess_123
Header: Authorization: Bearer xxx
```

### 7.2 流式消息格式
```
data: {"type":"THINK","content":"我正在思考..."}

data: {"type":"TOOL","content":"调用搜索中..."}

data: {"type":"CHUNK","content":"你好！"}

data: {"type":"FINISH","content":"DONE"}
```

---

## 八、错误码统一清单
| code | 说明 |
|------|------|
| 400 | 参数错误 |
| 401 | 未登录或 token 过期 |
| 403 | 无权访问 |
| 404 | 接口/资源不存在 |
| 429 | 请求频繁 |
| 500 | 服务内部错误 |
| 1001 | 会话不存在 |
| 1002 | 任务不存在 |
| 1003 | 技能未启用 |
| 1004 | 消息发送失败 |

---

## 九、接口安全规范
- 所有写操作必须鉴权(测试阶段可以不需要)
- 关键操作日志记录
- 限流规则：60次/分钟
- 返回脱敏：不返回敏感信息

---

## 十、前后端对接规范
1. 所有接口 **必须带 token**(测试阶段可以不需要)
2. 所有列表 **必须分页**
3. 所有响应 **必须按统一结构**
4. 流式对话 **必须使用 SSE**
5. 错误信息 **必须友好可读**

# 十一、后端 Go API 路由模板和统一返回工具
### 3.1 后端 Go API 路由模板（Gin 完整版）

文件名：`route/router.go` 

```go
package main

import (
	"github.com/gin-gonic/gin"
	"wukong/handler"
	"wukong/middleware"
)

func InitRouter() *gin.Engine {
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.Cors())       // 跨域
	r.Use(middleware.RequestID())   // 请求ID
	r.Use(middleware.Logger())      // 日志

	// API v1 分组
	v1 := r.Group("/api/v1")
	{
		// 不需要登录
		auth := v1.Group("/auth")
		{
			auth.POST("/login", handler.Login)
			auth.POST("/logout", handler.Logout)
		}

		// 需要 Token 鉴权
		api := v1.Group("/")
		api.Use(middleware.JWTAuth())
		{
			// 对话模块
			chat := api.Group("/chat")
			{
				chat.POST("/session/create", handler.CreateChatSession)
				chat.GET("/session/list", handler.GetChatSessionList)
				chat.POST("/message/send", handler.SendChatMessage)
				chat.GET("/message/list", handler.GetChatMessageList)
			}

			// 任务模块
			task := api.Group("/task")
			{
				task.POST("/create", handler.CreateTask)
				task.GET("/list", handler.GetTaskList)
				task.GET("/detail", handler.GetTaskDetail)
				task.POST("/cancel", handler.CancelTask)
			}

			// 子任务
			task.GET("/subtask/list", handler.GetSubTaskList)

			// 记忆模块
			memory := api.Group("/memory")
			{
				memory.GET("/working/list", handler.GetWorkingMemory)
				memory.GET("/long/list", handler.GetLongMemory)
			}

			// 技能模块
			skill := api.Group("/skill")
			{
				skill.GET("/list", handler.GetSkillList)
			}

			// 流式 SSE
			api.GET("/stream/chat", handler.StreamChat)
			api.GET("/stream/task", handler.StreamTask)
		}
	}

	return r
}
```

### 3.2、Go 统一返回工具类（全局成功/失败）

文件名：`pkg/response/response.go`

```go
package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data"`
}

// 成功
func Success(c *gin.Context, data interface{}) {
	reqID, _ := c.Get("RequestID")
	c.JSON(http.StatusOK, Response{
		Code:      200,
		Msg:       "success",
		RequestID: reqID.(string),
		Data:      data,
	})
}

// 失败
func Fail(c *gin.Context, code int, msg string) {
	reqID, _ := c.Get("RequestID")
	c.JSON(http.StatusOK, Response{
		Code:      code,
		Msg:       msg,
		RequestID: reqID.(string),
		Data:      nil,
	})
}
```

### 3.3、Go 分页统一结构体

```go
type PageReq struct {
	Page  int    `form:"page" default:"1"`
	Size  int    `form:"size" default:"10"`
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

type PageResp struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
	Pages int         `json:"pages"`
}
```

