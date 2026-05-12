package tool

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jiujuan/wukong/pkg/llm"
	pkglogger "github.com/jiujuan/wukong/pkg/logger"
	"github.com/jiujuan/wukong/pkg/skills"
)

type Option func(*Manager)

func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = pkglogger.FromSlog(logger)
		}
	}
}

func WithLLMProvider(provider *llm.Provider) Option {
	return func(m *Manager) {
		m.llmProvider = provider
	}
}

func WithSkillsRegistry(registry *skills.Registry) Option {
	return func(m *Manager) {
		m.skillsRegistry = registry
	}
}

func WithMemoryStore(store MemoryStore) Option {
	return func(m *Manager) {
		if store != nil {
			m.memoryStore = store
		}
	}
}

func WithBaseDir(baseDir string) Option {
	return func(m *Manager) {
		if strings.TrimSpace(baseDir) != "" {
			m.baseDir = baseDir
		}
	}
}

func WithFileWriteDir(dir string) Option {
	return func(m *Manager) {
		if strings.TrimSpace(dir) != "" {
			m.fileWriteDir = dir
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(m *Manager) {
		if client != nil {
			m.httpClient = client
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		if timeout > 0 {
			m.execTimeout = timeout
		}
	}
}
