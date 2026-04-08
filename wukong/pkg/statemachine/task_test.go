package statemachine

import (
	"testing"
)

func TestNewTaskStateMachine(t *testing.T) {
	tsm := NewTaskStateMachine()
	if tsm == nil {
		t.Fatal("NewTaskStateMachine() returned nil")
	}
}

func TestSetInitialState(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)

	state := tsm.GetState("task1")
	if state != StatusPending {
		t.Errorf("GetState() = %v, want %v", state, StatusPending)
	}
}

func TestChangeState(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)

	err := tsm.ChangeState("task1", StatusPlanning, "testing")
	if err != nil {
		t.Errorf("ChangeState() error = %v", err)
	}

	state := tsm.GetState("task1")
	if state != StatusPlanning {
		t.Errorf("GetState() = %v, want %v", state, StatusPlanning)
	}
}

func TestChangeStateInvalid(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)

	err := tsm.ChangeState("task1", StatusCompleted, "invalid")
	if err == nil {
		t.Error("ChangeState() should return error for invalid transition")
	}
}

func TestGetHistory(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)
	tsm.ChangeState("task1", StatusPlanning, "step1")
	tsm.ChangeState("task1", StatusRunning, "step2")

	history := tsm.GetHistory("task1")
	if len(history) != 3 {
		t.Errorf("History length = %d, want %d", len(history), 3)
	}

	// 验证初始状态
	if history[0].From != "" {
		t.Errorf("First history From = %v, want empty", history[0].From)
	}
	if history[0].To != StatusPending {
		t.Errorf("First history To = %v, want %v", history[0].To, StatusPending)
	}
}

func TestGetHistoryWithReasons(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)
	tsm.ChangeState("task1", StatusPlanning, "initial planning")

	history := tsm.GetHistory("task1")
	if history[1].Reason != "initial planning" {
		t.Errorf("Reason = %v, want %v", history[1].Reason, "initial planning")
	}
}

func TestIsCompleted(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)

	if tsm.IsCompleted("task1") {
		t.Error("IsCompleted() should return false for PENDING")
	}

	// 正确的状态转换流程: Pending -> Planning -> Running -> Completed
	tsm.ChangeState("task1", StatusPlanning, "planning")
	tsm.ChangeState("task1", StatusRunning, "running")
	tsm.ChangeState("task1", StatusCompleted, "done")

	if !tsm.IsCompleted("task1") {
		t.Error("IsCompleted() should return true for COMPLETED")
	}
}

func TestIsPending(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)

	if !tsm.IsPending("task1") {
		t.Error("IsPending() should return true for PENDING")
	}

	tsm.ChangeState("task1", StatusPlanning, "planning")

	if tsm.IsPending("task1") {
		t.Error("IsPending() should return false for non-PENDING")
	}
}

func TestIsRunning(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)
	if tsm.IsRunning("task1") {
		t.Error("IsRunning() should return false for PENDING")
	}

	// 正确的状态转换流程: Pending -> Planning -> Running
	tsm.ChangeState("task1", StatusPlanning, "planning")
	tsm.ChangeState("task1", StatusRunning, "running")
	if !tsm.IsRunning("task1") {
		t.Error("IsRunning() should return true for RUNNING")
	}

	// Waiting 从 Running 转换
	tsm.ChangeState("task1", StatusWaiting, "waiting")
	if !tsm.IsRunning("task1") {
		t.Error("IsRunning() should return true for WAITING")
	}
}

func TestRemoveTask(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)
	tsm.RemoveTask("task1")

	state := tsm.GetState("task1")
	if state != "" {
		t.Errorf("GetState() after RemoveTask() = %v, want empty", state)
	}
}

func TestMultipleTasks(t *testing.T) {
	tsm := NewTaskStateMachine()

	tsm.SetInitialState("task1", StatusPending)
	tsm.SetInitialState("task2", StatusPending)
	tsm.SetInitialState("task3", StatusPending)

	// task1: Pending -> Planning -> Running -> Completed
	tsm.ChangeState("task1", StatusPlanning, "planning1")
	tsm.ChangeState("task1", StatusRunning, "running1")
	tsm.ChangeState("task1", StatusCompleted, "done1")

	// task2: Pending -> Planning -> Running
	tsm.ChangeState("task2", StatusPlanning, "planning2")
	tsm.ChangeState("task2", StatusRunning, "running2")

	if tsm.GetState("task1") != StatusCompleted {
		t.Error("task1 should be COMPLETED")
	}
	if tsm.GetState("task2") != StatusRunning {
		t.Error("task2 should be RUNNING")
	}
	if tsm.GetState("task3") != StatusPending {
		t.Error("task3 should be PENDING")
	}
}
