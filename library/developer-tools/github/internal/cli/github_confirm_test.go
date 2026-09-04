package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirmDestructive_YesAndDryRunSkipPrompt(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "delete"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := confirmDestructive(cmd, &rootFlags{yes: true}); err != nil {
		t.Fatalf("--yes should skip confirmation: %v", err)
	}
	if err := confirmDestructive(cmd, &rootFlags{dryRun: true}); err != nil {
		t.Fatalf("--dry-run should skip confirmation: %v", err)
	}
}

func TestConfirmDestructive_NonInteractiveRequiresYes(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "delete"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := confirmDestructive(cmd, &rootFlags{noInput: true})
	if err == nil {
		t.Fatal("expected usage error without --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want mention of --yes", err)
	}
	err = confirmDestructive(cmd, &rootFlags{agent: true})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("agent mode should require --yes, got %v", err)
	}
}

func TestConfirmDestructive_PipedStdinRequiresYes(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "delete-file"}
	cmd.SetIn(strings.NewReader(`{"message":"remove file","sha":"abc"}`))
	cmd.SetOut(os.Stdout)
	cmd.SetErr(&bytes.Buffer{})
	err := confirmDestructive(cmd, &rootFlags{})
	if err == nil {
		t.Fatal("piped stdin must require --yes so the JSON body is not consumed as a y/N answer")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want mention of --yes", err)
	}
}
