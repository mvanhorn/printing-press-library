package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil/testenv"
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

func TestImportSavedProfileCannotConfirm(t *testing.T) {
	root := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
		"saved-yes": {Name: "saved-yes", Values: map[string]string{"yes": "true"}},
	}}); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"--no-learn", "import", "me", "--profile", "saved-yes", "--input", filepath.Join(root, "unopened.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("saved profile must not confirm a live import: %v", err)
	}
}

func TestImportConfirmedThroughRoot(t *testing.T) {
	root := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	input := filepath.Join(root, "input.jsonl")
	// A comment-only file exercises the live import path without an API call.
	if err := os.WriteFile(input, []byte("# no records\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--yes", "--no-learn", "import", "me", "--input", input},
		{"--no-learn", "import", "me", "--input", input, "--yes"},
	} {
		cmd := RootCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("confirmed import %v: %v", args, err)
		}
	}
}
