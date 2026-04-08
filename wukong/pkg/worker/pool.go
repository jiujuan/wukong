package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/queue"
	"github.com/jiujuan/wukong/pkg/statemachine"
)

// TaskHandler 任务处理器函数类型
// 入参是 queue.Task，其中 Data 字段是 *manager.SubTask（由 Manager.executeSubTask 填入）。
// 处理器将执行结果写入实现了 ResultHolder 接口的 Data，Manager 通过 ResultCallback 接收。
type TaskHandler func(ctx context.Context, task *queue.Task) error

// ResultCallback 子任务执行结果回调
// Pool 在每个子任务执行结束（成功/失败超重试）后调用，通知 Manager 驱动 DAG 继续推进。
// subTaskID: 子任务ID
// success:   是否成功
// result:    执行结果（可 nil）
// errMsg:    失败时的错误信息
type ResultCallback func(subTaskID string, success bool, result map[string]any, errMsg string)
type TaskEventCallback func(subTaskID string, logType string, content string)

// Option 函数选项模式
type Option func(*Pool)

// Pool Worker 协程池
type Pool struct {
	mu           sync.RWMutex
	name         string
	workerCount  int
	maxQueueSize int
	idleTimeout  time.Duration
	maxRetries   int
	execTimeout  time.Duration // 单任务执行超时，默认 5 分钟

	queue        *queue.Queue
	stateMachine *statemachine.TaskStateMachine
	taskHandler  TaskHandler    // 真正执行业务逻辑的函数，由外部注入
	resultCb     ResultCallback // 子任务结果回调，由 Manager 注入
	taskEventCb  TaskEventCallback

	workers   map[int]*worker
	ctx       context.Context
	cancel    context.CancelFunc
	running   atomic.Bool
	logger    *pkglogger.Logger
	wg        sync.WaitGroup
	idCounter int32

	// 指标统计（原子操作，线程安全）
	totalSubmitted atomic.Int64
	totalCompleted atomic.Int64
	totalFailed    atomic.Int64
	totalRetried   atomic.Int64
}

// worker 单个 goroutine
type worker struct {
	id      int
	pool    *Pool
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
}

// PoolStats 池运行时统计快照
type PoolStats struct {
	Name           string
	WorkerCount    int
	QueueSize      int
	TotalSubmitted int64
	TotalCompleted int64
	TotalFailed    int64
	TotalRetried   int64
}

// ---------- 构造 ----------

