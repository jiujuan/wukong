package statemachine

import (
	"sync"
	"time"
)

// TaskStateMachine 任务状态机
type TaskStateMachine struct {
	*StateMachine
	mu          sync.RWMutex
	taskStates  map[string]string        // task_id -> status
	taskHistory map[string][]StateChange // task_id -> 状态变更历史
}

// StateChange 状态变更记录
type StateChange struct {
	From   string
	To     string
	Time   time.Time
	Reason string
}

// NewTaskStateMachine 创建任务状态机
func NewTaskStateMachine(opts ...Option) *TaskStateMachine {
	sm := &TaskStateMachine{
		StateMachine: New(opts...),
		taskStates:   make(map[string]string),
		taskHistory:  make(map[string][]StateChange),
	}

	return sm
}

// SetInitialState 设置初始状态
func (tsm *TaskStateMachine) SetInitialState(taskID, status string) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	tsm.taskStates[taskID] = status
	tsm.taskHistory[taskID] = []StateChange{
		{From: "", To: status, Time: time.Now()},
	}
	tsm.logger.Info("[TaskStateMachine] set initial state", "task_id", taskID, "to", status)
}

// GetState 获取任务当前状态
func (tsm *TaskStateMachine) GetState(taskID string) string {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	return tsm.taskStates[taskID]
}

// ChangeState 变更任务状态
func (tsm *TaskStateMachine) ChangeState(taskID, to, reason string) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	from := tsm.taskStates[taskID]
	if from == "" {
		tsm.logger.Error("[TaskStateMachine] state change failed: task not found", "task_id", taskID, "to", to, "reason", reason)
		return NewTransitionError("", to)
	}

	// 内联检查转换（避免锁冲突）
	allowedStates, exists := tsm.transitions[from]
	if !exists {
		tsm.logger.Error("[TaskStateMachine] state change failed: source state missing", "task_id", taskID, "from", from, "to", to, "reason", reason)
		return NewTransitionError(from, to)
	}
	found := false
	for _, state := range allowedStates {
		if state == to {
			found = true
			break
		}
	}
	if !found {
		tsm.logger.Warn("[TaskStateMachine] state change rejected", "task_id", taskID, "from", from, "to", to, "reason", reason, "allowed", allowedStates)
		return NewTransitionError(from, to)
	}

	// 更新状态
	tsm.taskStates[taskID] = to

	// 记录历史
	change := StateChange{
		From:   from,
		To:     to,
		Time:   time.Now(),
		Reason: reason,
	}
	tsm.taskHistory[taskID] = append(tsm.taskHistory[taskID], change)

	tsm.logger.Info("[TaskStateMachine] state changed", "task_id", taskID, "from", from, "to", to, "reason", reason)
	return nil
}

// GetHistory 获取状态变更历史
func (tsm *TaskStateMachine) GetHistory(taskID string) []StateChange {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	history, exists := tsm.taskHistory[taskID]
	if !exists {
		return nil
	}

	result := make([]StateChange, len(history))
	copy(result, history)
	return result
}

// RemoveTask 移除任务状态记录
func (tsm *TaskStateMachine) RemoveTask(taskID string) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	delete(tsm.taskStates, taskID)
	delete(tsm.taskHistory, taskID)
	tsm.logger.Info("[TaskStateMachine] task removed", "task_id", taskID)
}

// IsCompleted 判断任务是否完成
func (tsm *TaskStateMachine) IsCompleted(taskID string) bool {
	state := tsm.GetState(taskID)
	return state == StatusCompleted || state == StatusFailed || state == StatusCancelled
}

// IsPending 判断任务是否待处理
func (tsm *TaskStateMachine) IsPending(taskID string) bool {
	state := tsm.GetState(taskID)
	return state == StatusPending
}

// IsRunning 判断任务是否运行中
func (tsm *TaskStateMachine) IsRunning(taskID string) bool {
	state := tsm.GetState(taskID)
	return state == StatusRunning || state == StatusPlanning || state == StatusWaiting
}
