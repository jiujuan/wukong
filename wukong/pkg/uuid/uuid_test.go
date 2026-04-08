package uuid

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	id := New()
	if id == "" {
		t.Error("New() returned empty string")
	}
	if len(id) != 36 { // UUID格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		t.Errorf("New() length = %d, want 36", len(id))
	}
}

func TestNewShort(t *testing.T) {
	id := NewShort()
	if id == "" {
		t.Error("NewShort() returned empty string")
	}
	if len(id) != 8 {
		t.Errorf("NewShort() length = %d, want 8", len(id))
	}
}

func TestNewTaskID(t *testing.T) {
	id := NewTaskID()
	if id == "" {
		t.Error("NewTaskID() returned empty string")
	}
	if !strings.HasPrefix(id, "task_") {
		t.Errorf("NewTaskID() should start with 'task_', got %s", id)
	}
}

func TestNewSessionID(t *testing.T) {
	id := NewSessionID()
	if id == "" {
		t.Error("NewSessionID() returned empty string")
	}
	if !strings.HasPrefix(id, "sess_") {
		t.Errorf("NewSessionID() should start with 'sess_', got %s", id)
	}
}

func TestNewMsgID(t *testing.T) {
	id := NewMsgID()
	if id == "" {
		t.Error("NewMsgID() returned empty string")
	}
	if !strings.HasPrefix(id, "msg_") {
		t.Errorf("NewMsgID() should start with 'msg_', got %s", id)
	}
}

func TestNewSubTaskID(t *testing.T) {
	id := NewSubTaskID()
	if id == "" {
		t.Error("NewSubTaskID() returned empty string")
	}
	if !strings.HasPrefix(id, "sub_") {
		t.Errorf("NewSubTaskID() should start with 'sub_', got %s", id)
	}
}

func TestNewUserID(t *testing.T) {
	id := NewUserID()
	if id == "" {
		t.Error("NewUserID() returned empty string")
	}
	if !strings.HasPrefix(id, "user_") {
		t.Errorf("NewUserID() should start with 'user_', got %s", id)
	}
}

func TestParse(t *testing.T) {
	original := "550e8400-e29b-41d4-a716-446655440000"
	parsed, err := Parse(original)
	if err != nil {
		t.Errorf("Parse() error = %v", err)
	}
	if parsed.String() != original {
		t.Errorf("Parse() = %v, want %v", parsed.String(), original)
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := Parse("invalid-uuid")
	if err == nil {
		t.Error("Parse() should return error for invalid UUID")
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"invalid UUID", "invalid-uuid", false},
		{"empty string", "", false},
		{"too short", "550e8400", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := New()
		if ids[id] {
			t.Errorf("Duplicate UUID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestIDPrefixes(t *testing.T) {
	// 确保每种ID都有正确的长度
	taskID := NewTaskID()
	if len(taskID) != 5+12 { // "task_" + 12位
		t.Errorf("TaskID length = %d, want %d", len(taskID), 5+12)
	}

	sessID := NewSessionID()
	if len(sessID) != 5+12 { // "sess_" + 12位
		t.Errorf("SessionID length = %d, want %d", len(sessID), 5+12)
	}

	msgID := NewMsgID()
	if len(msgID) != 4+12 { // "msg_" + 12位
		t.Errorf("MsgID length = %d, want %d", len(msgID), 4+12)
	}
}
