package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiujuan/wukong/internal/repository"
	"github.com/jiujuan/wukong/internal/route"
	"github.com/jiujuan/wukong/internal/service"
	"github.com/jiujuan/wukong/pkg/config"
	"github.com/jiujuan/wukong/pkg/database"
	"github.com/jiujuan/wukong/pkg/jwt"
	"github.com/jiujuan/wukong/pkg/llm"
	"github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/manager"
	"github.com/jiujuan/wukong/pkg/memory"
	"github.com/jiujuan/wukong/pkg/skills"
	"github.com/jiujuan/wukong/pkg/tool"
	"github.com/jiujuan/wukong/pkg/worker"
)

func main() {
	// 加载配置
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// 初始化日志
	log := logger.New(
		logger.WithFormat(cfg.String("log.format", "json")),
	)
	log.Info("starting wukong server...")

	// 设置Gin模式
	gin.SetMode(cfg.String("server.mode", "debug"))

	// 初始化JWT
	jwtTool := jwt.New(
		jwt.WithSecret(cfg.String("jwt.secret", "wukong-secret-key")),
		jwt.WithExpireHours(cfg.Int("jwt.expire_hours", 2)),
	)

	// 初始化LLM
	providerType := llm.ProviderType(cfg.String("llm.provider", string(llm.ProviderTypeDeepSeek)))
	llmProvider := llm.New(
		llm.WithProviderType(providerType),
		llm.WithBaseURL(cfg.String("llm.base_url", "https://api.deepseek.com/v1")),
		llm.WithAPIKey(cfg.String("llm.api_key", "")),
		llm.WithModel(cfg.String("llm.model", "deepseek-chat")),
		llm.WithTimeout(time.Duration(cfg.Int("llm.timeout", 60))*time.Second),
	)
	if cfg.Bool("llm.pool.enabled", false) {
		members := []llm.PoolMember{
			{
				Name:     "primary",
				Priority: 1,
				Provider: llmProvider,
			},
		}
		fallback1Type := cfg.String("llm.fallback1.provider", "")
		if fallback1Type != "" {
			members = append(members, llm.PoolMember{
				Name:     "fallback1",
				Priority: 2,
				Provider: llm.New(
					llm.WithProviderType(llm.ProviderType(fallback1Type)),
					llm.WithBaseURL(cfg.String("llm.fallback1.base_url", "")),
					llm.WithAPIKey(cfg.String("llm.fallback1.api_key", "")),
					llm.WithModel(cfg.String("llm.fallback1.model", "")),
					llm.WithTimeout(time.Duration(cfg.Int("llm.timeout", 60))*time.Second),
				),
			})
		}
		fallback2Type := cfg.String("llm.fallback2.provider", "")
		if fallback2Type != "" {
			members = append(members, llm.PoolMember{
				Name:     "fallback2",
				Priority: 3,
				Provider: llm.New(
					llm.WithProviderType(llm.ProviderType(fallback2Type)),
					llm.WithBaseURL(cfg.String("llm.fallback2.base_url", "")),
					llm.WithAPIKey(cfg.String("llm.fallback2.api_key", "")),
					llm.WithModel(cfg.String("llm.fallback2.model", "")),
					llm.WithTimeout(time.Duration(cfg.Int("llm.timeout", 60))*time.Second),
				),
			})
		}
		pool := llm.NewProviderPool(
			members,
			llm.WithPoolMaxRetries(cfg.Int("llm.pool.max_retries", 1)),
			llm.WithPoolFailureThreshold(cfg.Int("llm.pool.failure_threshold", 3)),
			llm.WithPoolCooldown(time.Duration(cfg.Int("llm.pool.cooldown_sec", 15))*time.Second),
			llm.WithPoolRetryBackoff(time.Duration(cfg.Int("llm.pool.retry_backoff_ms", 200))*time.Millisecond),
		)
		llmProvider.SetProviderPool(pool)
	}

	var db *database.DB
	var skillRepo *repository.SkillRepository
	var streamRepo *repository.StreamRepository
	var taskRepo *repository.TaskRepository
	dbCfg := database.Config{
		Host:            cfg.String("db.host", "localhost"),
		Port:            uint16(cfg.Int("db.port", 5432)),
		Database:        cfg.String("db.database", "wukong"),
		User:            cfg.String("db.user", "wukong"),
		Password:        cfg.String("db.password", "wukong123"),
		MaxConns:        int32(cfg.Int("db.max_open_conns", 25)),
		MinConns:        int32(cfg.Int("db.max_idle_conns", 5)),
		MaxConnIdle:     time.Duration(cfg.Int("db.conn_max_lifetime", 300)) * time.Second,
		MaxConnLifetime: time.Duration(cfg.Int("db.conn_max_lifetime", 300)) * time.Second,
	}
	db, err = database.New(dbCfg)
	if err != nil {
		log.Warn("init database failed, skill_meta persistence disabled", "error", err)
	} else {
		skillRepo = repository.NewSkillRepository(db)
		streamRepo = repository.NewStreamRepository(db)
		taskRepo = repository.NewTaskRepository(db)
		// if err := taskRepo.EnsureTaskSubColumns(context.Background()); err != nil {
		// 	log.Warn("ensure task_sub columns failed", "error", err)
		// }
	}
	streamService := service.NewStreamService(streamRepo)

	ctx, cancel := context.WithCancel(context.Background())
	skillRegistry := skills.New(
		skills.WithRootDir(cfg.String("skills.root_dir", "skills")),
		skills.WithPollInterval(time.Duration(cfg.Int("skills.poll_interval_sec", 3))*time.Second),
		skills.WithExecTimeout(time.Duration(cfg.Int("skills.exec_timeout_sec", 60))*time.Second),
		skills.WithLogger(log.With()),
		skills.WithMetaStore(skillRepo),
	)
	if err := skillRegistry.Start(ctx); err != nil {
		log.Warn("start skill registry failed", "error", err)
	}
	memoryManager := memory.NewManager(nil, nil)
	toolManager := tool.NewManager(
		tool.WithLogger(log.With()),
		tool.WithLLMProvider(llmProvider),
		tool.WithSkillsRegistry(skillRegistry),
		tool.WithMemoryStore(memoryManager),
		tool.WithBaseDir(cfg.String("skills.root_dir", "skills")),
		tool.WithExecTimeout(time.Duration(cfg.Int("skills.exec_timeout_sec", 60))*time.Second),
	)

	// 初始化Manager
	mgr := manager.NewManager(taskRepo)
	mgr.SetLogger(log.With())
	mgr.SetStreamPublisher(streamService)
	mgr.SetPlanner(manager.NewLLMPlanner(llmProvider, manager.NewTplPlanner()))

	log.Info("init worker pool...")

	// 初始化Worker池
	workerPool := worker.New(
		worker.WithName("worker-pools"),
		worker.WithWorkerCount(cfg.Int("worker.count", 4)),
		worker.WithMaxQueueSize(cfg.Int("worker.queue_size", 1000)),
		worker.WithLogger(log.With()),
		worker.WithTaskHandler(worker.NewRoutedSubTaskHandlerWithTools(llmProvider, log.With(), toolManager, skillRegistry)),
	)

	mgr.SetWorkerPool(workerPool)

	// 启动Manager
	if err := mgr.Start(ctx); err != nil {
		log.Error("start manager failed", "error", err)
	}

	// 初始化Gin引擎
	engine := gin.New()

	// 初始化路由
	router := route.NewRouter(engine, jwtTool, llmProvider, mgr, skillRegistry, memoryManager, streamService, db)
	router.InitRouter()

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.String("server.host", "0.0.0.0"), cfg.Int("server.port", 8080))
	log.Info(fmt.Sprintf("server listening on %s", addr))

	// 启动服务（后台）
	go func() {
		if err := engine.Run(addr); err != nil {
			log.Error(fmt.Sprintf("server start failed: %v", err))
			os.Exit(1)
		}
	}()

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// 停止Manager
	cancel()
	skillRegistry.Stop()
	mgr.Stop()
	if db != nil {
		db.Close()
	}

	log.Info("server stopped")
}
