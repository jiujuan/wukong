package queue

import (
	"fmt"
	"math/rand"
	"sort"
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
		t.Fatal("new queue should be empty")
	}
	if q.Size() != 0 {
		t.Fatalf("Size() = %d, want 0", q.Size())
	}
}

func TestPushIdempotentAndNormalization(t *testing.T) {
	q := New()

	task := &Task{
		TaskID:   "task-1",
		Priority: 0,
	}
	if !q.Push(task) {
		t.Fatal("Push() should accept a new task")
	}
	if q.Push(task) {
		t.Fatal("Push() should reject duplicate task IDs")
	}

	got, ok := q.Get("task-1")
	if !ok {
		t.Fatal("Get() should find pushed task")
	}
	if got.Priority != minPriority {
		t.Fatalf("Priority = %d, want %d", got.Priority, minPriority)
	}
	if got.ExecuteAt.IsZero() {
		t.Fatal("ExecuteAt should be initialized")
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be initialized")
	}
}

func TestPopHonorsPriorityThenExecuteAt(t *testing.T) {
	q := New()
	base := time.Now().Add(-time.Minute)

	mustPush(t, q, &Task{TaskID: "low-earlier", Priority: 1, ExecuteAt: base})
	mustPush(t, q, &Task{TaskID: "high-later", Priority: 10, ExecuteAt: base.Add(2 * time.Second)})
	mustPush(t, q, &Task{TaskID: "high-earlier", Priority: 10, ExecuteAt: base.Add(1 * time.Second)})

	if got := q.Pop(); got == nil || got.TaskID != "high-earlier" {
		t.Fatalf("first Pop() = %#v, want high-earlier", got)
	}
	if got := q.Pop(); got == nil || got.TaskID != "high-later" {
		t.Fatalf("second Pop() = %#v, want high-later", got)
	}
	if got := q.Pop(); got == nil || got.TaskID != "low-earlier" {
		t.Fatalf("third Pop() = %#v, want low-earlier", got)
	}
}

func TestPopSkipsDelayedTaskButKeepsLowerPriorityReadyBehavior(t *testing.T) {
	q := New()
	now := time.Now()

	mustPush(t, q, &Task{
		TaskID:    "high-delayed",
		Priority:  10,
		ExecuteAt: now.Add(time.Hour),
	})
	mustPush(t, q, &Task{
		TaskID:    "low-ready",
		Priority:  1,
		ExecuteAt: now.Add(-time.Second),
	})

	if got := q.Pop(); got == nil || got.TaskID != "low-ready" {
		t.Fatalf("Pop() = %#v, want low-ready", got)
	}
	if got := q.Pop(); got != nil {
		t.Fatalf("Pop() should skip delayed task, got %#v", got)
	}
}

func TestPeekReturnsHighestPriorityTopWithoutRemoving(t *testing.T) {
	q := New()
	now := time.Now()

	if got := q.Peek(); got != nil {
		t.Fatalf("Peek() on empty queue = %#v, want nil", got)
	}

	mustPush(t, q, &Task{TaskID: "p5", Priority: 5, ExecuteAt: now})
	mustPush(t, q, &Task{TaskID: "p10-delayed", Priority: 10, ExecuteAt: now.Add(time.Hour)})

	got := q.Peek()
	if got == nil || got.TaskID != "p10-delayed" {
		t.Fatalf("Peek() = %#v, want p10-delayed", got)
	}
	if q.Size() != 2 {
		t.Fatalf("Peek() should not remove items, size = %d", q.Size())
	}
}

func TestUpdateReordersWithinPriority(t *testing.T) {
	q := New()
	now := time.Now()

	mustPush(t, q, &Task{TaskID: "later", Priority: 5, ExecuteAt: now.Add(time.Second)})
	mustPush(t, q, &Task{TaskID: "earlier", Priority: 5, ExecuteAt: now.Add(2 * time.Second)})

	updated := &Task{TaskID: "earlier", Priority: 5, ExecuteAt: now.Add(-time.Second)}
	if !q.Update(updated) {
		t.Fatal("Update() should succeed")
	}

	if got := q.Pop(); got == nil || got.TaskID != "earlier" {
		t.Fatalf("Pop() after update = %#v, want earlier", got)
	}
	validateAllHeaps(t, q)
}

