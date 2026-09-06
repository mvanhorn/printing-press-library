package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil/testenv"
)

// isolateNovelTest sandboxes every path-resolving directory the CLI can touch
// and disables the cache auto-refresh hook, so a test that executes the root
// command can never reach the operator's real store.
func isolateNovelTest(t *testing.T) string {
	t.Helper()
	home := testenv.Isolate(t, cliutil.DataDir, cliutil.StateDir, cliutil.ConfigDir, cliutil.CacheDir)
	t.Setenv("SEATS_AERO_NO_AUTO_REFRESH", "1")
	t.Setenv("SEATS_AERO_API_KEY", "test-key-not-real")
	return home
}

func executeRoot(args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	cmd := RootCmd()
	out, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	return out, stderr, cmd.Execute()
}

func TestNovelCommandsNeverTouchDefaultStore(t *testing.T) {
	home := isolateNovelTest(t)
	dbPath := defaultDBPath("seats-aero-pp-cli")
	if !strings.HasPrefix(dbPath, home) {
		t.Fatalf("default DB path %q is outside isolated home %q", dbPath, home)
	}

	commands := [][]string{
		{"new-since", "--json"},
		{"calendar", "--origin", "JFK", "--destination", "NRT", "--json"},
		{"direct-scan", "--json"},
	}
	for _, args := range commands {
		stdout, stderr, err := executeRoot(args...)
		if err != nil {
			t.Fatalf("%s returned error on empty store: %v (stderr=%q)", args[0], err, stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "[]" {
			t.Fatalf("%s stdout=%q, want []", args[0], stdout.String())
		}
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("default DB %q exists or returned unexpected error: %v", dbPath, err)
	}
}
