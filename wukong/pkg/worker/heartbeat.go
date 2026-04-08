package worker

import (
	"sync"
	"time"
)

// Heartbeat 心跳信息
type Heartbeat struct {
	WorkerID    string            `json:"worker_id"`
	AliveAt     time.Time        `json:"alive_at"`
	Load        int               `json:"load"`        // 当前负载
	Capacity    int               `json:"capacity"`    // 容量
	Skills      []string          `json:"skills"`      // 支持的技能
	Status      string            `json:"status"`      // 状态
}

// WorkerRegistry Worker注册表
type WorkerRegistry struct {
	mu      sync.RWMutex
	workers map[string]*Heartbeat
}

// NewWorkerRegistry 创建Worker注册表
func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		workers: make(map[string]*Heartbeat),
	}
}

// Register 注册Worker
func (wr *WorkerRegistry) Register(heartbeat *Heartbeat) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	// 如果 AliveAt 未设置，则设置为当前时间
	if heartbeat.AliveAt.IsZero() {
		heartbeat.AliveAt = time.Now()
	}
	wr.workers[heartbeat.WorkerID] = heartbeat
}

// Unregister 注销Worker
func (wr *WorkerRegistry) Unregister(workerID string) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	delete(wr.workers, workerID)
}

// Get 获取Worker信息
func (wr *WorkerRegistry) Get(workerID string) (*Heartbeat, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	hb, ok := wr.workers[workerID]
	return hb, ok
}

// List 获取所有Worker
func (wr *WorkerRegistry) List() []*Heartbeat {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	result := make([]*Heartbeat, 0, len(wr.workers))
	for _, hb := range wr.workers {
		result = append(result, hb)
	}
	return result
}

// GetAvailable 获取可用的Worker（负载较低的）
func (wr *WorkerRegistry) GetAvailable() []*Heartbeat {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	result := make([]*Heartbeat, 0)
	for _, hb := range wr.workers {
		if hb.Status == "idle" && hb.Load < hb.Capacity {
			result = append(result, hb)
		}
	}
	return result
}

// GetBySkill 获取支持指定技能的Worker
func (wr *WorkerRegistry) GetBySkill(skill string) []*Heartbeat {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	result := make([]*Heartbeat, 0)
	for _, hb := range wr.workers {
		for _, s := range hb.Skills {
			if s == skill {
				result = append(result, hb)
				break
			}
		}
	}
	return result
}

// CleanStale 清理过期的Worker
func (wr *WorkerRegistry) CleanStale(ttl time.Duration) int {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, hb := range wr.workers {
		if now.Sub(hb.AliveAt) > ttl {
			delete(wr.workers, id)
			removed++
		}
	}
	return removed
}

// Count 获取Worker数量
func (wr *WorkerRegistry) Count() int {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	return len(wr.workers)
}

// UpdateLoad 更新Worker负载
func (wr *WorkerRegistry) UpdateLoad(workerID string, load int) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	hb, ok := wr.workers[workerID]
	if !ok {
		return false
	}
	hb.Load = load
	return true
}

// UpdateStatus 更新Worker状态
func (wr *WorkerRegistry) UpdateStatus(workerID string, status string) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	hb, ok := wr.workers[workerID]
	if !ok {
		return false
	}
	hb.Status = status
	return true
}
