package sandbox

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Option func(*Sandbox)

func WithLogger(logger *slog.Logger) Option {
	return func(s *Sandbox) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithPolicy(policy Policy) Option {
	return func(s *Sandbox) {
		s.policy = normalizePolicy(policy)
	}
}

func WithAllowedCommands(commands ...string) Option {
	return func(s *Sandbox) {
		if s.policy.AllowedCommands == nil {
			s.policy.AllowedCommands = make(map[string]struct{})
		}
		for _, item := range commands {
			key := commandKey(item)
			if key == "" {
				continue
			}
			s.policy.AllowedCommands[key] = struct{}{}
		}
	}
}

func WithAllowedEnvKeys(keys ...string) Option {
	return func(s *Sandbox) {
		if s.policy.AllowedEnvKeys == nil {
			s.policy.AllowedEnvKeys = make(map[string]struct{})
		}
		for _, item := range keys {
			key := normalizeName(item)
			if key == "" {
				continue
			}
			s.policy.AllowedEnvKeys[key] = struct{}{}
		}
	}
}

func WithAllowedWorkRoots(roots ...string) Option {
	return func(s *Sandbox) {
		cleaned := make([]string, 0, len(roots))
		for _, root := range roots {
			abs, err := absPath(root)
			if err != nil || strings.TrimSpace(abs) == "" {
				continue
			}
			cleaned = append(cleaned, abs)
		}
		s.policy.AllowedWorkRoots = append([]string(nil), cleaned...)
	}
}

func WithMaxOutputBytes(limit int) Option {
	return func(s *Sandbox) {
		if limit > 0 {
			s.policy.MaxOutputBytes = limit
		}
	}
}

func WithDefaultTimeout(timeout time.Duration) Option {
	return func(s *Sandbox) {
		if timeout > 0 {
			s.policy.DefaultTimeout = timeout
		}
	}
}

func WithAllowNetwork(allow bool) Option {
	return func(s *Sandbox) {
		s.policy.AllowNetwork = allow
	}
}

func defaultPolicy() Policy {
	roots := defaultAllowedWorkRoots()
	return Policy{
		AllowedCommands:  defaultAllowedCommands(),
		AllowedEnvKeys:   defaultAllowedEnvKeys(),
		AllowedWorkRoots: roots,
		MaxOutputBytes:   1 << 20,
		DefaultTimeout:   20 * time.Second,
		AllowNetwork:     false,
	}
}

func normalizePolicy(policy Policy) Policy {
	if policy.AllowedCommands == nil {
		policy.AllowedCommands = defaultAllowedCommands()
	} else {
		normalized := make(map[string]struct{}, len(policy.AllowedCommands))
		for key := range policy.AllowedCommands {
			if normalizedKey := commandKey(key); normalizedKey != "" {
				normalized[normalizedKey] = struct{}{}
			}
		}
		policy.AllowedCommands = normalized
	}
	if policy.AllowedEnvKeys == nil {
		policy.AllowedEnvKeys = defaultAllowedEnvKeys()
	}
	if len(policy.AllowedWorkRoots) == 0 {
		policy.AllowedWorkRoots = defaultAllowedWorkRoots()
	}
	if policy.MaxOutputBytes <= 0 {
		policy.MaxOutputBytes = 1 << 20
	}
	if policy.DefaultTimeout <= 0 {
		policy.DefaultTimeout = 20 * time.Second
	}
	return policy
}

func defaultAllowedCommands() map[string]struct{} {
	commands := []string{
		"command",
		"go",
		"python",
		"py",
		"python3",
		"python3.11",
		"javascript",
		"js",
		"node",
		"bash",
		"sh",
		"shell",
		"powershell",
		"ps1",
		"pwsh",
		"javac",
		"java",
		"typescript",
		"ts",
		"tsx",
		"ts-node",
		"deno",
	}
	result := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		result[commandKey(command)] = struct{}{}
	}
	return result
}

func defaultAllowedEnvKeys() map[string]struct{} {
	keys := []string{
		"PATH",
		"HOME",
		"USERPROFILE",
		"HOMEDRIVE",
		"HOMEPATH",
		"TEMP",
		"TMP",
		"TMPDIR",
		"SYSTEMROOT",
		"WINDIR",
		"COMSPEC",
		"PATHEXT",
		"LANG",
		"LC_ALL",
		"TERM",
		"GOCACHE",
		"GOMODCACHE",
		"GOPATH",
		"GOROOT",
	}
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		result[strings.ToLower(key)] = struct{}{}
	}
	return result
}

func defaultAllowedWorkRoots() []string {
	roots := make([]string, 0, 2)
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		if abs, err := filepath.Abs(wd); err == nil {
			roots = append(roots, abs)
		}
	}
	if tmp := os.TempDir(); strings.TrimSpace(tmp) != "" {
		if abs, err := filepath.Abs(tmp); err == nil {
			roots = append(roots, abs)
		}
	}
	return roots
}
