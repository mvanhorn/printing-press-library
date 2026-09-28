package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func projectionDiagnostic(t *testing.T, mode, fields string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "cuisines", "bar", "--data-source", "local", "--home", t.TempDir(), mode, "--select", fields)
	cmd.Env = recipeEnv(t, t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestMachineAllInvalidProjectionHasOneDiagnostic(t *testing.T) {
	for _, mode := range []string{"--agent", "--json"} {
		t.Run(mode, func(t *testing.T) {
			res := projectionDiagnostic(t, mode, "items.missing,items.also_missing")
			assertMachineDiagnostic(t, res, 2)
			var diagnostic map[string]any
			if err := json.Unmarshal(res.stderr, &diagnostic); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(diagnostic["error"].(string), "--select matched no fields") {
				t.Fatalf("unhelpful projection diagnostic: %s", res.stderr)
			}
		})
	}
}

func TestMachinePartialProjectionKeepsWarning(t *testing.T) {
	res := projectionDiagnostic(t, "--agent", "items.name,items.missing")
	equal(t, res.code, 0)
	if !strings.Contains(string(res.stderr), `warning: --select "items.missing" matched no fields`) {
		t.Fatalf("partial match warning lost: %s", res.stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.stdout, &payload); err != nil {
		t.Fatal(err)
	}
	if len(items(t, payload)) == 0 || items(t, payload)[0]["name"] == nil {
		t.Fatal("successful selected data lost")
	}
}

func TestHumanAllInvalidProjectionKeepsCobraError(t *testing.T) {
	res := projectionDiagnostic(t, "--plain", "items.missing")
	equal(t, res.code, 2)
	if !strings.HasPrefix(string(res.stderr), "Error: --select matched no fields:") || strings.Contains(string(res.stderr), "warning:") {
		t.Fatalf("human projection error changed shape: %s", res.stderr)
	}
}
