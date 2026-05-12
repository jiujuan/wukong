package sandbox

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPolicyProvidesSafeDefaults(t *testing.T) {
	policy := defaultPolicy()
	if len(policy.AllowedCommands) == 0 {
		t.Fatal("default AllowedCommands is empty")
	}
	if len(policy.AllowedEnvKeys) == 0 {
		t.Fatal("default AllowedEnvKeys is empty")
	}
	if len(policy.AllowedWorkRoots) == 0 {
		t.Fatal("default AllowedWorkRoots is empty")
	}
	if policy.MaxOutputBytes <= 0 {
		t.Fatalf("MaxOutputBytes = %d, want > 0", policy.MaxOutputBytes)
	}
	if policy.DefaultTimeout <= 0 {
		t.Fatalf("DefaultTimeout = %s, want > 0", policy.DefaultTimeout)
	}
	if policy.AllowNetwork {
		t.Fatal("AllowNetwork = true, want false")
	}
}

func TestNormalizePolicyCanonicalizesCommandsAndDefaults(t *testing.T) {
	policy := normalizePolicy(Policy{
		AllowedCommands: map[string]struct{}{
			" Go ":           {},
			"C:/tool/PY.EXE": {},
		},
		AllowedEnvKeys:   nil,
		AllowedWorkRoots: nil,
		MaxOutputBytes:   0,
		DefaultTimeout:   0,
	})

	if _, ok := policy.AllowedCommands["go"]; !ok {
		t.Fatalf("normalized commands missing go: %#v", policy.AllowedCommands)
	}
	if _, ok := policy.AllowedCommands["py"]; !ok {
		t.Fatalf("normalized commands missing py: %#v", policy.AllowedCommands)
	}
	if len(policy.AllowedEnvKeys) == 0 {
		t.Fatal("normalized AllowedEnvKeys is empty")
	}
	if len(policy.AllowedWorkRoots) == 0 {
		t.Fatal("normalized AllowedWorkRoots is empty")
	}
	if policy.MaxOutputBytes <= 0 {
		t.Fatalf("MaxOutputBytes = %d, want > 0", policy.MaxOutputBytes)
	}
	if policy.DefaultTimeout <= 0 {
		t.Fatalf("DefaultTimeout = %s, want > 0", policy.DefaultTimeout)
	}
}

func TestSandboxOptionsApplyPolicy(t *testing.T) {
	root := t.TempDir()
	s := New(
		WithAllowedCommands("Go", "python.exe"),
		WithAllowedEnvKeys("FOO", "BAR"),
		WithAllowedWorkRoots(root),
		WithMaxOutputBytes(2048),
		WithDefaultTimeout(3*time.Second),
		WithAllowNetwork(true),
	)

	if _, ok := s.policy.AllowedCommands["go"]; !ok {
		t.Fatalf("AllowedCommands missing go: %#v", s.policy.AllowedCommands)
	}
	if _, ok := s.policy.AllowedCommands["python"]; !ok {
		t.Fatalf("AllowedCommands missing python: %#v", s.policy.AllowedCommands)
	}
	if _, ok := s.policy.AllowedEnvKeys["foo"]; !ok {
		t.Fatalf("AllowedEnvKeys missing foo: %#v", s.policy.AllowedEnvKeys)
	}
	if _, ok := s.policy.AllowedEnvKeys["bar"]; !ok {
		t.Fatalf("AllowedEnvKeys missing bar: %#v", s.policy.AllowedEnvKeys)
	}
	if len(s.policy.AllowedWorkRoots) != 1 {
		t.Fatalf("AllowedWorkRoots = %#v, want one root", s.policy.AllowedWorkRoots)
	}
	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("filepath.Abs(root) error = %v", err)
	}
	if got := filepath.Clean(s.policy.AllowedWorkRoots[0]); got != filepath.Clean(wantRoot) {
		t.Fatalf("AllowedWorkRoots[0] = %q, want %q", got, wantRoot)
	}
	if s.policy.MaxOutputBytes != 2048 {
		t.Fatalf("MaxOutputBytes = %d, want 2048", s.policy.MaxOutputBytes)
	}
	if s.policy.DefaultTimeout != 3*time.Second {
		t.Fatalf("DefaultTimeout = %s, want 3s", s.policy.DefaultTimeout)
	}
	if !s.policy.AllowNetwork {
		t.Fatal("AllowNetwork = false, want true")
	}
}
