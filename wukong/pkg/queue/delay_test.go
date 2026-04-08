package queue

import (
	"testing"
	"time"
)

func TestNewDelayQueue(t *testing.T) {
	dq := NewDelayQueue()
	if dq == nil {
		t.Fatal("NewDelayQueue() returned nil")
	}
	if dq.Size() != 0 {
		t.Error("New delay queue should be empty")
	}
}

func TestDelayQueuePush(t *testing.T) {
	dq := NewDelayQueue()

	task := &Task{TaskID: "task1", Priority: 5}
	dq.Push("task1", 1*time.Second, task)

	if dq.Size() != 1 {
		t.Errorf("Size() = %d, want %d", dq.Size(), 1)
	}
}

func TestDelayQueueCancel(t *testing.T) {
	dq := NewDelayQueue()

	task := &Task{TaskID: "task1", Priority: 5}
	dq.Push("task1", 1*time.Second, task)

	if !dq.Cancel("task1") {
		t.Error("Cancel() should return true for existing task")
	}

	if dq.Size() != 0 {
		t.Errorf("After cancel, Size() = %d, want %d", dq.Size(), 0)
	}

	if dq.Cancel("nonexistent") {
		t.Error("Cancel() should return false for non-existing task")
	}
}

func TestDelayQueuePopReady(t *testing.T) {
	dq := NewDelayQueue()

	// 添加一个已过期的任务
	task := &Task{TaskID: "task1", Priority: 5}
	dq.Push("task1", -1*time.Second, task) // -1秒表示已过期

	tasks := dq.PopReady()
	if len(tasks) != 1 {
		t.Errorf("PopReady() returned %d tasks, want %d", len(tasks), 1)
	}

	// 再次调用应该为空
	tasks = dq.PopReady()
	if len(tasks) != 0 {
		t.Errorf("Second PopReady() returned %d tasks, want %d", len(tasks), 0)
	}
}

func TestDelayQueueMixed(t *testing.T) {
	dq := NewDelayQueue()

	// 添加一个已过期和一个未过期的任务
	task1 := &Task{TaskID: "task1", Priority: 5}
	task2 := &Task{TaskID: "task2", Priority: 5}

	dq.Push("task1", -1*time.Second, task1)
	dq.Push("task2", 1*time.Hour, task2)

	// 只能pop出已过期的
	tasks := dq.PopReady()
	if len(tasks) != 1 {
		t.Errorf("PopReady() returned %d tasks, want %d", len(tasks), 1)
	}
	if tasks[0].TaskID != "task1" {
		t.Errorf("PopReady() = %v, want task1", tasks[0].TaskID)
	}
}
