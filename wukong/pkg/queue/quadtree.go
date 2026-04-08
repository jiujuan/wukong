package queue

import (
	"context"
	"sync"
	"time"
)

// Task 任务结构
type Task struct {
	TaskID     string    // 任务ID
	Priority   int       // 优先级 1-10
	Status     string    // 状态
	RetryCount int       // 重试次数
	ExecuteAt  time.Time // 执行时间
	CreatedAt  time.Time // 创建时间
	Data       any       // 任务数据
}

// Option 函数选项模式
type Option func(*Queue)

// Queue 四叉树任务队列
type Queue struct {
	mu       sync.RWMutex
	tasks    map[string]*Task       // 任务存储
	priority [11][]*Task             // 按优先级存储 (1-10)
	size    int                     // 队列大小
	ctx     context.Context
	cancel  context.CancelFunc
}

// New 创建四叉树队列
func New(opts ...Option) *Queue {
	q := &Queue{
		tasks: make(map[string]*Task),
		size:  0,
	}
	q.ctx, q.cancel = context.WithCancel(context.Background())

	for i := range q.priority {
		q.priority[i] = make([]*Task, 0)
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(q *Queue) {
		q.ctx = ctx
		q.cancel()
	}
}

// Push 入队
func (q *Queue) Push(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 幂等检查
	if _, exists := q.tasks[task.TaskID]; exists {
		return false
	}

	// 设置默认值
	if task.Priority < 1 {
		task.Priority = 1
	}
	if task.Priority > 10 {
		task.Priority = 10
	}
	if task.ExecuteAt.IsZero() {
		task.ExecuteAt = time.Now()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	// 存储任务
	q.tasks[task.TaskID] = task
	q.size++

	// 按优先级插入
	priority := task.Priority
	q.priority[priority] = append(q.priority[priority], task)

	// 上移调整
	q.bubbleUp(priority, len(q.priority[priority])-1)

	return true
}

// Pop 出队（获取最高优先级任务）
func (q *Queue) Pop() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 从高优先级到低优先级查找
	for p := 10; p >= 1; p-- {
		if len(q.priority[p]) == 0 {
			continue
		}

		// 获取队首任务
		task := q.priority[p][0]
		if task.ExecuteAt.After(time.Now()) {
			continue // 延时任务，跳过
		}

		// 移除任务
		q.removeFromPriority(p, 0)
		delete(q.tasks, task.TaskID)
		q.size--

		return task
	}

	return nil
}

// PopWithTimeout 带超时的出队
func (q *Queue) PopWithTimeout(timeout time.Duration) (*Task, error) {
	ctx, cancel := context.WithTimeout(q.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if task := q.Pop(); task != nil {
				return task, nil
			}
		}
	}
}

// Peek 查看队首任务
func (q *Queue) Peek() *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for p := 10; p >= 1; p-- {
		if len(q.priority[p]) > 0 {
			return q.priority[p][0]
		}
	}
	return nil
}

// Get 获取任务
func (q *Queue) Get(taskID string) (*Task, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, ok := q.tasks[taskID]
	return task, ok
}

// Remove 移除任务
func (q *Queue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return false
	}

	// 从优先级队列移除
	priority := task.Priority
	for i, t := range q.priority[priority] {
		if t.TaskID == taskID {
			q.removeFromPriority(priority, i)
			break
		}
	}

	delete(q.tasks, taskID)
	q.size--

	return true
}

// Update 更新任务
func (q *Queue) Update(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	oldTask, exists := q.tasks[task.TaskID]
	if !exists {
		return false
	}

	oldPriority := oldTask.Priority
	newPriority := task.Priority
	if newPriority < 1 {
		newPriority = 1
	}
	if newPriority > 10 {
		newPriority = 10
	}

	// 如果优先级改变，需要移动
	if oldPriority != newPriority {
		// 从旧优先级队列移除
		for i, t := range q.priority[oldPriority] {
			if t.TaskID == task.TaskID {
				q.removeFromPriority(oldPriority, i)
				break
			}
		}

		// 添加到新优先级队列
		task.Priority = newPriority
		q.priority[newPriority] = append(q.priority[newPriority], task)
		q.bubbleUp(newPriority, len(q.priority[newPriority])-1)
	}

	q.tasks[task.TaskID] = task

	return true
}

// Size 获取队列大小
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size
}

// IsEmpty 检查队列是否为空
func (q *Queue) IsEmpty() bool {
	return q.Size() == 0
}

// List 获取所有任务
func (q *Queue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, 0, q.size)
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// ListByPriority 获取指定优先级的任务
func (q *Queue) ListByPriority(priority int) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if priority < 1 || priority > 10 {
		return nil
	}

	tasks := make([]*Task, len(q.priority[priority]))
	copy(tasks, q.priority[priority])
	return tasks
}

// Clear 清空队列
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks = make(map[string]*Task)
	for i := range q.priority {
		q.priority[i] = make([]*Task, 0)
	}
	q.size = 0
}

// Close 关闭队列
func (q *Queue) Close() {
	q.cancel()
}

// bubbleUp 上浮调整
func (q *Queue) bubbleUp(priority, index int) {
	heap := q.priority[priority]
	for index > 0 {
		parent := (index - 1) / 2
		if heap[index].ExecuteAt.After(heap[parent].ExecuteAt) {
			break
		}
		heap[index], heap[parent] = heap[parent], heap[index]
		index = parent
	}
}

// removeFromPriority 从优先级队列移除
func (q *Queue) removeFromPriority(priority, index int) {
	heap := q.priority[priority]
	if index < 0 || index >= len(heap) {
		return
	}

	// 将最后一个元素移到当前位置
	last := len(heap) - 1
	heap[index], heap[last] = heap[last], heap[index]
	q.priority[priority] = heap[:last]

	// 向下调整
	q.bubbleDown(priority, index)
}

// bubbleDown 下沉调整
func (q *Queue) bubbleDown(priority, index int) {
	heap := q.priority[priority]
	length := len(heap)

	for {
		left := 2*index + 1
		right := 2*index + 2
		smallest := index

		if left < length && heap[left].ExecuteAt.Before(heap[smallest].ExecuteAt) {
			smallest = left
		}
		if right < length && heap[right].ExecuteAt.Before(heap[smallest].ExecuteAt) {
			smallest = right
		}

		if smallest == index {
			break
		}

		heap[index], heap[smallest] = heap[smallest], heap[index]
		index = smallest
	}
}
