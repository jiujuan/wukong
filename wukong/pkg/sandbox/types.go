package sandbox

import (
	"context"
	"time"
)

type Request struct {
	Runtime    string            `json:"runtime"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	ScriptPath string            `json:"script_path,omitempty"`
	Code       string            `json:"code,omitempty"`
	WorkDir    string            `json:"work_dir,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Input      string            `json:"input,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
}

type Result struct {
	Stdout    string        `json:"stdout,omitempty"`
	Stderr    string        `json:"stderr,omitempty"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration,omitempty"`
	Truncated bool          `json:"truncated,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type Policy struct {
	AllowedCommands  map[string]struct{}
	AllowedEnvKeys   map[string]struct{}
	AllowedWorkRoots []string
	MaxOutputBytes   int
	DefaultTimeout   time.Duration
	AllowNetwork     bool
}

type Runner interface {
	Run(ctx context.Context, req Request, policy Policy) (Result, error)
}
