package skills

import (
	"log/slog"
	"strings"
	"time"
)

type Option func(*Registry)

func WithRootDir(rootDir string) Option {
	return func(r *Registry) {
		if strings.TrimSpace(rootDir) != "" {
			r.rootDir = rootDir
		}
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(r *Registry) {
		if interval > 0 {
			r.pollInterval = interval
		}
	}
}

func WithExecTimeout(timeout time.Duration) Option {
	return func(r *Registry) {
		if timeout > 0 {
			r.execTimeout = timeout
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(r *Registry) {
		if logger != nil {
			r.logger = logger
		}
	}
}

func WithMetaStore(store MetaStore) Option {
	return func(r *Registry) {
		r.store = store
	}
}
