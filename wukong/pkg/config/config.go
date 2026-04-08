package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Option 函数选项模式
type Option func(*options)

type options struct {
	configPath string
	env        string
}

// WithConfigPath 设置配置文件路径
func WithConfigPath(path string) Option {
	return func(o *options) {
		o.configPath = path
	}
}

// WithEnv 设置环境
func WithEnv(env string) Option {
	return func(o *options) {
		o.env = env
	}
}

// Config 配置管理
type Config struct {
	k *koanf.Koanf
}

// New 新建配置实例
func New(opts ...Option) (*Config, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	k := koanf.New(".")

	// 设置默认值
	defaults(k)

	// 确定配置文件路径
	configPath := o.configPath
	if configPath == "" {
		envName := o.env
		if envName == "" {
			envName = os.Getenv("APP_ENV")
		}
		if envName == "" {
			envName = "dev"
		}
		configPath = fmt.Sprintf("configs/%s.yaml", envName)
	}

	// 加载配置文件
	if resolved := resolveConfigPath(configPath); resolved != "" {
		if err := k.Load(file.Provider(resolved), kyaml.Parser()); err != nil {
			return nil, fmt.Errorf("load config file failed: %w", err)
		}
	}

	// 加载环境变量
	if err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.Replace(strings.ToLower(s), "_", ".", -1)
	}), nil); err != nil {
		return nil, fmt.Errorf("load env failed: %w", err)
	}

	return &Config{k: k}, nil
}

func resolveConfigPath(path string) string {
	candidates := []string{
		path,
		filepath.Join(".", path),
		filepath.Join("..", path),
	}
	for _, item := range candidates {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if _, err := os.Stat(item); err == nil {
			return item
		}
	}
	return ""
}

// defaults 设置默认值
func defaults(k *koanf.Koanf) {
	k.Set("server.host", "0.0.0.0")
	k.Set("server.port", 8080)
	k.Set("server.mode", "debug")
	k.Set("db.host", "localhost")
	k.Set("db.port", 5432)
	k.Set("db.user", "wukong")
	k.Set("db.password", "wukong123")
	k.Set("db.database", "wukong")
	k.Set("db.max_open_conns", 25)
	k.Set("db.max_idle_conns", 5)
	k.Set("db.conn_max_lifetime", 300)
	k.Set("redis.host", "localhost")
	k.Set("redis.port", 6379)
	k.Set("redis.password", "")
	k.Set("redis.db", 0)
	k.Set("redis.pool_size", 10)
	k.Set("jwt.secret", "wukong-secret-key-change-in-production")
	k.Set("jwt.expire_hours", 2)
	k.Set("log.level", "info")
	k.Set("log.format", "json")
	k.Set("llm.provider", "deepseek")
	k.Set("llm.base_url", "https://api.deepseek.com/v1")
	k.Set("llm.api_key", "")
	k.Set("llm.model", "deepseek-chat")
	k.Set("llm.timeout", 60)
	k.Set("llm.pool.enabled", false)
	k.Set("llm.pool.max_retries", 1)
	k.Set("llm.pool.failure_threshold", 3)
	k.Set("llm.pool.cooldown_sec", 15)
	k.Set("llm.pool.retry_backoff_ms", 200)
	k.Set("llm.fallback1.provider", "")
	k.Set("llm.fallback1.base_url", "")
	k.Set("llm.fallback1.api_key", "")
	k.Set("llm.fallback1.model", "")
	k.Set("llm.fallback2.provider", "")
	k.Set("llm.fallback2.base_url", "")
	k.Set("llm.fallback2.api_key", "")
	k.Set("llm.fallback2.model", "")
	k.Set("skills.root_dir", "skills")
	k.Set("skills.poll_interval_sec", 3)
	k.Set("skills.exec_timeout_sec", 60)
}

// String 获取字符串
func (c *Config) String(path, defaultVal string) string {
	if c.k.Exists(path) {
		return c.k.String(path)
	}
	return defaultVal
}

// Int 获取整数
func (c *Config) Int(path string, defaultVal int) int {
	if c.k.Exists(path) {
		return c.k.Int(path)
	}
	return defaultVal
}

// Int64 获取64位整数
func (c *Config) Int64(path string, defaultVal int64) int64 {
	if c.k.Exists(path) {
		return c.k.Int64(path)
	}
	return defaultVal
}

// Bool 获取布尔
func (c *Config) Bool(path string, defaultVal bool) bool {
	if c.k.Exists(path) {
		return c.k.Bool(path)
	}
	return defaultVal
}

// Float64 获取浮点数
func (c *Config) Float64(path string, defaultVal float64) float64 {
	if c.k.Exists(path) {
		return c.k.Float64(path)
	}
	return defaultVal
}
