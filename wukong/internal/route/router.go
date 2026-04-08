package route

import (
	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/handler"
	"github.com/jiujuan/wukong/internal/middleware"
	"github.com/jiujuan/wukong/internal/repository"
	"github.com/jiujuan/wukong/internal/service"
	"github.com/jiujuan/wukong/pkg/database"
	"github.com/jiujuan/wukong/pkg/jwt"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/manager"
	"github.com/jiujuan/wukong/pkg/memory"
	"github.com/jiujuan/wukong/pkg/skills"
)

// Router 路由管理
type Router struct {
	engine        *gin.Engine
	jwtTool       *jwt.JWT
	llmProvider   *llm.Provider
	manager       *manager.Manager
	authHandler   *handler.AuthHandler
	chatHandler   *handler.ChatHandler
	taskHandler   *handler.TaskHandler
	skillHandler  *handler.SkillHandler
	memoryHandler *handler.MemoryHandler
	streamHandler *handler.StreamHandler
}

// NewRouter 创建路由实例
func NewRouter(engine *gin.Engine, jwtTool *jwt.JWT, llmProvider *llm.Provider, mgr *manager.Manager, skillRegistry *skills.Registry, memoryManager *memory.Manager, streamService *service.StreamService, db *database.DB) *Router {
	r := &Router{
		engine:      engine,
		jwtTool:     jwtTool,
		llmProvider: llmProvider,
		manager:     mgr,
	}

	// 初始化处理器
	r.authHandler = handler.NewAuthHandler(r.jwtTool)

	// 初始化任务服务
	taskRepo := repository.NewTaskRepository(db)
	taskService := service.NewTaskService(mgr, taskRepo)
	chatRepo := repository.NewChatRepository(db)
	chatService := service.NewChatService(chatRepo, r.llmProvider, streamService)
	r.chatHandler = handler.NewChatHandler(chatService)
	r.taskHandler = handler.NewTaskHandler(taskService)
	skillRepo := repository.NewSkillRepository(db)
	skillService := service.NewSkillService(skillRepo, skillRegistry)
	r.skillHandler = handler.NewSkillHandler(skillService)
	memoryService := service.NewMemoryService(memoryManager)
	r.memoryHandler = handler.NewMemoryHandler(memoryService)
	streamAppService := service.NewStreamAppService(streamService, taskService)
	r.streamHandler = handler.NewStreamHandler(streamAppService)

	return r
}

// InitRouter 初始化路由
func (r *Router) InitRouter() *gin.Engine {
	if r.engine == nil {
		r.engine = gin.Default()
	}

	// 全局中间件
	r.engine.Use(middleware.Cors())
	r.engine.Use(middleware.RequestID())
	r.engine.Use(middleware.Recovery())

	// API v1 分组
	v1 := r.engine.Group("/api/v1")
	{
		// 不需要登录的路由
		auth := v1.Group("/auth")
		{
			auth.POST("/login", r.authHandler.Login)
		}

		// 需要登录的路由
		api := v1.Group("/")
		api.Use(middleware.JWTAuth(r.jwtTool))
		{
			// 认证
			api.POST("/auth/logout", r.authHandler.Logout)
			api.GET("/auth/profile", r.authHandler.Profile)

			// 对话模块
			chat := api.Group("/chat")
			{
				chat.POST("/session/create", r.chatHandler.CreateSession)
				chat.GET("/session/list", r.chatHandler.GetSessionList)
				chat.POST("/session/delete", r.chatHandler.DeleteSession)
				chat.DELETE("/session/delete", r.chatHandler.DeleteSession)
				chat.POST("/message/send", r.chatHandler.SendMessage)
				chat.GET("/message/list", r.chatHandler.GetMessageList)
			}

			// 任务模块
			task := api.Group("/task")
			{
				task.POST("/create", r.taskHandler.CreateTask)
				task.GET("/list", r.taskHandler.ListTasks)
				task.GET("/detail", r.taskHandler.Detail)
				task.POST("/cancel", r.taskHandler.Cancel)
			}

			// 子任务
			api.GET("/subtask/list", r.taskHandler.ListSubTasks)

			skill := api.Group("/skill")
			{
				skill.GET("/list", r.skillHandler.ListSkills)
			}

			mem := api.Group("/memory")
			{
				mem.GET("/working/list", r.memoryHandler.ListWorking)
				mem.GET("/long/list", r.memoryHandler.ListLong)
			}

			stream := api.Group("/stream")
			{
				stream.GET("/chat", r.streamHandler.ChatSSE)
				stream.GET("/task", r.streamHandler.TaskSSE)
				stream.GET("/ws/task", r.streamHandler.TaskWebSocket)
			}
		}
	}

	// 健康检查
	r.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r.engine
}
