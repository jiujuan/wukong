package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jiujuan/wukong/pkg/asyncdb"
	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/statemachine"
	"github.com/jiujuan/wukong/pkg/uuid"
	"github.com/jiujuan/wukong/pkg/worker"
)

// ============================================================
//  数据结构
// ============================================================

// Task 主任务
type Task struct {
	TaskID     string         `json:"task_id"`
	UserID     string         `json:"user_id"`
	SessionID  string         `json:"session_id,omitempty"`
	SkillName  string         `json:"skill_name"`
	Params     map[string]any `json:"params"`
	Status     string         `json:"status"`
	Priority   int            `json:"priority"`
	RetryCount int            `json:"retry_count"`
	MaxRetry   int            `json:"max_retry"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Result     map[string]any `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// SubTask 子任务，支持 DAG 依赖
type SubTask struct {
	SubTaskID string         `json:"sub_task_id"`
	TaskID    string         `json:"task_id"`
	DependsOn []string       `json:"depends_on"`
	Action    string         `json:"action"`
	Params    map[string]any `json:"params"`
	Status    string         `json:"status"`
	WorkerID  string         `json:"worker_id,omitempty"`
	Result    map[string]any `json:"result,omitempty"` // ← 子任务执行结果
	Error     string         `json:"error,omitempty"`  // ← 子任务错误信息
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type TaskExecLog struct {
	TaskID    string    `json:"task_id"`
	SubTaskID string    `json:"sub_task_id,omitempty"`
	LogType   string    `json:"log_type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetResult 实现 worker.ResultHolder 接口，Worker 通过此接口读取执行结果
func (s *SubTask) GetResult() map[string]any {
	return s.Result
}
func (s *SubTask) GetSubTaskID() string {
	return s.SubTaskID
}
func (s *SubTask) GetTaskID() string {
	return s.TaskID
}
func (s *SubTask) GetAction() string {
	return s.Action
}
func (s *SubTask) GetParams() map[string]any {
	return s.Params
}
func (s *SubTask) SetResult(result map[string]any) {
	s.Result = result
}
func (s *SubTask) SetError(errMsg string) {
	s.Error = errMsg
}
func (s *SubTask) SetUpdatedAt(ts time.Time) {
	s.UpdatedAt = ts
}

// TaskRepository 任务仓库接口
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	UpdateTask(ctx context.Context, task *Task) error
	ListTasks(ctx context.Context, userID string, status string, page, size int) ([]*Task, int64, error)

	CreateSubTask(ctx context.Context, subtask *SubTask) error
	GetSubTasks(ctx context.Context, taskID string) ([]*SubTask, error)
	UpdateSubTask(ctx context.Context, subtask *SubTask) error
	CreateTaskExecLog(ctx context.Context, item *TaskExecLog) error

	LoadPendingTasks(ctx context.Context) ([]*Task, error)
}

type BatchUpsertRepository interface {
	BatchUpsertTasks(ctx context.Context, tasks []*Task) error
	BatchUpsertSubTasks(ctx context.Context, subtasks []*SubTask) error
}

type StreamPublisher interface {
	ReportTaskEvent(ctx context.Context, taskID string, msgType string, content string)
}

// ============================================================
//  Manager
// ============================================================

// Manager 调度中枢：接收任务、规划 DAG、调度子任务、汇总结果
type Manager struct {
	mu             sync.RWMutex
	taskQueue      *queue.Queue
	stateMachine   *statemachine.TaskStateMachine
	workerPool     *worker.Pool
	workerRegistry *worker.WorkerRegistry
	repo           TaskRepository
	batchRepo      BatchUpsertRepository
	asyncWriter    *asyncdb.Writer
	logger         *pkglogger.Logger
	planner        TaskPlanner
	ctx            context.Context
	cancel         context.CancelFunc
	running        bool

	// 内存缓存：避免高频 DB 读写
	taskCache    map[string]*Task
	subTaskCache map[string][]*SubTask // taskID -> []SubTask
	streamer     StreamPublisher
}

// NewManager 创建 Manager
func NewManager(repo TaskRepository) *Manager {
	m := &Manager{
		taskQueue:      queue.New(),
		stateMachine:   statemachine.NewTaskStateMachine(),
		workerRegistry: worker.NewWorkerRegistry(),
		repo:           repo,
		logger:         pkglogger.FromSlog(slog.Default()),
		asyncWriter:    asyncdb.New(asyncdb.WithFlushInterval(20*time.Millisecond), asyncdb.WithMaxBatchSize(128)),
		planner:        NewTplPlanner(),
		taskCache:      make(map[string]*Task),
		subTaskCache:   make(map[string][]*SubTask),
	}
	if batchRepo, ok := repo.(BatchUpsertRepository); ok {
		m.batchRepo = batchRepo
	}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// 构建 WorkerPool，并注入结果回调
	m.workerPool = worker.New(
		worker.WithName("manager-pool"),
		worker.WithResultCallback(m.onSubTaskResult),
		worker.WithTaskEventCallback(m.onWorkerTaskEvent),
	)
	return m
}

// SetLogger 替换日志器
func (m *Manager) SetLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	m.logger = pkglogger.FromSlog(logger)
	if m.asyncWriter != nil {
		m.asyncWriter.SetLogger(logger)
	}
}

func (m *Manager) SetStreamPublisher(streamer StreamPublisher) {
	m.streamer = streamer
}

func (m *Manager) SetPlanner(planner TaskPlanner) {
	if planner == nil {
		return
	}
	m.planner = planner
}

// SetWorkerPool 替换 WorkerPool（必须在 Start 前调用；会自动注入 resultCb）
func (m *Manager) SetWorkerPool(pool *worker.Pool) {
	if pool == nil {
		m.workerPool = worker.New(
			worker.WithName("manager-pool"),
			worker.WithResultCallback(m.onSubTaskResult),
			worker.WithTaskEventCallback(m.onWorkerTaskEvent),
		)
		return
	}
	pool.SetResultCallback(m.onSubTaskResult)
	pool.SetTaskEventCallback(m.onWorkerTaskEvent)
	m.workerPool = pool
}

func (m *Manager) publishTaskEvent(ctx context.Context, taskID string, msgType string, content string) {
	if m == nil || m.streamer == nil || taskID == "" {
		return
	}
	m.streamer.ReportTaskEvent(ctx, taskID, msgType, content)
}

// ============================================================
//  生命周期
// ============================================================

// Start 启动 Manager（幂等）
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("[Manager] starting...")

	// 启动 Worker 池
	if err := m.workerPool.Start(); err != nil {
		return fmt.Errorf("start worker pool: %w", err)
	}

	// 加载数据库中的未完成任务（崩溃恢复）
	if err := m.loadPendingTasks(ctx); err != nil {
		m.logger.Error("[Manager] load pending tasks failed", "error", err)
	}

	// 启动调度循环
	go m.scheduleLoop()
	// 启动过期 Worker 清理循环
	go m.cleanWorkerLoop()

	m.logger.Info("[Manager] started")
	return nil
}

// Stop 停止 Manager
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	m.cancel()
	m.workerPool.Stop()
	if m.asyncWriter != nil {
		_ = m.asyncWriter.Stop(context.Background())
	}
	m.logger.Info("[Manager] stopped")
}

// ============================================================
//  任务 CRUD
// ============================================================

// CreateTask 创建主任务并入队
func (m *Manager) CreateTask(ctx context.Context,
	userID, sessionID, skillName string,
	params map[string]any, priority int,
) (*Task, error) {
	task := &Task{
		TaskID:    uuid.NewTaskID(),
		UserID:    userID,
		SessionID: sessionID,
		SkillName: skillName,
		Params:    params,
		Status:    statemachine.StatusPending,
		Priority:  priority,
		MaxRetry:  3,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if m.repo != nil {
		if err := m.persistCreateTask(ctx, task); err != nil {
			return nil, fmt.Errorf("persist task: %w", err)
		}
	}

	m.mu.Lock()
	m.taskCache[task.TaskID] = task
	m.mu.Unlock()

	m.stateMachine.SetInitialState(task.TaskID, statemachine.StatusPending)
	m.taskQueue.Push(&queue.Task{
		TaskID:   task.TaskID,
		Priority: task.Priority,
		Data:     task,
	})

	m.logger.Info("[Manager] task created",
		"task_id", task.TaskID, "skill", task.SkillName, "priority", priority)
	m.logTaskExec(ctx, task.TaskID, "", "INFO", "task created and queued")
	return task, nil
}

// GetTask 读任务（先走缓存，缓存 miss 再查 DB）
func (m *Manager) GetTask(ctx context.Context, taskID string) (*Task, error) {
	m.mu.RLock()
	task, ok := m.taskCache[taskID]
	m.mu.RUnlock()
	if ok {
		return task, nil
	}
	if m.repo != nil {
		return m.repo.GetTask(ctx, taskID)
	}
	return nil, nil
}

// UpdateTaskStatus 通过状态机更新主任务状态，同步缓存和 DB
func (m *Manager) UpdateTaskStatus(ctx context.Context, taskID, status, reason string) error {
	from := m.stateMachine.GetState(taskID)
	if err := m.stateMachine.ChangeState(taskID, status, reason); err != nil {
		m.logger.Error("[Manager] task state transition failed",
			"task_id", taskID, "from", from, "to", status, "reason", reason, "error", err)
		return fmt.Errorf("state transition %s: %w", taskID, err)
	}

	m.mu.Lock()
	if task, ok := m.taskCache[taskID]; ok {
		task.Status = status
		task.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	if m.repo != nil {
		task, _ := m.GetTask(ctx, taskID)
		if task != nil {
			task.Status = status
			task.UpdatedAt = time.Now()
			if err := m.persistUpdateTask(ctx, task); err != nil {
				return err
			}
		}
	}
	m.publishTaskEvent(ctx, taskID, "STATUS", fmt.Sprintf(`{"status":"%s","reason":"%s"}`, status, reason))
	m.logTaskExec(ctx, taskID, "", "INFO", fmt.Sprintf("task status changed: %s -> %s, reason=%s", from, status, reason))
	switch status {
	case statemachine.StatusCompleted, statemachine.StatusFailed, statemachine.StatusCancelled:
		m.publishTaskEvent(ctx, taskID, "FINISH", fmt.Sprintf(`{"status":"%s","reason":"%s"}`, status, reason))
	}
	m.logger.Info("[Manager] task status updated",
		"task_id", taskID, "from", from, "to", status, "reason", reason)
	return nil
}

// CancelTask 取消主任务
func (m *Manager) CancelTask(ctx context.Context, taskID string) error {
	m.logger.Warn("[Manager] cancel task requested", "task_id", taskID)
	m.taskQueue.Remove(taskID)
	return m.UpdateTaskStatus(ctx, taskID, statemachine.StatusCancelled, "cancelled by user")
}

func (m *Manager) InjectTaskInstruction(ctx context.Context, taskID string, content string) error {
	if taskID == "" {
		return fmt.Errorf("task id empty")
	}
	m.mu.Lock()
	task := m.taskCache[taskID]
	if task != nil {
		if task.Params == nil {
			task.Params = map[string]any{}
		}
		list, _ := task.Params["injected_instructions"].([]string)
		list = append(list, content)
		task.Params["injected_instructions"] = list
		task.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	if task != nil && m.repo != nil {
		if err := m.persistUpdateTask(ctx, task); err != nil {
			return err
		}
	}
	m.publishTaskEvent(ctx, taskID, "THINK", content)
	return nil
}

// GetTaskState 获取主任务当前状态
func (m *Manager) GetTaskState(taskID string) string {
	return m.stateMachine.GetState(taskID)
}

// SubmitTask 直接将任务入队（不走 CreateTask，用于外部已创建的任务）
func (m *Manager) SubmitTask(task *Task) bool {
	ok := m.taskQueue.Push(&queue.Task{
		TaskID:   task.TaskID,
		Priority: task.Priority,
		Data:     task,
	})
	if ok {
		m.logger.Info("[Manager] task submitted", "task_id", task.TaskID, "priority", task.Priority)
	} else {
		m.logger.Warn("[Manager] task submit rejected", "task_id", task.TaskID, "priority", task.Priority)
	}
	return ok
}

// GetQueueSize 获取主任务队列大小
func (m *Manager) GetQueueSize() int { return m.taskQueue.Size() }

// ============================================================
//  子任务 CRUD
// ============================================================

// CreateSubTask 创建子任务，写缓存和 DB
func (m *Manager) CreateSubTask(ctx context.Context,
	taskID, action string, dependsOn []string, params map[string]any,
) (*SubTask, error) {
	return m.createSubTask(ctx, "", taskID, action, dependsOn, params)
}

func (m *Manager) createSubTask(ctx context.Context,
	subTaskID string, taskID, action string, dependsOn []string, params map[string]any,
) (*SubTask, error) {
	if strings.TrimSpace(subTaskID) == "" {
		subTaskID = uuid.NewSubTaskID()
	}
	subtask := &SubTask{
		SubTaskID: subTaskID,
		TaskID:    taskID,
		Action:    action,
		DependsOn: dependsOn,
		Params:    params,
		Status:    statemachine.SubStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if m.repo != nil {
		if err := m.persistCreateSubTask(ctx, subtask); err != nil {
			return nil, fmt.Errorf("persist subtask: %w", err)
		}
	}

	m.mu.Lock()
	if _, ok := m.subTaskCache[taskID]; !ok {
		m.subTaskCache[taskID] = make([]*SubTask, 0)
	}
	m.subTaskCache[taskID] = append(m.subTaskCache[taskID], subtask)
	m.mu.Unlock()
	m.logTaskExec(ctx, taskID, subtask.SubTaskID, "INFO", fmt.Sprintf("subtask created action=%s", action))

	return subtask, nil
}

// GetSubTasks 读子任务列表（先走缓存）
func (m *Manager) GetSubTasks(ctx context.Context, taskID string) ([]*SubTask, error) {
	m.mu.RLock()
	subtasks, ok := m.subTaskCache[taskID]
	m.mu.RUnlock()
	if ok {
		return subtasks, nil
	}
	if m.repo != nil {
		return m.repo.GetSubTasks(ctx, taskID)
	}
	return nil, nil
}

// ============================================================
//  调度循环
// ============================================================

// scheduleLoop 每秒从主任务队列弹出任务，规划子任务并分发
func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.processTasks()
		}
	}
}

// processTasks 从主任务队列取出所有可处理的任务
func (m *Manager) processTasks() {
	for {
		qTask := m.taskQueue.Pop()
		if qTask == nil {
			break
		}
		m.logger.Info("[Manager] scheduling task", "task_id", qTask.TaskID, "priority", qTask.Priority)
		current := m.GetTaskState(qTask.TaskID)
		if current == "" {
			current = statemachine.StatusPending
			m.stateMachine.SetInitialState(qTask.TaskID, current)
		}
		if current == statemachine.StatusRunning || current == statemachine.StatusWaiting {
			m.logger.Info("[Manager] task already active, continue dispatch only",
				"task_id", qTask.TaskID, "state", current)
			go m.dispatchReadySubTasks(qTask.TaskID)
			continue
		}
		if current == statemachine.StatusPlanning {
			subtasks, err := m.GetSubTasks(m.ctx, qTask.TaskID)
			if err == nil && len(subtasks) > 0 {
				m.logger.Info("[Manager] task already planned, moving to RUNNING",
					"task_id", qTask.TaskID, "subtask_count", len(subtasks))
				if err := m.UpdateTaskStatus(m.ctx, qTask.TaskID, statemachine.StatusRunning, "resume planned task"); err != nil {
					m.logger.Error("[Manager] PLANNING→RUNNING failed",
						"task_id", qTask.TaskID, "error", err)
					continue
				}
				go m.dispatchReadySubTasks(qTask.TaskID)
				continue
			}
		}
		if current != statemachine.StatusPending && current != statemachine.StatusPlanning {
			m.logger.Warn("[Manager] skip scheduling task with unexpected state",
				"task_id", qTask.TaskID, "state", current)
			continue
		}

		// PENDING → PLANNING
		if current == statemachine.StatusPending {
			if err := m.UpdateTaskStatus(m.ctx, qTask.TaskID,
				statemachine.StatusPlanning, "scheduling"); err != nil {
				m.logger.Error("[Manager] PENDING→PLANNING failed",
					"task_id", qTask.TaskID, "error", err)
				continue
			}
		}

		// 规划：拆解为子任务 DAG
		if err := m.planTask(qTask); err != nil {
			m.logger.Error("[Manager] plan task failed",
				"task_id", qTask.TaskID, "error", err)
			m.UpdateTaskStatus(m.ctx, qTask.TaskID, statemachine.StatusFailed, err.Error())
			continue
		}

		// PLANNING → RUNNING
		if err := m.UpdateTaskStatus(m.ctx, qTask.TaskID,
			statemachine.StatusRunning, "planned"); err != nil {
			m.logger.Error("[Manager] PLANNING→RUNNING failed",
				"task_id", qTask.TaskID, "error", err)
			continue
		}

		// 非阻塞分发子任务
		go m.dispatchReadySubTasks(qTask.TaskID)
	}
}

// planTask 调用 Planner 拆解主任务为子任务，持久化并写入缓存
func (m *Manager) planTask(qTask *queue.Task) error {
	taskData, ok := qTask.Data.(*Task)
	if !ok {
		return fmt.Errorf("invalid task data type")
	}

	if m.planner == nil {
		m.planner = NewTplPlanner()
	}
	m.logger.Info("[Manager] start task planning",
		"task_id", taskData.TaskID, "planner", m.planner.Name(), "skill", taskData.SkillName)
	m.publishTaskEvent(m.ctx, taskData.TaskID, "STATUS", fmt.Sprintf(`{"planner":"%s","phase":"start"}`, m.planner.Name()))
	planCtx := WithPlanReporter(m.ctx, func(msgType string, content string) {
		m.publishTaskEvent(m.ctx, taskData.TaskID, msgType, content)
	})
	defs, err := m.planner.PlanSubTasks(planCtx, taskData)
	if err != nil {
		return fmt.Errorf("planner error: %w", err)
	}

	for _, def := range defs {
		taskID := def.TaskID
		if strings.TrimSpace(taskID) == "" {
			taskID = taskData.TaskID
		}
		if _, err := m.createSubTask(m.ctx, def.SubTaskID, taskID, def.Action, def.DependsOn, def.Params); err != nil {
			return fmt.Errorf("create subtask %s: %w", def.SubTaskID, err)
		}
	}

	m.logger.Info("[Manager] task planned",
		"task_id", qTask.TaskID, "subtask_count", len(defs))
	m.publishTaskEvent(m.ctx, taskData.TaskID, "STATUS", fmt.Sprintf(`{"planner":"%s","phase":"done","subtasks":%d}`, m.planner.Name(), len(defs)))
	return nil
}

// ============================================================
//  子任务分发（DAG 驱动）
// ============================================================

// dispatchReadySubTasks 扫描 taskID 下所有 PENDING 且依赖已满足的子任务，提交到 WorkerPool
func (m *Manager) dispatchReadySubTasks(taskID string) {
	subtasks, err := m.GetSubTasks(m.ctx, taskID)
	if err != nil {
		m.logger.Error("[Manager] load subtasks failed", "task_id", taskID, "error", err)
		m.UpdateTaskStatus(m.ctx, taskID, statemachine.StatusFailed, "load subtasks failed")
		return
	}
	if len(subtasks) == 0 {
		m.logger.Warn("[Manager] no subtasks found, marking task completed",
			"task_id", taskID)
		m.UpdateTaskStatus(m.ctx, taskID, statemachine.StatusCompleted, "no subtasks")
		return
	}

	dispatched := 0
	for _, st := range subtasks {
		if m.canExecute(subtasks, st) {
			m.submitSubTask(st)
			dispatched++
		}
	}

	if dispatched == 0 {
		// 没有可分发的子任务：可能全部已经在跑或已完成
		m.logger.Debug("[Manager] no ready subtasks at this tick", "task_id", taskID)
		return
	}
	m.logger.Info("[Manager] subtasks dispatched", "task_id", taskID, "dispatched_count", dispatched)
}

// canExecute 判断子任务是否满足 DAG 执行条件：
//   - 自身状态为 PENDING
//   - 所有 DependsOn 的子任务状态均为 SUCCESS
func (m *Manager) canExecute(all []*SubTask, st *SubTask) bool {
	if st.Status != statemachine.SubStatusPending {
		return false
	}
	for _, depID := range st.DependsOn {
		dep := findInSlice(all, depID)
		if dep == nil || dep.Status != statemachine.SubStatusSuccess {
			return false
		}
	}
	return true
}

// submitSubTask 将子任务状态置为 RUNNING 并提交到 WorkerPool
func (m *Manager) submitSubTask(st *SubTask) {
	st.Status = statemachine.SubStatusRunning
	st.UpdatedAt = time.Now()
	if m.repo != nil {
		if err := m.persistUpdateSubTask(m.ctx, st); err != nil {
			m.logger.Error("[Manager] persist subtask status failed", "subtask_id", st.SubTaskID, "error", err)
		}
	}
	m.updateSubTaskInCache(st)

	qTask := &queue.Task{
		TaskID:   st.SubTaskID,
		Priority: 5,
		Data:     st, // SubTask 实现了 ResultHolder 接口
	}

	if m.workerPool.Submit(qTask) {
		m.logger.Info("[Manager] subtask submitted to pool",
			"task_id", st.TaskID, "subtask_id", st.SubTaskID, "action", st.Action)
		m.logTaskExec(m.ctx, st.TaskID, st.SubTaskID, "INFO", fmt.Sprintf("subtask submitted to worker pool action=%s", st.Action))
	} else {
		m.logger.Error("[Manager] failed to submit subtask, marking FAILED",
			"subtask_id", st.SubTaskID)
		st.Status = statemachine.SubStatusFailed
		st.Error = "worker pool rejected submission"
		st.UpdatedAt = time.Now()
		if m.repo != nil {
			if err := m.persistUpdateSubTask(m.ctx, st); err != nil {
				m.logger.Error("[Manager] persist subtask failed-reject failed", "subtask_id", st.SubTaskID, "error", err)
			}
		}
		m.updateSubTaskInCache(st)
		m.logTaskExec(m.ctx, st.TaskID, st.SubTaskID, "ERROR", "subtask submit rejected by worker pool")
		// 子任务失败后立即尝试聚合（可能整个主任务需要 FAILED）
		m.tryAggregateTask(st.TaskID)
	}
}

// ============================================================
//  结果回调（Worker → Manager）
// ============================================================

// onSubTaskResult 是注入到 WorkerPool 的 ResultCallback。
// Worker 每完成或彻底失败一个子任务时调用此函数。
//
// 职责：
//  1. 更新子任务状态（SUCCESS / FAILED）及结果字段
//  2. 持久化子任务到 DB
//  3. 检查是否有新的子任务因依赖满足而可以继续分发
//  4. 检查主任务是否全部子任务完成，若是则聚合结果并更新主任务状态
func (m *Manager) onSubTaskResult(subTaskID string, success bool, result map[string]any, errMsg string) {
	// 找到对应的子任务
	st := m.findSubTaskByID(subTaskID)
	if st == nil {
		m.logger.Warn("[Manager] onSubTaskResult: subtask not found in cache",
			"subtask_id", subTaskID)
		return
	}

	taskID := st.TaskID

	// ① 更新子任务状态和结果
	if success {
		st.Status = statemachine.SubStatusSuccess
		st.Result = result
		st.Error = ""
	} else {
		st.Status = statemachine.SubStatusFailed
		st.Error = errMsg
	}
	st.UpdatedAt = time.Now()

	// ② 持久化子任务
	if m.repo != nil {
		if err := m.persistUpdateSubTask(m.ctx, st); err != nil {
			m.logger.Error("[Manager] persist subtask result failed",
				"subtask_id", subTaskID, "error", err)
		}
	}
	m.updateSubTaskInCache(st)

	m.logger.Info("[Manager] subtask result received",
		"task_id", taskID, "subtask_id", subTaskID,
		"success", success, "error", errMsg)
	if success {
		m.logTaskExec(m.ctx, taskID, subTaskID, "INFO", "subtask finished successfully")
	} else {
		m.logTaskExec(m.ctx, taskID, subTaskID, "ERROR", fmt.Sprintf("subtask finished with error: %s", errMsg))
	}
	m.emitResultEvents(taskID, result, errMsg)

	// ③ 继续分发 DAG 中的下游子任务（成功时才能解锁依赖）
	if success {
		go m.dispatchReadySubTasks(taskID)
	}

	// ④ 尝试聚合：检查整个主任务是否完结
	m.tryAggregateTask(taskID)
}

func (m *Manager) emitResultEvents(taskID string, result map[string]any, errMsg string) {
	if taskID == "" {
		return
	}
	if errMsg != "" {
		m.publishTaskEvent(m.ctx, taskID, "TOOL", errMsg)
		return
	}
	if result == nil {
		return
	}
	if rawSteps, ok := result["react_steps"].([]any); ok {
		for _, row := range rawSteps {
			data, ok := row.(map[string]any)
			if !ok {
				continue
			}
			if thought, ok := data["thought"].(string); ok && thought != "" {
				m.publishTaskEvent(m.ctx, taskID, "THINK", thought)
			}
			if toolName, ok := data["tool_name"].(string); ok && toolName != "" {
				raw, _ := json.Marshal(data)
				m.publishTaskEvent(m.ctx, taskID, "TOOL", string(raw))
			}
		}
	}
	if out, ok := result["output"].(string); ok && out != "" {
		m.publishTaskEvent(m.ctx, taskID, "CHUNK", out)
		return
	}
	raw, _ := json.Marshal(result)
	m.publishTaskEvent(m.ctx, taskID, "CHUNK", string(raw))
}

func (m *Manager) onWorkerTaskEvent(subTaskID string, logType string, content string) {
	if subTaskID == "" || content == "" {
		return
	}
	st := m.findSubTaskByID(subTaskID)
	if st == nil {
		return
	}
	m.logTaskExec(m.ctx, st.TaskID, subTaskID, logType, content)
}

// ============================================================
//  结果聚合（核心：子任务全部完成后汇总主任务结果）
// ============================================================

// tryAggregateTask 检查 taskID 下所有子任务是否全部终态，
// 若是则聚合结果并将主任务标记为 COMPLETED 或 FAILED。
//
// 聚合规则：
//   - 所有子任务均为 SUCCESS → 主任务 COMPLETED，合并各子任务 Result
//   - 任意子任务为 FAILED    → 主任务 FAILED，错误信息汇总
//   - 存在子任务仍在 PENDING/RUNNING → 什么都不做（等待）
func (m *Manager) tryAggregateTask(taskID string) {
	subtasks, err := m.GetSubTasks(m.ctx, taskID)
	if err != nil || len(subtasks) == 0 {
		return
	}

	// 检查是否全部终态
	allDone := true
	anyFailed := false
	failedIDs := make([]string, 0)

	for _, st := range subtasks {
		switch st.Status {
		case statemachine.SubStatusSuccess:
			// OK
		case statemachine.SubStatusFailed:
			anyFailed = true
			failedIDs = append(failedIDs, st.SubTaskID)
		case statemachine.SubStatusSkipped:
			// 跳过也算终态
		default:
			// PENDING / RUNNING：尚未完成，不能聚合
			allDone = false
		}
	}

	if !allDone {
		return // 还有子任务在跑，等待下次回调
	}

	// —— 全部终态，开始聚合 ——

	task, _ := m.GetTask(m.ctx, taskID)
	if task == nil {
		m.logger.Warn("[Manager] aggregate: main task not found", "task_id", taskID)
		return
	}

	// 确认主任务当前还在 RUNNING 状态（避免重复聚合）
	currentState := m.stateMachine.GetState(taskID)
	if currentState != statemachine.StatusRunning {
		m.logger.Debug("[Manager] aggregate: task already in terminal state",
			"task_id", taskID, "state", currentState)
		return
	}

	if anyFailed {
		// 主任务失败
		errSummary := fmt.Sprintf("subtasks failed: %v", failedIDs)
		task.Error = errSummary
		task.UpdatedAt = time.Now()

		m.mu.Lock()
		m.taskCache[taskID] = task
		m.mu.Unlock()

		if m.repo != nil {
			if err := m.persistUpdateTask(m.ctx, task); err != nil {
				m.logger.Error("[Manager] persist failed task failed", "task_id", taskID, "error", err)
			}
		}

		// 状态机：RUNNING → FAILED
		m.UpdateTaskStatus(m.ctx, taskID, statemachine.StatusFailed, errSummary)

		m.logger.Error("[Manager] task FAILED",
			"task_id", taskID, "failed_subtasks", failedIDs)
	} else {
		// 所有子任务成功：合并结果
		aggregated := m.aggregateResults(subtasks)
		task.Result = aggregated
		task.Error = ""
		task.UpdatedAt = time.Now()

		m.mu.Lock()
		m.taskCache[taskID] = task
		m.mu.Unlock()

		if m.repo != nil {
			if err := m.persistUpdateTask(m.ctx, task); err != nil {
				m.logger.Error("[Manager] persist completed task failed", "task_id", taskID, "error", err)
			}
		}

		// 状态机：RUNNING → COMPLETED
		m.UpdateTaskStatus(m.ctx, taskID, statemachine.StatusCompleted, "all subtasks succeeded")

		m.logger.Info("[Manager] task COMPLETED",
			"task_id", taskID,
			"subtask_count", len(subtasks),
			"result_keys", resultKeys(aggregated))
	}
}

// aggregateResults 将所有子任务的 Result 合并为主任务的最终结果。
//
// 合并策略：
//   - 每个子任务的结果以其 Action 为 key 存入顶层 map，避免同名 key 覆盖。
//   - 若多个子任务有相同 Action（并行 DAG 场景），以 subTaskID 为 key。
//   - 同时生成 "summary" 字段：按执行顺序记录每个子任务的 action 和结果摘要。
func (m *Manager) aggregateResults(subtasks []*SubTask) map[string]any {
	merged := make(map[string]any)
	summary := make([]map[string]any, 0, len(subtasks))
	actionCount := make(map[string]int)

	for _, st := range subtasks {
		if st.Status != statemachine.SubStatusSuccess {
			continue
		}
		actionCount[st.Action]++
	}

	for _, st := range subtasks {
		if st.Status != statemachine.SubStatusSuccess {
			continue
		}

		// 确定 key：若 action 唯一则用 action，否则用 subTaskID 避免覆盖
		key := st.Action
		if actionCount[st.Action] > 1 {
			key = st.SubTaskID
		}

		if st.Result != nil {
			merged[key] = st.Result
		}

		// 生成摘要条目
		entry := map[string]any{
			"sub_task_id": st.SubTaskID,
			"action":      st.Action,
			"status":      st.Status,
		}
		if st.Result != nil {
			entry["result"] = st.Result
		}
		summary = append(summary, entry)
	}

	merged["_summary"] = summary
	merged["_completed_at"] = time.Now().Format(time.RFC3339)
	merged["_subtask_count"] = len(subtasks)

	return merged
}

// ============================================================
//  辅助方法
// ============================================================

// findSubTaskByID 在所有缓存的子任务中按 SubTaskID 查找
func (m *Manager) findSubTaskByID(subTaskID string) *SubTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, subtasks := range m.subTaskCache {
		for _, st := range subtasks {
			if st.SubTaskID == subTaskID {
				return st
			}
		}
	}
	return nil
}

// findSubTask 在某个主任务的子任务列表中按 SubTaskID 查找
func (m *Manager) findSubTask(taskID, subTaskID string) *SubTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	subtasks, ok := m.subTaskCache[taskID]
	if !ok {
		return nil
	}
	return findInSlice(subtasks, subTaskID)
}

// updateSubTaskInCache 更新缓存中的子任务对象（原地修改，不替换指针）
func (m *Manager) updateSubTaskInCache(st *SubTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subtasks, ok := m.subTaskCache[st.TaskID]
	if !ok {
		return
	}
	for i, s := range subtasks {
		if s.SubTaskID == st.SubTaskID {
			subtasks[i] = st
			return
		}
	}
}

// findInSlice 线性查找子任务切片
func findInSlice(subtasks []*SubTask, subTaskID string) *SubTask {
	for _, st := range subtasks {
		if st.SubTaskID == subTaskID {
			return st
		}
	}
	return nil
}

// resultKeys 返回 map 的所有 key，用于日志输出
func resultKeys(m map[string]any) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m *Manager) persistCreateTask(ctx context.Context, task *Task) error {
	if m.repo == nil || m.asyncWriter == nil {
		return nil
	}
	snapshot := cloneTask(task)
	return m.asyncWriter.Submit(ctx, asyncdb.Job{
		Name: "create_task",
		Run: func(jobCtx context.Context) error {
			return m.repo.CreateTask(jobCtx, snapshot)
		},
	})
}

func (m *Manager) persistUpdateTask(ctx context.Context, task *Task) error {
	if m.repo == nil || m.asyncWriter == nil {
		return nil
	}
	snapshot := cloneTask(task)
	if m.batchRepo != nil {
		return m.asyncWriter.Submit(ctx, asyncdb.Job{
			Name:     "batch_upsert_task",
			Group:    "task_info_upsert",
			Key:      snapshot.TaskID,
			Payload:  snapshot,
			BatchRun: m.runBatchUpsertTasks,
		})
	}
	return m.asyncWriter.Submit(ctx, asyncdb.Job{
		Name: "update_task",
		Key:  "update_task:" + snapshot.TaskID,
		Run: func(jobCtx context.Context) error {
			return m.repo.UpdateTask(jobCtx, snapshot)
		},
	})
}

func (m *Manager) persistCreateSubTask(ctx context.Context, subtask *SubTask) error {
	if m.repo == nil || m.asyncWriter == nil {
		return nil
	}
	snapshot := cloneSubTask(subtask)
	return m.asyncWriter.Submit(ctx, asyncdb.Job{
		Name: "create_subtask",
		Run: func(jobCtx context.Context) error {
			return m.repo.CreateSubTask(jobCtx, snapshot)
		},
	})
}

func (m *Manager) persistUpdateSubTask(ctx context.Context, subtask *SubTask) error {
	if m.repo == nil || m.asyncWriter == nil {
		return nil
	}
	snapshot := cloneSubTask(subtask)
	if m.batchRepo != nil {
		return m.asyncWriter.Submit(ctx, asyncdb.Job{
			Name:     "batch_upsert_subtask",
			Group:    "task_sub_upsert",
			Key:      snapshot.SubTaskID,
			Payload:  snapshot,
			BatchRun: m.runBatchUpsertSubTasks,
		})
	}
	return m.asyncWriter.Submit(ctx, asyncdb.Job{
		Name: "update_subtask",
		Key:  "update_subtask:" + snapshot.SubTaskID,
		Run: func(jobCtx context.Context) error {
			return m.repo.UpdateSubTask(jobCtx, snapshot)
		},
	})
}

func (m *Manager) persistCreateTaskExecLog(ctx context.Context, item *TaskExecLog) error {
	if m.repo == nil || m.asyncWriter == nil {
		return nil
	}
	snapshot := *item
	return m.asyncWriter.Submit(ctx, asyncdb.Job{
		Name: "create_task_exec_log",
		Run: func(jobCtx context.Context) error {
			return m.repo.CreateTaskExecLog(jobCtx, &snapshot)
		},
	})
}

func (m *Manager) logTaskExec(ctx context.Context, taskID string, subTaskID string, logType string, content string) {
	if taskID == "" || logType == "" || content == "" {
		return
	}
	item := &TaskExecLog{
		TaskID:    taskID,
		SubTaskID: subTaskID,
		LogType:   logType,
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := m.persistCreateTaskExecLog(ctx, item); err != nil {
		m.logger.Error("[Manager] persist task exec log failed",
			"task_id", taskID, "sub_task_id", subTaskID, "log_type", logType, "error", err)
	}
}

func cloneTask(src *Task) *Task {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Params = cloneMap(src.Params)
	dst.Result = cloneMap(src.Result)
	return &dst
}

func cloneSubTask(src *SubTask) *SubTask {
	if src == nil {
		return nil
	}
	dst := *src
	if src.DependsOn != nil {
		dst.DependsOn = append([]string(nil), src.DependsOn...)
	}
	dst.Params = cloneMap(src.Params)
	dst.Result = cloneMap(src.Result)
	return &dst
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (m *Manager) runBatchUpsertTasks(ctx context.Context, jobs []asyncdb.Job) error {
	tasks := make([]*Task, 0, len(jobs))
	for _, job := range jobs {
		task, ok := job.Payload.(*Task)
		if !ok || task == nil {
			continue
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		return nil
	}
	return m.batchRepo.BatchUpsertTasks(ctx, tasks)
}

func (m *Manager) runBatchUpsertSubTasks(ctx context.Context, jobs []asyncdb.Job) error {
	subtasks := make([]*SubTask, 0, len(jobs))
	for _, job := range jobs {
		subtask, ok := job.Payload.(*SubTask)
		if !ok || subtask == nil {
			continue
		}
		subtasks = append(subtasks, subtask)
	}
	if len(subtasks) == 0 {
		return nil
	}
	return m.batchRepo.BatchUpsertSubTasks(ctx, subtasks)
}

// ============================================================
//  崩溃恢复：启动时加载未完成任务
// ============================================================

// loadPendingTasks 从 DB 加载 PENDING/PLANNING/RUNNING/WAITING 的任务入内存队列
func (m *Manager) loadPendingTasks(ctx context.Context) error {
	if m.repo == nil {
		return nil
	}
	tasks, err := m.repo.LoadPendingTasks(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		m.stateMachine.SetInitialState(task.TaskID, task.Status)
		m.mu.Lock()
		m.taskCache[task.TaskID] = task
		m.mu.Unlock()
		m.taskQueue.Push(&queue.Task{
			TaskID:   task.TaskID,
			Priority: task.Priority,
			Status:   task.Status,
			Data:     task,
		})
	}
	m.logger.Info("[Manager] loaded pending tasks", "count", len(tasks))
	return nil
}

// ============================================================
//  Worker 心跳管理
// ============================================================

// RegisterWorker 注册 Worker 心跳
func (m *Manager) RegisterWorker(heartbeat *worker.Heartbeat) {
	m.workerRegistry.Register(heartbeat)
}

// UnregisterWorker 注销 Worker
func (m *Manager) UnregisterWorker(workerID string) {
	m.workerRegistry.Unregister(workerID)
}

// cleanWorkerLoop 定时清理超时未更新的 Worker 记录
func (m *Manager) cleanWorkerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			removed := m.workerRegistry.CleanStale(60 * time.Second)
			if removed > 0 {
				m.logger.Info("[Manager] cleaned stale workers", "count", removed)
			}
		}
	}
}

// ============================================================
//  Task 序列化工具
// ============================================================

// ToJSON 序列化主任务为 JSON 字符串
func (t *Task) ToJSON() (string, error) {
	data, err := json.Marshal(t)
	return string(data), err
}
