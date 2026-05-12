package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (s *Sandbox) registerBuiltins() {
	s.RegisterRunner("command", processRunner{plan: func(_ context.Context, req Request) (commandPlan, error) {
		command := strings.TrimSpace(req.Command)
		if command == "" {
			return commandPlan{}, fmt.Errorf("command is required")
		}
		if strings.ContainsAny(command, `/\`) {
			return commandPlan{}, fmt.Errorf("command path is not allowed: %s", command)
		}
		return commandPlan{command: command, args: append([]string(nil), req.Args...)}, nil
	}})
	s.RegisterRunner("python", processRunner{plan: scriptCommandPlan("python", "python3", "py")})
	s.RegisterRunner("py", processRunner{plan: scriptCommandPlan("python", "python3", "py")})
	s.RegisterRunner("python3", processRunner{plan: scriptCommandPlan("python", "python3", "py")})
	s.RegisterRunner("javascript", processRunner{plan: scriptCommandPlan("node")})
	s.RegisterRunner("js", processRunner{plan: scriptCommandPlan("node")})
	s.RegisterRunner("node", processRunner{plan: scriptCommandPlan("node")})
	s.RegisterRunner("bash", processRunner{plan: scriptCommandPlan("bash", "sh")})
	s.RegisterRunner("sh", processRunner{plan: scriptCommandPlan("bash", "sh")})
	s.RegisterRunner("shell", processRunner{plan: scriptCommandPlan("bash", "sh")})
	s.RegisterRunner("powershell", processRunner{plan: powershellPlan})
	s.RegisterRunner("ps1", processRunner{plan: powershellPlan})
	s.RegisterRunner("go", processRunner{plan: goPlan})
	s.RegisterRunner("java", javaRunner{})
	s.RegisterRunner("typescript", tsRunner{})
	s.RegisterRunner("ts", tsRunner{})
}

func scriptCommandPlan(candidates ...string) func(context.Context, Request) (commandPlan, error) {
	return func(_ context.Context, req Request) (commandPlan, error) {
		if strings.TrimSpace(req.ScriptPath) == "" {
			return commandPlan{}, fmt.Errorf("script path is required")
		}
		command, ok := lookPathAny(candidates...)
		if !ok {
			return commandPlan{}, fmt.Errorf("%s runtime not found in PATH: tried %s", candidates[0], strings.Join(candidates, ", "))
		}
		scriptPath := req.ScriptPath
		switch commandKey(command) {
		case "bash", "sh", "shell":
			if runtime.GOOS == "windows" {
				scriptPath = filepath.ToSlash(scriptPath)
				if len(scriptPath) >= 2 && scriptPath[1] == ':' {
					scriptPath = "/" + strings.ToLower(scriptPath[:1]) + scriptPath[2:]
				}
			}
		}
		return commandPlan{command: command, args: []string{scriptPath}}, nil
	}
}

func powershellPlan(_ context.Context, req Request) (commandPlan, error) {
	if strings.TrimSpace(req.ScriptPath) == "" {
		return commandPlan{}, fmt.Errorf("script path is required")
	}
	shell, err := findPowerShell()
	if err != nil {
		return commandPlan{}, err
	}
	return commandPlan{
		command: shell,
		args:    []string{"-ExecutionPolicy", "Bypass", "-File", req.ScriptPath},
	}, nil
}

func goPlan(_ context.Context, req Request) (commandPlan, error) {
	if strings.TrimSpace(req.ScriptPath) == "" {
		return commandPlan{}, fmt.Errorf("script path is required")
	}
	command, ok := lookPathAny("go")
	if !ok {
		return commandPlan{}, fmt.Errorf("go runtime not found in PATH")
	}
	env := map[string]string{}
	if strings.TrimSpace(req.Env["GOCACHE"]) == "" {
		env["GOCACHE"] = filepath.Join(os.TempDir(), "wukong-go-cache")
	}
	return commandPlan{command: command, args: []string{"run", req.ScriptPath}, env: env}, nil
}

type javaRunner struct{}

func (javaRunner) Run(ctx context.Context, req Request, policy Policy) (Result, error) {
	if strings.TrimSpace(req.ScriptPath) == "" {
		return Result{Error: "script path is required"}, fmt.Errorf("script path is required")
	}
	base := filepath.Base(req.ScriptPath)
	className := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.TrimSpace(className) == "" {
		return Result{Error: "java class name is empty"}, fmt.Errorf("java class name is empty")
	}
	tempDir, err := os.MkdirTemp("", "wukong-java-*")
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer os.RemoveAll(tempDir)
	javac, ok := lookPathAny("javac")
	if !ok {
		return Result{Error: "javac runtime not found in PATH"}, fmt.Errorf("javac runtime not found in PATH")
	}
	javaCmd, ok := lookPathAny("java")
	if !ok {
		return Result{Error: "java runtime not found in PATH"}, fmt.Errorf("java runtime not found in PATH")
	}

	compile := processRunner{plan: func(_ context.Context, _ Request) (commandPlan, error) {
		return commandPlan{command: javac, args: []string{"-d", tempDir, req.ScriptPath}}, nil
	}}
	compileReq := req
	compileReq.Command = javac
	compileRes, compileErr := compile.Run(ctx, compileReq, policy)
	if compileErr != nil {
		compileRes.Error = compileErr.Error()
		return compileRes, compileErr
	}

	run := processRunner{plan: func(_ context.Context, _ Request) (commandPlan, error) {
		return commandPlan{command: javaCmd, args: []string{"-cp", tempDir, className}}, nil
	}}
	runReq := req
	runReq.Command = javaCmd
	runRes, runErr := run.Run(ctx, runReq, policy)
	if runErr != nil {
		runRes.Error = runErr.Error()
		return runRes, runErr
	}
	return runRes, nil
}

type tsRunner struct{}

func (tsRunner) Run(ctx context.Context, req Request, policy Policy) (Result, error) {
	if strings.TrimSpace(req.ScriptPath) == "" {
		return Result{Error: "script path is required"}, fmt.Errorf("script path is required")
	}
	command, args, err := locateTypeScriptCommand(req.ScriptPath)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	return processRunner{plan: func(_ context.Context, _ Request) (commandPlan, error) {
		return commandPlan{command: command, args: args}, nil
	}}.Run(ctx, req, policy)
}

func locateTypeScriptCommand(scriptPath string) (string, []string, error) {
	if cmd, ok := lookPathAny("tsx", "ts-node"); ok {
		return cmd, []string{scriptPath}, nil
	}
	if deno, ok := lookPathAny("deno"); ok {
		return deno, []string{"run", "--allow-read", scriptPath}, nil
	}
	return "", nil, fmt.Errorf("typescript runtime not found: tried tsx, ts-node, deno")
}

func lookPathAny(names ...string) (string, bool) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

func findPowerShell() (string, error) {
	candidates := []string{"pwsh", "powershell"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("powershell runtime not found in PATH: tried %s", strings.Join(candidates, ", "))
}

func (s *Sandbox) initBuiltinsOnce() {
	_ = runtime.GOOS
}
