package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type processRunner struct {
	plan func(context.Context, Request) (commandPlan, error)
}

type commandPlan struct {
	command string
	args    []string
	env     map[string]string
}

func (r processRunner) Run(ctx context.Context, req Request, policy Policy) (Result, error) {
	if r.plan == nil {
		return Result{Error: "process plan is nil"}, fmt.Errorf("process plan is nil")
	}
	plan, err := r.plan(ctx, req)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := validateAllowedCommand(policy, plan.command); err != nil {
		return Result{Error: err.Error()}, err
	}
	req.Command = plan.command
	req.Args = append([]string(nil), plan.args...)
	if len(plan.env) > 0 {
		if req.Env == nil {
			req.Env = make(map[string]string, len(plan.env))
		}
		for key, value := range plan.env {
			req.Env[key] = value
		}
	}
	return runProcess(ctx, req, policy)
}

func runProcess(ctx context.Context, req Request, policy Policy) (Result, error) {
	result := Result{}
	if strings.TrimSpace(req.Command) == "" {
		err := fmt.Errorf("%w: command is empty", ErrInvalidRequest)
		result.Error = err.Error()
		return result, err
	}
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx2, req.Command, req.Args...)
	if strings.TrimSpace(req.WorkDir) != "" {
		cmd.Dir = req.WorkDir
	}
	cmd.Env = buildCommandEnv(policy, req.Env)
	if strings.TrimSpace(req.Input) != "" {
		cmd.Stdin = strings.NewReader(req.Input)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	collector := newOutputCollector(policy.MaxOutputBytes, cancel)
	if err := cmd.Start(); err != nil {
		result.Error = err.Error()
		return result, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(collector.stdoutWriter(), stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(collector.stderrWriter(), stderr)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	result.Stdout = collector.stdout.String()
	result.Stderr = collector.stderr.String()
	result.Truncated = collector.truncated
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if collector.truncated {
		result.Error = ErrOutputLimitExceeded.Error()
		return result, ErrOutputLimitExceeded
	}
	if waitErr != nil {
		if result.Error == "" {
			result.Error = waitErr.Error()
		}
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, waitErr
	}
	return result, nil
}

type outputCollector struct {
	maxBytes  int
	cancel    context.CancelFunc
	mu        sync.Mutex
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	total     int
	truncated bool
}

func newOutputCollector(maxBytes int, cancel context.CancelFunc) *outputCollector {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return &outputCollector{maxBytes: maxBytes, cancel: cancel}
}

func (c *outputCollector) stdoutWriter() io.Writer {
	return writerFunc(func(p []byte) (int, error) {
		return c.write(&c.stdout, p)
	})
}

func (c *outputCollector) stderrWriter() io.Writer {
	return writerFunc(func(p []byte) (int, error) {
		return c.write(&c.stderr, p)
	})
}

func (c *outputCollector) write(buf *bytes.Buffer, p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.truncated {
		return 0, ErrOutputLimitExceeded
	}
	if c.total+len(p) > c.maxBytes {
		remain := c.maxBytes - c.total
		if remain > 0 {
			_, _ = buf.Write(p[:remain])
			c.total += remain
		}
		c.truncated = true
		if c.cancel != nil {
			c.cancel()
		}
		return len(p), ErrOutputLimitExceeded
	}
	n, err := buf.Write(p)
	c.total += n
	return n, err
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

func buildCommandEnv(policy Policy, requestEnv map[string]string) []string {
	allowed := policy.AllowedEnvKeys
	if allowed == nil {
		allowed = defaultAllowedEnvKeys()
	}
	envMap := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := allowed[strings.ToLower(key)]; exists {
			envMap[key] = value
		}
	}
	for key, value := range requestEnv {
		if _, exists := allowed[strings.ToLower(key)]; exists {
			envMap[key] = value
		}
	}
	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+envMap[key])
	}
	return result
}

func absPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	return filepath.Abs(path)
}

func filepathDir(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Dir(path)
}

func currentWorkDir() (string, error) {
	return os.Getwd()
}

func filterAllowedEnv(src map[string]string, allowed map[string]struct{}) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	if allowed == nil {
		allowed = defaultAllowedEnvKeys()
	}
	dst := make(map[string]string)
	for key, value := range src {
		if _, ok := allowed[strings.ToLower(key)]; ok {
			dst[key] = value
		}
	}
	return dst
}

func withinAllowedRoots(path string, roots []string) bool {
	if strings.TrimSpace(path) == "" {
		return true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		normalized := filepath.Clean(rootAbs)
		if strings.EqualFold(abs, normalized) {
			return true
		}
		prefix := normalized + string(filepath.Separator)
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(strings.ToLower(abs), strings.ToLower(prefix)) {
				return true
			}
			continue
		}
		if strings.HasPrefix(abs, prefix) {
			return true
		}
	}
	return false
}

func validateAllowedCommand(policy Policy, command string) error {
	key := commandKey(command)
	if key == "" {
		return fmt.Errorf("%w: command is required", ErrInvalidRequest)
	}
	if len(policy.AllowedCommands) == 0 {
		return fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
	}
	if _, ok := policy.AllowedCommands[key]; ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
}

func commandKey(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	base := filepath.Base(command)
	lower := strings.ToLower(base)
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com", ".ps1"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSuffix(lower, suffix)
		}
	}
	return lower
}
