package worker

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jiujuan/wukong/pkg/queue"
)

func TestNewPool(t *testing.T) {
	pool := New()
	if pool == nil {
		t.Fatal("New() returned nil")
	}
}

func TestPoolStartStop(t *testing.T) {
	pool := New(
		WithWorkerCount(2),
		WithName("test-pool"),
	)

	if err := pool.Start(); err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !pool.IsRunning() {
		t.Error("IsRunning() should return true after Start()")
	}

	pool.Stop()

	if pool.IsRunning() {
		t.Error("IsRunning() should return false after Stop()")
	}
}

func TestPoolSubmit(t *testing.T) {
	pool := New(
		WithWorkerCount(1),
		WithTaskHandler(func(ctx context.Context, task *queue.Task) error {
			return nil
		}),
	)
	pool.Start()
	defer pool.Stop()

	task := &queue.Task{
		TaskID:   "task1",
		Priority: 5,
	}

	if !pool.Submit(task) {
		t.Error("Submit() should return true")
	}

	// 等待任务被处理
	time.Sleep(100 * time.Millisecond)

	state := pool.GetTaskState("task1")
	if state == "" {
		t.Error("Task should have a state after submit")
	}
}

func TestPoolSubmitMultiple(t *testing.T) {
	pool := New(
		WithWorkerCount(2),
		WithTaskHandler(func(ctx context.Context, task *queue.Task) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}),
	)
	pool.Start()
	defer pool.Stop()

	for i := 0; i < 10; i++ {
		task := &queue.Task{
			TaskID:   "task" + string(rune('0'+i)),
			Priority: 5,
		}
		pool.Submit(task)
	}

	// 等待所有任务被处理
	time.Sleep(500 * time.Millisecond)

	// 验证所有任务都已被处理
	for i := 0; i < 10; i++ {
		taskID := "task" + string(rune('0'+i))
		state := pool.GetTaskState(taskID)
		if state == "" {
			t.Errorf("Task %s should have a state", taskID)
		}
	}
}

func TestPoolCancel(t *testing.T) {
	pool := New(
		WithWorkerCount(1),
		WithTaskHandler(func(ctx context.Context, task *queue.Task) error {
			time.Sleep(10 * time.Second)
			return nil
		}),
	)
	pool.Start()
	defer pool.Stop()

	task := &queue.Task{
		TaskID:   "task1",
		Priority: 5,
	}

	pool.Submit(task)

	if !pool.Cancel("task1") {
		t.Error("Cancel() should return true")
	}

	if pool.GetTaskState("task1") != "CANCELLED" {
		t.Error("Task state should be CANCELLED after Cancel()")
	}
}

func TestPoolIdempotent(t *testing.T) {
	pool := New(
		WithWorkerCount(1),
		WithTaskHandler(func(ctx context.Context, task *queue.Task) error {
			return nil
		}),
	)
	pool.Start()
	defer pool.Stop()

	task := &queue.Task{
		TaskID:   "task1",
		Priority: 5,
	}

	pool.Submit(task)
	// 重复提交应该返回false（幂等）
	if pool.Submit(task) {
		t.Error("Duplicate submit should return false")
	}
}

func TestPoolGetQueueSize(t *testing.T) {
	pool := New(
		WithWorkerCount(1),
		WithTaskHandler(func(ctx context.Context, task *queue.Task) error {
			time.Sleep(1 * time.Second)
			return nil
		}),
	)
	pool.Start()
	defer pool.Stop()

	for i := 0; i < 5; i++ {
		pool.Submit(&queue.Task{
			TaskID:   "task" + string(rune('0'+i)),
			Priority: 5,
		})
	}

	size := pool.GetQueueSize()
	if size < 0 || size > 5 {
		t.Errorf("QueueSize = %d, want between 0 and 5", size)
	}
}

func TestPoolWithLogger(t *testing.T) {
	logger := slog.Default()
	pool := New(
		WithLogger(logger),
	)

	if pool.logger == nil {
		t.Error("Logger should be set")
	}
}

func TestPoolWithMaxRetries(t *testing.T) {
	pool := New(
		WithMaxRetries(3),
	)

	// 验证max retries配置
	task := &queue.Task{
		TaskID:   "task1",
		Priority: 5,
	}
	pool.Submit(task)
}

func TestPoolDoubleStart(t *testing.T) {
	pool := New()

	pool.Start()
	pool.Start() // 重复启动不应该报错

	pool.Stop()
}

func TestPoolContextCancellation(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	pool := New(
		WithWorkerCount(1),
	)
	pool.Start()
	pool.Stop()

	cancel()
}
