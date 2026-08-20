// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain isolates the CLI-owned session path for the whole package.
//
// PATCH(cli-owned-workos-session): loadTokensRaw now probes a CLI-owned session
// ahead of the desktop stores. Without this, every token-resolution test would
// pick up the developer's real session at ~/.local/share/granola-pp-cli and
// assert against it -- the tests set GRANOLA_SUPPORT_DIR, which redirects the
// Granola-owned files, but the CLI session deliberately does not live there.
//
// Individual tests that care about the session still override the path
// themselves; this only guarantees no test can reach the real one by default.
func TestMain(m *testing.M) {
	if os.Getenv("GRANOLA_CLI_SESSION_PATH") == "" {
		dir, err := os.MkdirTemp("", "granola-session-isolation-")
		if err != nil {
			panic("granola tests: cannot create session isolation dir: " + err.Error())
		}
		os.Setenv("GRANOLA_CLI_SESSION_PATH", filepath.Join(dir, "session.json"))
		code := m.Run()
		os.RemoveAll(dir)
		os.Exit(code)
	}
	os.Exit(m.Run())
}
