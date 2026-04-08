package queue

import (
	"sync"
	"time"
)

// DelayTask 延时任务结构
type DelayTask struct {
	TaskID    string
	Delay     time.Duration
	ExecuteAt time.Time
	Task      *Task
}

// DelayQueue 延时任务队列
type DelayQueue struct {
	mu    sync.RWMutex
	tasks []*DelayTask
}

// NewDelayQueue 创建延时队列
func NewDelayQueue() *DelayQueue {
	return &DelayQueue{
		tasks: make([]*DelayTask, 0),
	}
}

// Push 添加延时任务
func (dq *DelayQueue) Push(taskID string, delay time.Duration, task *Task) {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	delayTask := &DelayTask{
		TaskID:    taskID,
		Delay:     delay,
		ExecuteAt: time.Now().Add(delay),
		Task:      task,
	}

	dq.tasks = append(dq.tasks, delayTask)
	dq.bubbleUp(len(dq.tasks) - 1)
}

// PopReady 获取到期的延时任务
func (dq *DelayQueue) PopReady() []*Task {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	now := time.Now()
	var readyTasks []*Task
	var remaining []*DelayTask

	for _, dt := range dq.tasks {
		if dt.ExecuteAt.Before(now) || dt.ExecuteAt.Equal(now) {
			readyTasks = append(readyTasks, dt.Task)
		} else {
			remaining = append(remaining, dt)
		}
	}

	dq.tasks = remaining
	return readyTasks
}

// Cancel 取消延时任务
func (dq *DelayQueue) Cancel(taskID string) bool {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	for i, dt := range dq.tasks {
		if dt.TaskID == taskID {
			dq.tasks = append(dq.tasks[:i], dq.tasks[i+1:]...)
			return true
		}
	}
	return false
}

// Size 获取延时任务数量
func (dq *DelayQueue) Size() int {
	dq.mu.RLock()
	defer dq.mu.RUnlock()
	return len(dq.tasks)
}

// bubbleUp 上浮
func (dq *DelayQueue) bubbleUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if dq.tasks[index].ExecuteAt.After(dq.tasks[parent].ExecuteAt) {
			break
		}
		dq.tasks[index], dq.tasks[parent] = dq.tasks[parent], dq.tasks[index]
		index = parent
	}
}
