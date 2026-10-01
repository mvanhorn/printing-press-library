package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportRequiresExplicitConfirmation(t *testing.T) {
	cmd := newImportCmd(&rootFlags{})
	cmd.SetArgs([]string{"me", "--input", "unopened.jsonl"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected confirmation before reading input or making requests, got %v", err)
	}
}

func TestImportDryRunDoesNotRequireConfirmation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SEEK_HOME", root)
	input := filepath.Join(root, "input.jsonl")
	if err := os.WriteFile(input, []byte("{\"query\":\"mutation { example }\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newImportCmd(&rootFlags{})
	cmd.SetArgs([]string{"me", "--input", input, "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry run requires no confirmation: %v", err)
	}
}
