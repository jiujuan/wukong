package statemachine

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

// 任务状态
const (
	StatusPending   = "PENDING"   // 待处理
	StatusPlanning  = "PLANNING"  // 规划中
	StatusRunning   = "RUNNING"   // 运行中
	StatusWaiting   = "WAITING"   // 等待中
	StatusCompleted = "COMPLETED" // 已完成
	StatusFailed    = "FAILED"    // 失败
	StatusCancelled = "CANCELLED" // 已取消
)

// 子任务状态
const (
	SubStatusPending = "PENDING" // 待处理
	SubStatusRunning = "RUNNING" // 运行中
	SubStatusSuccess = "SUCCESS" // 成功
	SubStatusFailed  = "FAILED"  // 失败
	SubStatusSkipped = "SKIPPED" // 跳过
)

// Option 函数选项模式
type Option func(*StateMachine)

// StateMachine 状态机
type StateMachine struct {
	mu          sync.RWMutex
	transitions map[string][]string             // 状态转换规则
	onEnter     map[string]func(string, string) // 进入状态回调
	onExit      map[string]func(string, string) // 退出状态回调
	logger      *pkglogger.Logger
}

// New 创建状态机
func New(opts ...Option) *StateMachine {
	sm := &StateMachine{
		transitions: make(map[string][]string),
		onEnter:     make(map[string]func(string, string)),
		onExit:      make(map[string]func(string, string)),
		logger:      pkglogger.New(),
	}

	// 默认转换规则
	sm.initDefaultTransitions()

	for _, opt := range opts {
		opt(sm)
	}

	sm.logger.Info("[StateMachine] initialized", "transition_states", len(sm.transitions))
	return sm
}

func WithLogger(logger *slog.Logger) Option {
	return func(sm *StateMachine) {
		sm.logger = pkglogger.FromSlog(logger)
	}
}

// WithTransition 添加转换规则
func WithTransition(from string, to ...string) Option {
	return func(sm *StateMachine) {
		sm.transitions[from] = append(sm.transitions[from], to...)
	}
}

// WithOnEnter 设置进入状态回调
func WithOnEnter(status string, callback func(from, to string)) Option {
	return func(sm *StateMachine) {
		sm.onEnter[status] = callback
	}
}

// WithOnExit 设置退出状态回调
func WithOnExit(status string, callback func(from, to string)) Option {
	return func(sm *StateMachine) {
		sm.onExit[status] = callback
	}
}

// initDefaultTransitions 初始化默认转换规则
func (sm *StateMachine) initDefaultTransitions() {
	// 主任务状态转换
	sm.transitions[StatusPending] = []string{StatusPlanning, StatusCancelled}
	sm.transitions[StatusPlanning] = []string{StatusRunning, StatusFailed, StatusCancelled}
	sm.transitions[StatusRunning] = []string{StatusWaiting, StatusCompleted, StatusFailed, StatusCancelled}
	sm.transitions[StatusWaiting] = []string{StatusRunning, StatusCompleted, StatusFailed, StatusCancelled}
	sm.transitions[StatusCompleted] = []string{}
	sm.transitions[StatusFailed] = []string{StatusPending} // 可以重试
	sm.transitions[StatusCancelled] = []string{}
}

// CanTransition 检查是否可以转换
func (sm *StateMachine) CanTransition(from, to string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	allowedStates, exists := sm.transitions[from]
	if !exists {
		sm.logger.Warn("[StateMachine] transition check failed: source state missing", "from", from, "to", to)
		return false
	}

	for _, state := range allowedStates {
		if state == to {
			sm.logger.Debug("[StateMachine] transition check passed", "from", from, "to", to)
			return true
		}
	}
	sm.logger.Warn("[StateMachine] transition check failed: target state not allowed", "from", from, "to", to, "allowed", allowedStates)
	return false
}

// Transition 执行状态转换
func (sm *StateMachine) Transition(from, to string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否可以转换（内联检查，避免锁冲突）
	allowedStates, exists := sm.transitions[from]
	if !exists {
		sm.logger.Error("[StateMachine] transition failed: source state missing", "from", from, "to", to)
		return fmt.Errorf("invalid transition from %s to %s", from, to)
	}
	found := false
	for _, state := range allowedStates {
		if state == to {
			found = true
			break
		}
	}
	if !found {
		sm.logger.Error("[StateMachine] transition failed: target state not allowed", "from", from, "to", to, "allowed", allowedStates)
		return fmt.Errorf("invalid transition from %s to %s", from, to)
	}

	// 执行退出回调
	if callback, exists := sm.onExit[from]; exists {
		callback(from, to)
	}

	// 执行进入回调
	if callback, exists := sm.onEnter[to]; exists {
		callback(from, to)
	}

	sm.logger.Info("[StateMachine] transition success", "from", from, "to", to)
	return nil
}

// GetAllowedTransitions 获取允许的转换
func (sm *StateMachine) GetAllowedTransitions(from string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.transitions[from]
}

// IsFinalState 判断是否为终态
func (sm *StateMachine) IsFinalState(status string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	allowed, exists := sm.transitions[status]
	if !exists {
		return true
	}
	return len(allowed) == 0
}

// Error 状态机错误
type Error struct {
	From string
	To   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("invalid transition from %s to %s", e.From, e.To)
}

// NewTransitionError 创建转换错误
func NewTransitionError(from, to string) *Error {
	return &Error{From: from, To: to}
}

// IsTransitionError 判断是否为转换错误
func IsTransitionError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}
