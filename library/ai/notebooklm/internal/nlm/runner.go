package nlm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type Runner struct {
	bin string
}

func NewRunner() (*Runner, error) {
	bin, err := exec.LookPath("nlm")
	if err != nil {
		return nil, fmt.Errorf("nlm not found on PATH: install with 'pipx install notebooklm-mcp-cli' or 'uv tool install notebooklm-mcp-cli'")
	}
	return &Runner{bin: bin}, nil
}

func (r *Runner) BinPath() string { return r.bin }

// Run executes nlm with the given arguments and returns stdout.
func (r *Runner) Run(args ...string) ([]byte, error) {
	cmd := exec.Command(r.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nlm %s: %w\n%s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// RunJSON executes nlm with --json appended and unmarshals into dst.
func (r *Runner) RunJSON(dst any, args ...string) error {
	args = append(args, "--json")
	out, err := r.Run(args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, dst)
}
