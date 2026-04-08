package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		opts   []Option
		wantOK bool
	}{
		{
			name:   "default logger",
			opts:   []Option{},
			wantOK: true,
		},
		{
			name: "with format",
			opts: []Option{WithFormat("text")},
			wantOK: true,
		},
		{
			name: "with level",
			opts: []Option{WithLevel(slog.LevelDebug)},
			wantOK: true,
		},
		{
			name: "with output",
			opts: []Option{WithOutput(&bytes.Buffer{})},
			wantOK: true,
		},
		{
			name: "all options",
			opts: []Option{
				WithFormat("json"),
				WithLevel(slog.LevelInfo),
				WithOutput(&bytes.Buffer{}),
				WithAddSource(true),
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.opts...)
			if logger == nil {
				t.Error("New() returned nil")
			}
		})
	}
}

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("json"))

	logger.Info("test info message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Error("Info() should produce output")
	}

	// 验证是有效的JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
}

func TestLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("json"), WithLevel(slog.LevelDebug))

	logger.Debug("test debug message")

	output := buf.String()
	if output == "" {
		t.Error("Debug() should produce output")
	}
}

func TestLoggerWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("json"))

	logger.Warn("test warning message")

	output := buf.String()
	if output == "" {
		t.Error("Warn() should produce output")
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("json"))

	logger.Error("test error message")

	output := buf.String()
	if output == "" {
		t.Error("Error() should produce output")
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("json"))

	logger.With("key1", "value1").Info("test with attrs")

	output := buf.String()
	if output == "" {
		t.Error("With() should produce output")
	}
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithOutput(&buf), WithFormat("text"))

	logger.Info("test text format")

	output := buf.String()
	if output == "" {
		t.Error("text format should produce output")
	}
}
