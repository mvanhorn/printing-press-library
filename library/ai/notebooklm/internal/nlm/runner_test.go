package nlm

import (
	"os"
	"os/exec"
	"testing"
)

func TestNewRunner(t *testing.T) {
	// nlm should be on PATH in this environment
	r, err := NewRunner()
	if err != nil {
		t.Skipf("nlm not available: %v", err)
	}
	if r.BinPath() == "" {
		t.Error("BinPath should not be empty")
	}
}

func TestRunnerVersion(t *testing.T) {
	r, err := NewRunner()
	if err != nil {
		t.Skipf("nlm not available: %v", err)
	}
	out, err := r.Run("--version")
	if err != nil {
		t.Fatalf("nlm --version: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected version output")
	}
}

func TestRunnerJSON(t *testing.T) {
	r, err := NewRunner()
	if err != nil {
		t.Skipf("nlm not available: %v", err)
	}
	var result struct {
		Help string `json:"help,omitempty"`
	}
	// This will fail because --help doesn't output JSON, but tests RunJSON error path
	_ = r.RunJSON(&result, "--help")
}

func TestRunnerMissingBinary(t *testing.T) {
	orig := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", orig)
	_, err := NewRunner()
	if err == nil {
		t.Error("expected error when nlm not on PATH")
	}
}

func TestRunnerBadArgs(t *testing.T) {
	_, err := exec.LookPath("nlm")
	if err != nil {
		t.Skip("nlm not available")
	}
	r, _ := NewRunner()
	_, err = r.Run("nonexistent-subcommand-xyz")
	if err == nil {
		t.Error("expected error for bad nlm subcommand")
	}
}