func TestUpdateMovesTaskAcrossPriorities(t *testing.T) {
	q := New()
	now := time.Now()

	mustPush(t, q, &Task{TaskID: "task-a", Priority: 3, ExecuteAt: now})
	mustPush(t, q, &Task{TaskID: "task-b", Priority: 8, ExecuteAt: now})

	if !q.Update(&Task{TaskID: "task-a", Priority: 9, ExecuteAt: now}) {
		t.Fatal("Update() should succeed")
	}

	got := q.Pop()
	if got == nil || got.TaskID != "task-a" {
		t.Fatalf("Pop() = %#v, want task-a", got)
	}
	validateAllHeaps(t, q)
}

func TestRemoveAndClear(t *testing.T) {
	q := New()
	now := time.Now()

	mustPush(t, q, &Task{TaskID: "task-1", Priority: 4, ExecuteAt: now})
	mustPush(t, q, &Task{TaskID: "task-2", Priority: 7, ExecuteAt: now})

	if !q.Remove("task-1") {
		t.Fatal("Remove() should succeed for existing task")
	}
	if q.Remove("missing") {
		t.Fatal("Remove() should fail for missing task")
	}
	if q.Size() != 1 {
		t.Fatalf("Size() = %d, want 1", q.Size())
	}

	q.Clear()
	if !q.IsEmpty() {
		t.Fatal("queue should be empty after Clear()")
	}
	validateAllHeaps(t, q)
}

func TestListByPriorityReturnsCopy(t *testing.T) {
	q := New()
	now := time.Now()

	mustPush(t, q, &Task{TaskID: "task-1", Priority: 5, ExecuteAt: now})
	mustPush(t, q, &Task{TaskID: "task-2", Priority: 5, ExecuteAt: now.Add(time.Second)})

	items := q.ListByPriority(5)
	if len(items) != 2 {
		t.Fatalf("ListByPriority(5) len = %d, want 2", len(items))
	}
	items[0] = nil

	again := q.ListByPriority(5)
	if len(again) != 2 || again[0] == nil {
		t.Fatal("ListByPriority() should return a copy")
	}
	if got := q.ListByPriority(0); got != nil {
		t.Fatalf("ListByPriority(0) = %#v, want nil", got)
	}
}

func TestPopWithTimeout(t *testing.T) {
	q := New()

	if _, err := q.PopWithTimeout(100 * time.Millisecond); err == nil {
		t.Fatal("PopWithTimeout() should return timeout error for empty queue")
	}
}

func TestConcurrentPushWithUniqueTaskIDs(t *testing.T) {
	q := New()
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mustPushConcurrent(t, q, &Task{
				TaskID:    fmt.Sprintf("task-%03d", id),
				Priority:  (id % maxPriority) + 1,
				ExecuteAt: time.Now().Add(-time.Duration(id) * time.Millisecond),
			})
		}(i)
	}
	wg.Wait()

	if q.Size() != 200 {
		t.Fatalf("Size() = %d, want 200", q.Size())
	}
	validateAllHeaps(t, q)
}

func TestQueueMaintainsQuadHeapInvariant(t *testing.T) {
	q := New()
	now := time.Now()

	for i := 0; i < 128; i++ {
		mustPush(t, q, &Task{
			TaskID:    fmt.Sprintf("task-%03d", i),
			Priority:  (i % maxPriority) + 1,
			ExecuteAt: now.Add(time.Duration((i*37)%23) * time.Millisecond),
		})
	}

	for i := 0; i < 32; i++ {
		if !q.Update(&Task{
			TaskID:    fmt.Sprintf("task-%03d", i),
			Priority:  ((i + 3) % maxPriority) + 1,
			ExecuteAt: now.Add(-time.Duration(i) * time.Millisecond),
		}) {
			t.Fatalf("Update() failed for task-%03d", i)
		}
	}

	for i := 96; i < 112; i++ {
		if !q.Remove(fmt.Sprintf("task-%03d", i)) {
			t.Fatalf("Remove() failed for task-%03d", i)
		}
	}

	validateAllHeaps(t, q)
}

