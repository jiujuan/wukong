package queue

import (
	"context"
	"sync"
	"time"
)

const (
	minPriority   = 1
	maxPriority   = 10
	quadHeapArity = 4
)

// Task represents a scheduled task in the queue.
type Task struct {
	TaskID     string
	Priority   int
	Status     string
	RetryCount int
	ExecuteAt  time.Time
	CreatedAt  time.Time
	Data       any
}

// Option configures the queue instance.
type Option func(*Queue)

type queueItem struct {
	task  *Task
	order int64
}

// quadHeap is a min-heap ordered by ExecuteAt and then insertion order.
type quadHeap struct {
	items   []*queueItem
	indices map[string]int
}

func newQuadHeap() *quadHeap {
	return &quadHeap{
		items:   make([]*queueItem, 0),
		indices: make(map[string]int),
	}
}

func (h *quadHeap) len() int {
	return len(h.items)
}

func (h *quadHeap) peek() *Task {
	if len(h.items) == 0 {
		return nil
	}
	return h.items[0].task
}

func (h *quadHeap) push(task *Task, order int64) {
	item := &queueItem{task: task, order: order}
	h.items = append(h.items, item)
	index := len(h.items) - 1
	h.indices[task.TaskID] = index
	h.bubbleUp(index)
}

func (h *quadHeap) pop() *Task {
	if len(h.items) == 0 {
		return nil
	}
	return h.removeAt(0)
}

func (h *quadHeap) remove(taskID string) bool {
	index, ok := h.indices[taskID]
	if !ok {
		return false
	}
	h.removeAt(index)
	return true
}

func (h *quadHeap) update(task *Task) bool {
	index, ok := h.indices[task.TaskID]
	if !ok {
		return false
	}
	h.items[index].task = task
	h.fix(index)
	return true
}

func (h *quadHeap) list() []*Task {
	out := make([]*Task, len(h.items))
	for i, item := range h.items {
		out[i] = item.task
	}
	return out
}

func (h *quadHeap) removeAt(index int) *Task {
	last := len(h.items) - 1
	removed := h.items[index]

	if index != last {
		h.swap(index, last)
	}
	delete(h.indices, removed.task.TaskID)
	h.items[last] = nil
	h.items = h.items[:last]

	if index < len(h.items) {
		h.fix(index)
	}

	return removed.task
}

func (h *quadHeap) fix(index int) {
	if index > 0 {
		parent := (index - 1) / quadHeapArity
		if h.less(index, parent) {
			h.bubbleUp(index)
			return
		}
	}
	h.bubbleDown(index)
}

func (h *quadHeap) bubbleUp(index int) {
	for index > 0 {
		parent := (index - 1) / quadHeapArity
		if !h.less(index, parent) {
			return
		}
		h.swap(index, parent)
		index = parent
	}
}

func (h *quadHeap) bubbleDown(index int) {
	for {
		best := index
		firstChild := index*quadHeapArity + 1
		for child := firstChild; child < firstChild+quadHeapArity && child < len(h.items); child++ {
			if h.less(child, best) {
				best = child
			}
		}
		if best == index {
			return
		}
		h.swap(index, best)
		index = best
	}
}

func (h *quadHeap) less(i, j int) bool {
	left := h.items[i]
	right := h.items[j]

	if left.task.ExecuteAt.Before(right.task.ExecuteAt) {
		return true
	}
	if left.task.ExecuteAt.After(right.task.ExecuteAt) {
		return false
	}
	return left.order < right.order
}

func (h *quadHeap) swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.indices[h.items[i].task.TaskID] = i
	h.indices[h.items[j].task.TaskID] = j
}

// Queue is a priority queue backed by per-priority quaternary heaps.
type Queue struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	heaps   [maxPriority + 1]*quadHeap
	size    int
	nextOrd int64
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates a new queue.
func New(opts ...Option) *Queue {
	q := &Queue{
		tasks: make(map[string]*Task),
		size:  0,
	}
	q.ctx, q.cancel = context.WithCancel(context.Background())

	for priority := range q.heaps {
		q.heaps[priority] = newQuadHeap()
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// WithContext sets the queue context.
func WithContext(ctx context.Context) Option {
	return func(q *Queue) {
		q.ctx = ctx
		q.cancel()
	}
}

// Push inserts a new task. Duplicate task IDs are rejected.
func (q *Queue) Push(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.tasks[task.TaskID]; exists {
		return false
	}

	q.normalizeTask(task)
	q.tasks[task.TaskID] = task
	q.size++
	q.nextOrd++
	q.heaps[task.Priority].push(task, q.nextOrd)
	return true
}

// Pop returns the highest-priority ready task.
func (q *Queue) Pop() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for priority := maxPriority; priority >= minPriority; priority-- {
		task := q.heaps[priority].peek()
		if task == nil {
			continue
		}
		if task.ExecuteAt.After(now) {
			continue
		}
		task = q.heaps[priority].pop()
		delete(q.tasks, task.TaskID)
		q.size--
		return task
	}

	return nil
}

// PopWithTimeout waits for a task until the timeout expires.
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

// Peek returns the current top task for the highest available priority.
func (q *Queue) Peek() *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for priority := maxPriority; priority >= minPriority; priority-- {
		if task := q.heaps[priority].peek(); task != nil {
			return task
		}
	}
	return nil
}

// Get retrieves a task by ID.
func (q *Queue) Get(taskID string) (*Task, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, ok := q.tasks[taskID]
	return task, ok
}

// Remove deletes a task from the queue.
func (q *Queue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return false
	}

	if !q.heaps[task.Priority].remove(taskID) {
		return false
	}
	delete(q.tasks, taskID)
	q.size--
	return true
}

// Update replaces an existing task and repositions it in the heap when needed.
func (q *Queue) Update(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	oldTask, exists := q.tasks[task.TaskID]
	if !exists {
		return false
	}

	q.normalizeTask(task)
	oldPriority := oldTask.Priority
	newPriority := task.Priority

	if oldPriority != newPriority {
		if !q.heaps[oldPriority].remove(task.TaskID) {
			return false
		}
		q.nextOrd++
		q.heaps[newPriority].push(task, q.nextOrd)
	} else if !q.heaps[oldPriority].update(task) {
		return false
	}

	q.tasks[task.TaskID] = task
	return true
}

// Size returns the number of queued tasks.
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size
}

// IsEmpty reports whether the queue has no tasks.
func (q *Queue) IsEmpty() bool {
	return q.Size() == 0
}

// List returns all queued tasks.
func (q *Queue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, 0, q.size)
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// ListByPriority returns the heap contents for a priority level.
func (q *Queue) ListByPriority(priority int) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if priority < minPriority || priority > maxPriority {
		return nil
	}
	return q.heaps[priority].list()
}

// Clear removes all tasks from the queue.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks = make(map[string]*Task)
	for priority := range q.heaps {
		q.heaps[priority] = newQuadHeap()
	}
	q.size = 0
	q.nextOrd = 0
}

// Close releases the queue context.
func (q *Queue) Close() {
	q.cancel()
}

func (q *Queue) normalizeTask(task *Task) {
	if task.Priority < minPriority {
		task.Priority = minPriority
	}
	if task.Priority > maxPriority {
		task.Priority = maxPriority
	}
	if task.ExecuteAt.IsZero() {
		task.ExecuteAt = time.Now()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
}
