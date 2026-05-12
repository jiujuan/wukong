package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type Sandbox struct {
	mu      sync.RWMutex
	policy  Policy
	runners map[string]Runner
	logger  *slog.Logger
}

var (
	defaultOnce    sync.Once
	defaultSandbox *Sandbox
)

func New(opts ...Option) *Sandbox {
	s := &Sandbox{
		policy:  defaultPolicy(),
		runners: make(map[string]Runner),
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.policy = normalizePolicy(s.policy)
	s.registerBuiltins()
	return s
}

func Default() *Sandbox {
	defaultOnce.Do(func() {
		defaultSandbox = New()
	})
	return defaultSandbox
}

func Execute(ctx context.Context, req Request) (Result, error) {
	return Default().Execute(ctx, req)
}

func RegisterRunner(runtime string, runner Runner) {
	Default().RegisterRunner(runtime, runner)
}

func (s *Sandbox) RegisterRunner(runtime string, runner Runner) {
	key := normalizeName(runtime)
	if key == "" || runner == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runners == nil {
		s.runners = make(map[string]Runner)
	}
	s.runners[key] = runner
}

func (s *Sandbox) Execute(ctx context.Context, req Request) (Result, error) {
	start := time.Now()
	normalized, err := s.normalizeRequest(req)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := s.Validate(normalized); err != nil {
		return Result{Error: err.Error()}, err
	}
	runner := s.getRunner(normalized.Runtime)
	if runner == nil {
		err := fmt.Errorf("%w: %s", ErrRunnerNotFound, normalized.Runtime)
		return Result{Error: err.Error()}, err
	}
	runCtx := ctx
	cancel := func() {}
	if normalized.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, normalized.Timeout)
	}
	defer cancel()
	result, runErr := runner.Run(runCtx, normalized, s.policy)
	result.Duration = time.Since(start)
	if runErr != nil {
		if result.Error == "" {
			result.Error = runErr.Error()
		}
		if s.logger != nil {
			s.logger.Warn("[sandbox] execute failed",
				"runtime", normalized.Runtime,
				"command", normalized.Command,
				"script_path", normalized.ScriptPath,
				"error", runErr,
				"duration_ms", result.Duration.Milliseconds(),
			)
		}
		return result, runErr
	}
	if s.logger != nil {
		s.logger.Info("[sandbox] execute success",
			"runtime", normalized.Runtime,
			"command", normalized.Command,
			"script_path", normalized.ScriptPath,
			"duration_ms", result.Duration.Milliseconds(),
			"truncated", result.Truncated,
		)
	}
	return result, nil
}

func (s *Sandbox) Validate(req Request) error {
	normalized, err := s.normalizeRequest(req)
	if err != nil {
		return err
	}
	if normalized.Runtime == "" {
		return fmt.Errorf("%w: runtime is required", ErrInvalidRequest)
	}
	if normalized.Command != "" && normalized.Runtime != "command" {
		return fmt.Errorf("%w: command is only allowed for command runtime", ErrInvalidRequest)
	}
	if s.getRunner(normalized.Runtime) == nil {
		return fmt.Errorf("%w: %s", ErrRunnerNotFound, normalized.Runtime)
	}
	if normalized.Runtime == "command" {
		command := commandKey(normalized.Command)
		if command == "" {
			return fmt.Errorf("%w: command is required", ErrInvalidRequest)
		}
		if _, ok := s.policy.AllowedCommands[command]; !ok {
			return fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
		}
	}
	allowedRoots := normalized.AllowedWorkRoots
	if len(allowedRoots) == 0 {
		allowedRoots = s.policy.AllowedWorkRoots
	}
	if len(allowedRoots) > 0 {
		if normalized.WorkDir != "" && !withinAllowedRoots(normalized.WorkDir, allowedRoots) {
			return fmt.Errorf("%w: %s", ErrWorkDirNotAllowed, normalized.WorkDir)
		}
		if normalized.ScriptPath != "" && !withinAllowedRoots(normalized.ScriptPath, allowedRoots) {
			return fmt.Errorf("%w: %s", ErrWorkDirNotAllowed, normalized.ScriptPath)
		}
	}
	return nil
}

func (s *Sandbox) normalizeRequest(req Request) (Request, error) {
	req.Runtime = normalizeName(req.Runtime)
	if req.Runtime == "" && normalizeName(req.Command) != "" {
		req.Runtime = "command"
	}
	if req.Timeout <= 0 {
		req.Timeout = s.policy.DefaultTimeout
	}
	if strings.TrimSpace(req.WorkDir) == "" {
		if strings.TrimSpace(req.ScriptPath) != "" {
			req.WorkDir = filepathDir(req.ScriptPath)
		} else if wd, err := currentWorkDir(); err == nil {
			req.WorkDir = wd
		}
	}
	var err error
	if strings.TrimSpace(req.WorkDir) != "" {
		req.WorkDir, err = absPath(req.WorkDir)
		if err != nil {
			return Request{}, err
		}
	}
	if strings.TrimSpace(req.ScriptPath) != "" {
		req.ScriptPath, err = absPath(req.ScriptPath)
		if err != nil {
			return Request{}, err
		}
	}
	if len(req.AllowedWorkRoots) > 0 {
		roots := make([]string, 0, len(req.AllowedWorkRoots))
		for _, root := range req.AllowedWorkRoots {
			abs, err := absPath(root)
			if err != nil || strings.TrimSpace(abs) == "" {
				continue
			}
			roots = append(roots, abs)
		}
		req.AllowedWorkRoots = roots
	}
	req.Env = filterAllowedEnv(req.Env, s.policy.AllowedEnvKeys)
	if req.Args != nil {
		req.Args = append([]string(nil), req.Args...)
	}
	return req, nil
}

func (s *Sandbox) getRunner(runtime string) Runner {
	key := normalizeName(runtime)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runners[key]
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
