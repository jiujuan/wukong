package queue

import (
	"sync"
	"testing"
	"time"
)

func TestNewQueue(t *testing.T) {
	q := New()
	if q == nil {
		t.Fatal("New() returned nil")
	}
	if !q.IsEmpty() {
		t.Error("New queue should be empty")
	}
}

func TestPush(t *testing.T) {
	q := New()

	task := &Task{
		TaskID:   "task1",
		Priority: 5,
	}

	if !q.Push(task) {
		t.Error("Push() should return true for new task")
	}
	if q.Size() != 1 {
		t.Errorf("Size() = %d, want %d", q.Size(), 1)
	}
}

func TestPushIdempotent(t *testing.T) {
	q := New()

	task := &Task{
		TaskID:   "task1",
		Priority: 5,
	}

	// First push should return true
	if !q.Push(task) {
		t.Error("Push() should return true for new task")
	}

	// Second push (duplicate) should return false
	if q.Push(task) {
		t.Error("Push() should return false for duplicate task")
	}

	if q.Size() != 1 {
		t.Errorf("Size() should remain 1 for duplicate, got %d", q.Size())
	}
}

func TestPop(t *testing.T) {
	q := New()

	// 按优先级插入
	q.Push(&Task{TaskID: "low", Priority: 1})
	q.Push(&Task{TaskID: "high", Priority: 10})
	q.Push(&Task{TaskID: "medium", Priority: 5})

	// 第一个pop应该是最高优先级
	task := q.Pop()
	if task == nil {
		t.Fatal("Pop() returned nil")
	}
	if task.TaskID != "high" {
		t.Errorf("Pop() = %v, want high", task.TaskID)
	}
}

func TestPopEmpty(t *testing.T) {
	q := New()

	task := q.Pop()
	if task != nil {
		t.Error("Pop() should return nil for empty queue")
	}
}

func TestPeek(t *testing.T) {
	q := New()

	task := q.Peek()
	if task != nil {
		t.Error("Peek() should return nil for empty queue")
	}

	q.Push(&Task{TaskID: "task1", Priority: 5})

	peeked := q.Peek()
	if peeked == nil {
		t.Error("Peek() should return task")
	}
	if peeked.TaskID != "task1" {
		t.Errorf("Peek() = %v, want task1", peeked.TaskID)
	}

	// peek should not remove
	if q.Size() != 1 {
		t.Error("Peek() should not remove task")
	}
}

func TestGet(t *testing.T) {
	q := New()

	q.Push(&Task{TaskID: "task1", Priority: 5})

	task, ok := q.Get("task1")
	if !ok {
		t.Error("Get() should return true for existing task")
	}
	if task.TaskID != "task1" {
		t.Errorf("Get() = %v, want task1", task.TaskID)
	}

	_, ok = q.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existing task")
	}
}

func TestRemove(t *testing.T) {
	q := New()

	q.Push(&Task{TaskID: "task1", Priority: 5})
	q.Push(&Task{TaskID: "task2", Priority: 3})

	if !q.Remove("task1") {
		t.Error("Remove() should return true for existing task")
	}
	if q.Size() != 1 {
		t.Errorf("Size() = %d, want %d", q.Size(), 1)
	}

	if q.Remove("nonexistent") {
		t.Error("Remove() should return false for non-existing task")
	}
}

func TestUpdate(t *testing.T) {
	q := New()

	q.Push(&Task{TaskID: "task1", Priority: 5})

	q.Update(&Task{TaskID: "task1", Priority: 8})

	task, _ := q.Get("task1")
	if task.Priority != 8 {
		t.Errorf("Priority = %d, want %d", task.Priority, 8)
	}
}

func TestPriorityBoundary(t *testing.T) {
	q := New()

	// 测试优先级边界
	q.Push(&Task{TaskID: "task0", Priority: 0})
	q.Push(&Task{TaskID: "task11", Priority: 11})

	task0, _ := q.Get("task0")
	if task0.Priority != 1 { // 应该被限制为1
		t.Errorf("Priority should be 1, got %d", task0.Priority)
	}

	task11, _ := q.Get("task11")
	if task11.Priority != 10 { // 应该被限制为10
		t.Errorf("Priority should be 10, got %d", task11.Priority)
	}
}

func TestSize(t *testing.T) {
	q := New()

	if q.Size() != 0 {
		t.Errorf("Empty size = %d, want %d", q.Size(), 0)
	}

	for i := 0; i < 10; i++ {
		q.Push(&Task{TaskID: "task" + string(rune('0'+i)), Priority: 5})
	}

	if q.Size() != 10 {
		t.Errorf("Size() = %d, want %d", q.Size(), 10)
	}
}

func TestClear(t *testing.T) {
	q := New()

	for i := 0; i < 10; i++ {
		q.Push(&Task{TaskID: "task" + string(rune('0'+i)), Priority: 5})
	}

	q.Clear()

	if !q.IsEmpty() {
		t.Error("After Clear(), queue should be empty")
	}
}

func TestList(t *testing.T) {
	q := New()

	q.Push(&Task{TaskID: "task1", Priority: 5})
	q.Push(&Task{TaskID: "task2", Priority: 3})

	tasks := q.List()
	if len(tasks) != 2 {
		t.Errorf("List() length = %d, want %d", len(tasks), 2)
	}
}

func TestListByPriority(t *testing.T) {
	q := New()

	q.Push(&Task{TaskID: "task1", Priority: 5})
	q.Push(&Task{TaskID: "task2", Priority: 5})
	q.Push(&Task{TaskID: "task3", Priority: 3})

	tasks := q.ListByPriority(5)
	if len(tasks) != 2 {
		t.Errorf("ListByPriority(5) length = %d, want %d", len(tasks), 2)
	}

	tasks = q.ListByPriority(3)
	if len(tasks) != 1 {
		t.Errorf("ListByPriority(3) length = %d, want %d", len(tasks), 1)
	}

	tasks = q.ListByPriority(0) // 无效优先级
	if tasks != nil {
		t.Error("ListByPriority(0) should return nil")
	}
}

func TestConcurrentPush(t *testing.T) {
	q := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q.Push(&Task{TaskID: "task" + string(rune('0'+id%10)), Priority: 5})
		}(i)
	}

	wg.Wait()

	// 由于幂等性，最终只有10个任务
	if q.Size() != 10 {
		t.Errorf("Final size = %d, want %d", q.Size(), 10)
	}
}

func TestPopWithTimeout(t *testing.T) {
	q := New()

	// 队列为空，应该在超时后返回
	_, err := q.PopWithTimeout(100 * time.Millisecond)
	if err == nil {
		t.Error("PopWithTimeout should return error on timeout")
	}
}

func TestExecuteAt(t *testing.T) {
	q := New()

	// 创建一个延时任务
	futureTime := time.Now().Add(1 * time.Hour)
	q.Push(&Task{
		TaskID:    "delayed",
		Priority:  5,
		ExecuteAt: futureTime,
	})

	// 立即pop应该跳过延时任务
	task := q.Pop()
	if task != nil {
		t.Error("Pop() should skip delayed task")
	}

	// 修改为立即执行
	task, _ = q.Get("delayed")
	task.ExecuteAt = time.Now()
	q.Update(task)

	// 现在应该能pop出来
	task = q.Pop()
	if task == nil {
		t.Error("Pop() should return delayed task after ExecuteAt updated")
	}
}
