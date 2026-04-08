package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("New() returned nil")
	}
}

func TestConfigString(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		defaultVal string
		want       string
	}{
		{"server host", "server.host", "localhost", "0.0.0.0"},
		{"server port", "server.port", "8080", "8080"},
		{"non-existent key", "non.existent", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.String(tt.path, tt.defaultVal)
			if tt.name == "non-existent key" {
				if got != tt.defaultVal {
					t.Errorf("String() = %v, want %v", got, tt.defaultVal)
				}
			} else {
				if got != tt.want {
					t.Errorf("String() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestConfigInt(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		defaultVal int
		want       int
	}{
		{"server port int", "server.port", 0, 8080},
		{"db max open conns", "db.max_open_conns", 0, 25},
		{"non-existent key", "non.existent", 99, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.Int(tt.path, tt.defaultVal)
			if tt.name == "non-existent key" {
				if got != tt.defaultVal {
					t.Errorf("Int() = %v, want %v", got, tt.defaultVal)
				}
			} else {
				if got != tt.want {
					t.Errorf("Int() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestConfigBool(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	got := cfg.Bool("non.existent", true)
	if got != true {
		t.Errorf("Bool() = %v, want true", got)
	}
}

func TestConfigFloat64(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	got := cfg.Float64("non.existent", 1.5)
	if got != 1.5 {
		t.Errorf("Float64() = %v, want 1.5", got)
	}
}

func TestConfigInt64(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	got := cfg.Int64("non.existent", 100)
	if got != 100 {
		t.Errorf("Int64() = %v, want 100", got)
	}
}

func TestWithEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	cfg, err := New(WithEnv("dev"))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// 验证配置加载成功
	got := cfg.String("server.host", "")
	if got != "0.0.0.0" {
		t.Errorf("Config loaded, got %v", got)
	}
}
