package worker

import (
	"sync"
	"testing"
	"time"
)

func TestNewWorkerRegistry(t *testing.T) {
	registry := NewWorkerRegistry()
	if registry == nil {
		t.Fatal("NewWorkerRegistry() returned nil")
	}
}

func TestWorkerRegistryRegister(t *testing.T) {
	registry := NewWorkerRegistry()

	heartbeat := &Heartbeat{
		WorkerID: "worker1",
		Load:     10,
		Capacity: 100,
		Skills:   []string{"skill1", "skill2"},
		Status:   "idle",
	}

	registry.Register(heartbeat)

	hb, ok := registry.Get("worker1")
	if !ok {
		t.Error("Get() should return true for registered worker")
	}
	if hb.WorkerID != "worker1" {
		t.Errorf("WorkerID = %v, want %v", hb.WorkerID, "worker1")
	}
}

func TestWorkerRegistryUnregister(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1"})
	registry.Unregister("worker1")

	_, ok := registry.Get("worker1")
	if ok {
		t.Error("Get() should return false after Unregister()")
	}
}

func TestWorkerRegistryList(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1"})
	registry.Register(&Heartbeat{WorkerID: "worker2"})
	registry.Register(&Heartbeat{WorkerID: "worker3"})

	list := registry.List()
	if len(list) != 3 {
		t.Errorf("List() length = %d, want %d", len(list), 3)
	}
}

func TestWorkerRegistryGetAvailable(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1", Load: 50, Capacity: 100, Status: "idle"})
	registry.Register(&Heartbeat{WorkerID: "worker2", Load: 100, Capacity: 100, Status: "busy"})
	registry.Register(&Heartbeat{WorkerID: "worker3", Load: 20, Capacity: 100, Status: "idle"})

	available := registry.GetAvailable()
	if len(available) != 2 {
		t.Errorf("GetAvailable() length = %d, want %d", len(available), 2)
	}
}

func TestWorkerRegistryGetBySkill(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1", Skills: []string{"skill1", "skill2"}})
	registry.Register(&Heartbeat{WorkerID: "worker2", Skills: []string{"skill2", "skill3"}})
	registry.Register(&Heartbeat{WorkerID: "worker3", Skills: []string{"skill3"}})

	workers := registry.GetBySkill("skill2")
	if len(workers) != 2 {
		t.Errorf("GetBySkill() length = %d, want %d", len(workers), 2)
	}
}

func TestWorkerRegistryCleanStale(t *testing.T) {
	registry := NewWorkerRegistry()

	// 注册一个旧的worker
	oldHeartbeat := &Heartbeat{
		WorkerID: "old-worker",
		Load:     10,
		Capacity: 100,
		Status:   "idle",
	}
	oldHeartbeat.AliveAt = time.Now().Add(-2 * time.Minute)
	registry.Register(oldHeartbeat)

	// 注册一个新鲜的worker
	registry.Register(&Heartbeat{
		WorkerID: "new-worker",
		Load:     10,
		Capacity: 100,
		Status:   "idle",
	})

	removed := registry.CleanStale(1 * time.Minute) // 1分钟TTL

	if removed != 1 {
		t.Errorf("CleanStale() removed %d, want %d", removed, 1)
	}

	if registry.Count() != 1 {
		t.Errorf("Count() = %d, want %d", registry.Count(), 1)
	}
}

func TestWorkerRegistryCount(t *testing.T) {
	registry := NewWorkerRegistry()

	if registry.Count() != 0 {
		t.Error("Initial count should be 0")
	}

	registry.Register(&Heartbeat{WorkerID: "worker1"})
	if registry.Count() != 1 {
		t.Errorf("Count() = %d, want %d", registry.Count(), 1)
	}

	registry.Register(&Heartbeat{WorkerID: "worker2"})
	if registry.Count() != 2 {
		t.Errorf("Count() = %d, want %d", registry.Count(), 2)
	}
}

func TestWorkerRegistryUpdateLoad(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1", Load: 10})

	if !registry.UpdateLoad("worker1", 50) {
		t.Error("UpdateLoad() should return true")
	}

	hb, _ := registry.Get("worker1")
	if hb.Load != 50 {
		t.Errorf("Load = %d, want %d", hb.Load, 50)
	}
}

func TestWorkerRegistryUpdateStatus(t *testing.T) {
	registry := NewWorkerRegistry()

	registry.Register(&Heartbeat{WorkerID: "worker1", Status: "idle"})

	if !registry.UpdateStatus("worker1", "busy") {
		t.Error("UpdateStatus() should return true")
	}

	hb, _ := registry.Get("worker1")
	if hb.Status != "busy" {
		t.Errorf("Status = %v, want %v", hb.Status, "busy")
	}
}

func TestWorkerRegistryConcurrent(t *testing.T) {
	registry := NewWorkerRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			registry.Register(&Heartbeat{WorkerID: "worker" + string(rune('0'+id%10))})
		}(i)
	}

	wg.Wait()

	count := registry.Count()
	if count != 10 {
		t.Errorf("Final count = %d, want %d", count, 10)
	}
}
