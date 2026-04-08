package statemachine

import (
	"testing"
)

func TestNew(t *testing.T) {
	sm := New()
	if sm == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCanTransition(t *testing.T) {
	sm := New()

	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"pending to planning", StatusPending, StatusPlanning, true},
		{"pending to cancelled", StatusPending, StatusCancelled, true},
		{"pending to running", StatusPending, StatusRunning, false},
		{"planning to running", StatusPlanning, StatusRunning, true},
		{"running to completed", StatusRunning, StatusCompleted, true},
		{"completed to pending", StatusCompleted, StatusPending, false}, // 终态不能转换
		{"failed to pending", StatusFailed, StatusPending, true},      // 失败可以重试
		{"cancelled to completed", StatusCancelled, StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sm.CanTransition(tt.from, tt.to); got != tt.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestTransition(t *testing.T) {
	sm := New()

	err := sm.Transition(StatusPending, StatusPlanning)
	if err != nil {
		t.Errorf("Transition() error = %v", err)
	}
}

func TestTransitionInvalid(t *testing.T) {
	sm := New()

	err := sm.Transition(StatusPending, StatusCompleted)
	if err == nil {
		t.Error("Transition() should return error for invalid transition")
	}
}

func TestGetAllowedTransitions(t *testing.T) {
	sm := New()

	tests := []struct {
		status string
		want   int
	}{
		{StatusPending, 2},    // planning, cancelled
		{StatusRunning, 4},     // waiting, completed, failed, cancelled
		{StatusCompleted, 0},  // 终态
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			transitions := sm.GetAllowedTransitions(tt.status)
			if len(transitions) != tt.want {
				t.Errorf("GetAllowedTransitions(%s) length = %d, want %d", tt.status, len(transitions), tt.want)
			}
		})
	}
}

func TestIsFinalState(t *testing.T) {
	sm := New()

	tests := []struct {
		status string
		want   bool
	}{
		{StatusCompleted, true},
		{StatusCancelled, true},
		{StatusPending, false},
		{StatusRunning, false},
		{StatusFailed, false}, // 失败可以重试，不是真正的终态
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := sm.IsFinalState(tt.status); got != tt.want {
				t.Errorf("IsFinalState(%s) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestWithTransition(t *testing.T) {
	sm := New(
		WithTransition("CUSTOM", "CUSTOM_DONE"),
	)

	if !sm.CanTransition("CUSTOM", "CUSTOM_DONE") {
		t.Error("Custom transition should be allowed")
	}
}

func TestWithOnEnterCallback(t *testing.T) {
	called := false
	sm := New(
		WithOnEnter(StatusCompleted, func(from, to string) {
			called = true
		}),
	)

	sm.Transition(StatusRunning, StatusCompleted)

	if !called {
		t.Error("OnEnter callback should be called")
	}
}

func TestWithOnExitCallback(t *testing.T) {
	called := false
	sm := New(
		WithOnExit(StatusRunning, func(from, to string) {
			called = true
		}),
	)

	sm.Transition(StatusRunning, StatusCompleted)

	if !called {
		t.Error("OnExit callback should be called")
	}
}

func TestNewTransitionError(t *testing.T) {
	err := NewTransitionError("A", "B")
	if err.From != "A" {
		t.Errorf("From = %v, want %v", err.From, "A")
	}
	if err.To != "B" {
		t.Errorf("To = %v, want %v", err.To, "B")
	}
}

func TestErrorInterface(t *testing.T) {
	err := NewTransitionError("A", "B")
	expected := "invalid transition from A to B"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}
