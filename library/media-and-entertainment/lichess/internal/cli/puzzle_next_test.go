package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPuzzleNextRequiresThemeAndExecutesOneDryRun(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"puzzle", "next", "--angle", "pin", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("puzzle next dry run: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Fatalf("unexpected puzzle follow-up dry-run output: %s", out.String())
	}
}

func TestPuzzleNextRejectsMissingTheme(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"puzzle", "next", "--dry-run", "--json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--angle is required") {
		t.Fatalf("puzzle next without --angle error = %v, want required-theme error", err)
	}
}