// New 创建 Worker 池（默认 4 workers，队列容量 1000，单任务 5 分钟超时）
func New(opts ...Option) *Pool {
	p := &Pool{
		workerCount:  4,
		maxQueueSize: 1000,
		idleTimeout:  60 * time.Second,
		maxRetries:   3,
		execTimeout:  5 * time.Minute,
		queue:        queue.New(),
		logger:       pkglogger.FromSlog(slog.Default()),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.stateMachine = statemachine.NewTaskStateMachine(
		statemachine.WithTransition(statemachine.StatusPending, statemachine.StatusRunning),
	)
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.workers = make(map[int]*worker)
	return p
}

// ---------- Option 函数 ----------

func WithName(name string) Option {
	return func(p *Pool) { p.name = name }
}
func WithWorkerCount(count int) Option {
	return func(p *Pool) { p.workerCount = count }
}
func WithMaxQueueSize(size int) Option {
	return func(p *Pool) { p.maxQueueSize = size }
}
func WithIdleTimeout(timeout time.Duration) Option {
	return func(p *Pool) { p.idleTimeout = timeout }
}
func WithMaxRetries(retries int) Option {
	return func(p *Pool) { p.maxRetries = retries }
}

// WithExecTimeout 设置单个子任务执行超时
func WithExecTimeout(timeout time.Duration) Option {
	return func(p *Pool) { p.execTimeout = timeout }
}
func WithLogger(logger *slog.Logger) Option {
	return func(p *Pool) { p.logger = pkglogger.FromSlog(logger) }
}

// WithTaskHandler 注入任务执行函数（必须在 Start 前调用）
// handler 是真正的业务执行逻辑：读取 task.Data(*SubTask) 的 Action/Params，
// 完成后将结果写回 task.Data 的 Result 字段（通过实现 ResultHolder 接口）。
func WithTaskHandler(handler TaskHandler) Option {
	return func(p *Pool) { p.taskHandler = handler }
}

// WithResultCallback 注入子任务结果回调（由 Manager 调用，用于 DAG 推进和结果汇总）
func WithResultCallback(cb ResultCallback) Option {
	return func(p *Pool) { p.resultCb = cb }
}
func WithTaskEventCallback(cb TaskEventCallback) Option {
	return func(p *Pool) { p.taskEventCb = cb }
}

// ---------- 生命周期 ----------

// Start 启动 Worker 池，幂等
func (p *Pool) Start() error {
	if p.running.Load() {
		return nil
	}
	if p.taskHandler == nil {
		p.logger.Warn("[Pool] taskHandler not set — tasks will be treated as no-op")
	}
	p.running.Store(true)
	workerIDs := make([]int, 0, p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		w := p.newWorker()
		p.mu.Lock()
		p.workers[w.id] = w
		p.mu.Unlock()
		workerIDs = append(workerIDs, w.id)
		w.start()
	}
	p.logger.Info("[Pool] started",
		"name", p.name, "workers", p.workerCount,
		"worker_ids", workerIDs,
		"exec_timeout", p.execTimeout, "max_retries", p.maxRetries)
	return nil
}

// Stop 优雅停止：取消上下文，等待所有正在执行的 worker goroutine 自然退出
func (p *Pool) Stop() {
	if !p.running.Load() {
		return
	}
	p.running.Store(false)
	p.cancel()
	p.queue.Close()

	p.mu.Lock()
	for _, w := range p.workers {
		w.stop()
	}
	p.mu.Unlock()

	p.wg.Wait()

	p.mu.Lock()
	p.workers = make(map[int]*worker)
	p.mu.Unlock()

	p.logger.Info("[Pool] stopped",
		"name", p.name,
		"total_completed", p.totalCompleted.Load(),
		"total_failed", p.totalFailed.Load(),
		"total_retried", p.totalRetried.Load(),
	)
}

// ---------- 提交 ----------

// Submit 提交即时任务。队列已满返回 false，Pool 未启动返回 false。
func (p *Pool) Submit(task *queue.Task) bool {
	if !p.running.Load() {
		p.logger.Warn("[Pool] Submit called but pool is not running", "task_id", task.TaskID)
		return false
	}
	if p.queue.Size() >= p.maxQueueSize {
		p.logger.Warn("[Pool] queue full, task rejected",
			"task_id", task.TaskID, "size", p.queue.Size())
		return false
	}
	p.stateMachine.SetInitialState(task.TaskID, statemachine.StatusPending)
	qTask := &queue.Task{
		TaskID:     task.TaskID,
		Priority:   task.Priority,
		Status:     statemachine.StatusPending,
		RetryCount: task.RetryCount,
		ExecuteAt:  time.Now(),
		CreatedAt:  time.Now(),
		Data:       task.Data,
	}
	if p.queue.Push(qTask) {
		p.totalSubmitted.Add(1)
		p.logger.Info("[Pool] task submitted",
			"task_id", task.TaskID, "priority", task.Priority, "queue_size", p.queue.Size())
		return true
	}
	p.logger.Warn("[Pool] task submit failed", "task_id", task.TaskID)
	return false
}

// SubmitDelay 提交延时任务
func (p *Pool) SubmitDelay(task *queue.Task, delay time.Duration) bool {
	if !p.running.Load() {
		p.logger.Warn("[Pool] submit delay called but pool is not running", "task_id", task.TaskID)
		return false
	}
	if p.queue.Size() >= p.maxQueueSize {
		p.logger.Warn("[Pool] queue full, delayed task rejected",
			"task_id", task.TaskID, "size", p.queue.Size())
		return false
	}
	p.stateMachine.SetInitialState(task.TaskID, statemachine.StatusPending)
	qTask := &queue.Task{
		TaskID:     task.TaskID,
		Priority:   task.Priority,
		Status:     statemachine.StatusPending,
		RetryCount: task.RetryCount,
		ExecuteAt:  time.Now().Add(delay),
		CreatedAt:  time.Now(),
		Data:       task.Data,
	}
	if p.queue.Push(qTask) {
		p.totalSubmitted.Add(1)
		p.logger.Info("[Pool] delayed task submitted",
			"task_id", task.TaskID, "priority", task.Priority, "delay", delay.String())
		return true
	}
	p.logger.Warn("[Pool] delayed task submit failed", "task_id", task.TaskID)
	return false
}

// Cancel 从队列中撤销尚未被 worker 取走的任务
func (p *Pool) Cancel(taskID string) bool {
	removed := p.queue.Remove(taskID)
	if removed {
		if err := p.changeTaskState(taskID, statemachine.StatusCancelled, "cancelled by user"); err != nil {
			p.logger.Error("[Pool] cancel task state change failed", "task_id", taskID, "error", err)
		}
		p.logger.Info("[Pool] task cancelled", "task_id", taskID)
	} else {
		p.logger.Warn("[Pool] cancel task missed", "task_id", taskID)
	}
	return removed
}

// ---------- 查询 ----------

func (p *Pool) GetTaskState(taskID string) string { return p.stateMachine.GetState(taskID) }
func (p *Pool) GetQueueSize() int                 { return p.queue.Size() }
func (p *Pool) IsRunning() bool                   { return p.running.Load() }
func (p *Pool) SetTaskHandler(handler TaskHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskHandler = handler
}
func (p *Pool) SetResultCallback(cb ResultCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resultCb = cb
}
func (p *Pool) SetTaskEventCallback(cb TaskEventCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.taskEventCb = cb
}

// Stats 返回当前池的运行快照（非阻塞）
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Name:           p.name,
		WorkerCount:    p.workerCount,
		QueueSize:      p.queue.Size(),
		TotalSubmitted: p.totalSubmitted.Load(),
		TotalCompleted: p.totalCompleted.Load(),
		TotalFailed:    p.totalFailed.Load(),
		TotalRetried:   p.totalRetried.Load(),
	}
}

