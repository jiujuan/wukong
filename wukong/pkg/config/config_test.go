package config

import (
	"os"
	"path/filepath"
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

func TestConfigResolvePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")
	content := []byte("server:\n  host: \"127.0.0.1\"\n")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}

	cfg, err := New(WithConfigPath(configPath))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if got := cfg.Dir(); got != dir {
		t.Fatalf("Dir() = %q, want %q", got, dir)
	}

	want := filepath.Join(dir, "storage", "output_data")
	if got := cfg.ResolvePath("storage/output_data"); got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestSkillsRootDirDefaultAndResolvePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")
	content := []byte("server:\n  host: \"127.0.0.1\"\n")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}

	cfg, err := New(WithConfigPath(configPath))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if got := cfg.String("skills.root_dir", "../skills"); got != "../skills" {
		t.Fatalf("skills.root_dir = %q, want ../skills", got)
	}
	want := filepath.Join(dir, "..", "skills")
	if got := cfg.ResolvePath(cfg.String("skills.root_dir", "../skills")); got != filepath.Clean(want) {
		t.Fatalf("resolved skills root = %q, want %q", got, filepath.Clean(want))
	}
}