func TestRandomizedPopMatchesReferenceOrdering(t *testing.T) {
	q := New()
	base := time.Now().Add(-time.Hour)
	rng := rand.New(rand.NewSource(42))

	type expectedItem struct {
		taskID    string
		priority  int
		executeAt time.Time
		order     int
	}

	expected := make([]expectedItem, 0, 300)
	for i := 0; i < 300; i++ {
		task := &Task{
			TaskID:    fmt.Sprintf("task-%03d", i),
			Priority:  rng.Intn(maxPriority) + 1,
			ExecuteAt: base.Add(time.Duration(rng.Intn(5000)) * time.Millisecond),
		}
		mustPush(t, q, task)
		expected = append(expected, expectedItem{
			taskID:    task.TaskID,
			priority:  task.Priority,
			executeAt: task.ExecuteAt,
			order:     i,
		})
	}

	sort.Slice(expected, func(i, j int) bool {
		if expected[i].priority != expected[j].priority {
			return expected[i].priority > expected[j].priority
		}
		if !expected[i].executeAt.Equal(expected[j].executeAt) {
			return expected[i].executeAt.Before(expected[j].executeAt)
		}
		return expected[i].order < expected[j].order
	})

	for _, want := range expected {
		got := q.Pop()
		if got == nil {
			t.Fatalf("Pop() returned nil before queue drained, want %s", want.taskID)
		}
		if got.TaskID != want.taskID {
			t.Fatalf("Pop() = %s, want %s", got.TaskID, want.taskID)
		}
	}

	if got := q.Pop(); got != nil {
		t.Fatalf("Pop() after drain = %#v, want nil", got)
	}
}

func mustPush(t *testing.T, q *Queue, task *Task) {
	t.Helper()
	if !q.Push(task) {
		t.Fatalf("Push() failed for task %s", task.TaskID)
	}
}

func mustPushConcurrent(t *testing.T, q *Queue, task *Task) {
	t.Helper()
	if !q.Push(task) {
		t.Errorf("Push() failed for task %s", task.TaskID)
	}
}

func validateAllHeaps(t *testing.T, q *Queue) {
	t.Helper()

	q.mu.RLock()
	defer q.mu.RUnlock()

	total := 0
	for priority := minPriority; priority <= maxPriority; priority++ {
		heap := q.heaps[priority]
		total += heap.len()
		validateHeap(t, priority, heap)
	}

	if total != q.size {
		t.Fatalf("heap item total = %d, queue size = %d", total, q.size)
	}
	if len(q.tasks) != q.size {
		t.Fatalf("task map len = %d, queue size = %d", len(q.tasks), q.size)
	}
}

func validateHeap(t *testing.T, priority int, heap *quadHeap) {
	t.Helper()

	if heap == nil {
		t.Fatalf("priority %d heap is nil", priority)
	}
	if len(heap.items) != len(heap.indices) {
		t.Fatalf("priority %d heap len = %d, index len = %d", priority, len(heap.items), len(heap.indices))
	}

	for i, item := range heap.items {
		if item == nil || item.task == nil {
			t.Fatalf("priority %d heap has nil item at index %d", priority, i)
		}
		if got := heap.indices[item.task.TaskID]; got != i {
			t.Fatalf("priority %d task %s index = %d, want %d", priority, item.task.TaskID, got, i)
		}

		firstChild := i*quadHeapArity + 1
		for child := firstChild; child < firstChild+quadHeapArity && child < len(heap.items); child++ {
			if heap.less(child, i) {
				t.Fatalf("priority %d violates heap invariant: child %d is less than parent %d", priority, child, i)
			}
		}
	}
}