// ---------- Worker 内部实现 ----------

func (p *Pool) newWorker() *worker {
	id := int(atomic.AddInt32(&p.idCounter, 1))
	ctx, cancel := context.WithCancel(p.ctx)
	return &worker{id: id, pool: p, ctx: ctx, cancel: cancel}
}

func (w *worker) start() {
	w.pool.wg.Add(1)
	go func() {
		defer w.pool.wg.Done()
		w.running.Store(true)
		w.pool.logger.Debug("[Worker] started", "id", w.id, "pool", w.pool.name)

		for w.running.Load() {
			task := w.pool.queue.Pop()
			if task != nil {
				w.executeTask(task)
				continue
			}
			// 队列为空：等待 100ms 再检查，ctx 取消时立即退出
			select {
			case <-w.ctx.Done():
				w.pool.logger.Debug("[Worker] exiting by ctx cancel", "id", w.id)
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		w.pool.logger.Debug("[Worker] loop exited", "id", w.id)
	}()
}

func (w *worker) stop() {
	w.running.Store(false)
	w.cancel()
}

// executeTask 是 Worker 执行子任务的核心方法，包含：
//
//  1. 状态机流转：PENDING → RUNNING
//  2. 带超时 context 的 taskHandler 调用（panic 安全）
//  3. 成功路径：RUNNING → COMPLETED，触发 resultCb(success=true)
//  4. 失败可重试：指数退避后重新入队，状态机重置为 PENDING
//  5. 失败超限：RUNNING → FAILED，触发 resultCb(success=false)
func (w *worker) executeTask(task *queue.Task) {
	pool := w.pool
	log := pool.logger

	if err := pool.changeTaskState(task.TaskID, statemachine.StatusRunning, "worker picked up"); err != nil {
		log.Warn("[Worker] skip task: state transition failed",
			"worker_id", w.id, "task_id", task.TaskID, "err", err)
		pool.fireTaskEvent(task.TaskID, "WARN", fmt.Sprintf("state transition failed before execution: %v", err))
		return
	}

	log.Info("[Worker] task start",
		"worker_id", w.id, "task_id", task.TaskID,
		"priority", task.Priority, "retry", task.RetryCount)
	pool.fireTaskEvent(task.TaskID, "INFO", fmt.Sprintf("worker %d started task, retry=%d", w.id, task.RetryCount))

	// ② 带超时的执行上下文
	execCtx, cancel := context.WithTimeout(pool.ctx, pool.execTimeout)
	defer cancel()

	// ③ panic 安全包裹调用 taskHandler
	var execErr error
	var result map[string]any
	timedOut := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("panic: %v", r)
				log.Error("[Worker] panic recovered in taskHandler",
					"worker_id", w.id, "task_id", task.TaskID, "panic", r)
			}
		}()

		if pool.taskHandler != nil {
			// === 真正执行业务逻辑 ===
			// taskHandler 内部应读取 task.Data（*SubTask）的 Action/Params，
			// 完成后通过实现 ResultHolder 接口将结果写回 task.Data。
			execErr = pool.taskHandler(execCtx, task)
		} else {
			log.Warn("[Worker] no taskHandler, task is no-op", "task_id", task.TaskID)
		}

		// 提取业务结果（handler 应将结果写进 task.Data 的 Result 字段）
		result = extractResult(task)
	}()
	if execErr != nil && (errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execCtx.Err(), context.DeadlineExceeded)) {
		timedOut = true
		execErr = fmt.Errorf("task execution timeout after %s", pool.execTimeout)
	}

	// ④ 执行结果处理
	if execErr != nil {
		log.Warn("[Worker] task execution failed",
			"worker_id", w.id, "task_id", task.TaskID,
			"error", execErr, "retry_count", task.RetryCount, "max_retries", pool.maxRetries, "timed_out", timedOut)
		if timedOut {
			pool.fireTaskEvent(task.TaskID, "WARN", fmt.Sprintf("task execution timed out after %s", pool.execTimeout))
		} else {
			pool.fireTaskEvent(task.TaskID, "WARN", fmt.Sprintf("task execution failed: %v", execErr))
		}

		if task.RetryCount < pool.maxRetries {
			task.RetryCount++
			backoff := time.Duration(task.RetryCount*task.RetryCount) * time.Second
			task.ExecuteAt = time.Now().Add(backoff)
			task.Status = statemachine.StatusPending
			pool.stateMachine.SetInitialState(task.TaskID, statemachine.StatusPending)

			if pool.queue.Push(task) {
				pool.totalRetried.Add(1)
				log.Info("[Worker] task requeued",
					"task_id", task.TaskID,
					"retry_count", task.RetryCount,
					"backoff_sec", backoff.Seconds())
				if timedOut {
					pool.fireTaskEvent(task.TaskID, "INFO", fmt.Sprintf("task timeout retry scheduled: retry=%d/%d backoff=%s", task.RetryCount, pool.maxRetries, backoff))
				} else {
					pool.fireTaskEvent(task.TaskID, "INFO", fmt.Sprintf("task requeued: retry=%d/%d backoff=%s", task.RetryCount, pool.maxRetries, backoff))
				}
			} else {
				if err := pool.changeTaskState(task.TaskID, statemachine.StatusFailed, "queue full on retry"); err != nil {
					log.Error("[Worker] retry state change failed",
						"task_id", task.TaskID, "error", err)
				}
				pool.totalFailed.Add(1)
				pool.fireTaskEvent(task.TaskID, "ERROR", "retry failed because queue is full")
				pool.fireResultCb(task.TaskID, false, nil,
					fmt.Sprintf("retry failed (queue full): %v", execErr))
			}
		} else {
			failedReason := execErr.Error()
			if timedOut {
				failedReason = fmt.Sprintf("task timeout exceeded max retries (%d)", pool.maxRetries)
			}
			if err := pool.changeTaskState(task.TaskID, statemachine.StatusFailed, failedReason); err != nil {
				log.Error("[Worker] failed state change failed", "task_id", task.TaskID, "error", err)
			}
			pool.totalFailed.Add(1)
			pool.fireResultCb(task.TaskID, false, nil, failedReason)
			if timedOut {
				pool.fireTaskEvent(task.TaskID, "ERROR", failedReason)
			} else {
				pool.fireTaskEvent(task.TaskID, "ERROR", fmt.Sprintf("task failed after max retries: %v", execErr))
			}
			log.Error("[Worker] task FAILED (max retries exceeded)",
				"task_id", task.TaskID, "error", execErr, "timed_out", timedOut)
		}
	} else {
		if err := pool.changeTaskState(task.TaskID, statemachine.StatusCompleted, "success"); err != nil {
			log.Error("[Worker] completed state change failed", "task_id", task.TaskID, "error", err)
		}
		pool.totalCompleted.Add(1)
		pool.fireResultCb(task.TaskID, true, result, "")
		pool.fireTaskEvent(task.TaskID, "INFO", fmt.Sprintf("task completed by worker %d", w.id))
		log.Info("[Worker] task COMPLETED", "worker_id", w.id, "task_id", task.TaskID)
	}
}

func (p *Pool) changeTaskState(taskID string, to string, reason string) error {
	from := p.stateMachine.GetState(taskID)
	if err := p.stateMachine.ChangeState(taskID, to, reason); err != nil {
		p.logger.Error("[Pool] task state transition failed",
			"task_id", taskID, "from", from, "to", to, "reason", reason, "error", err)
		return err
	}
	p.logger.Info("[Pool] task state transitioned",
		"task_id", taskID, "from", from, "to", to, "reason", reason)
	return nil
}

// fireResultCb 安全触发结果回调：回调内的 panic 不会崩溃 Worker goroutine
func (p *Pool) fireResultCb(subTaskID string, success bool, result map[string]any, errMsg string) {
	if p.resultCb == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("[Pool] panic in resultCb",
				"sub_task_id", subTaskID, "panic", r)
		}
	}()
	p.resultCb(subTaskID, success, result, errMsg)
}

func (p *Pool) fireTaskEvent(subTaskID string, logType string, content string) {
	if p.taskEventCb == nil || subTaskID == "" || content == "" {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("[Pool] panic in taskEventCb",
				"sub_task_id", subTaskID, "panic", r)
		}
	}()
	p.taskEventCb(subTaskID, logType, content)
}

// extractResult 从 task.Data 中提取业务结果。
// 约定 taskHandler 将结果写进实现了 ResultHolder 接口的 Data 对象。
func extractResult(task *queue.Task) map[string]any {
	if task.Data == nil {
		return nil
	}
	type resultHolder interface {
		GetResult() map[string]any
	}
	if rh, ok := task.Data.(resultHolder); ok {
		return rh.GetResult()
	}
	return nil
}
