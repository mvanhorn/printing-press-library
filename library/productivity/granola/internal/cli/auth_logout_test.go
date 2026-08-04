// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// `auth logout` used to clear only the config API key, leaving the CLI-owned
// session -- a live access and refresh token -- on disk while printing
// "Credentials cleared". Every later command stayed authenticated and kept
// refreshing, and `auth status` still reported the account as signed in. For a
// command whose entire purpose is removing a credential, reporting success
// while the credential survives is the worst available failure.
func TestAuthLogoutClearsCLISession(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "granola-pp-cli", "session.json")
	t.Setenv("GRANOLA_CLI_SESSION_PATH", sessionPath)
	t.Setenv("GRANOLA_API_KEY", "")

	if err := granola.SaveCLISession(granola.CLISession{
		AccessToken:  "access-token-value-aaaaaaaaaaaa",
		RefreshToken: "refresh-token-value-bbbb",
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		AccountEmail: "someone@example.com",
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if !granola.HasCLISession() {
		t.Fatal("precondition: session should exist before logout")
	}

	epochBefore := granola.RevocationEpoch()
	flags := &rootFlags{configPath: filepath.Join(dir, "config.json")}
	cmd := newAuthLogoutCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("auth logout: %v", err)
	}

	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("logout reported success but the session file is still on disk")
	}
	if granola.HasCLISession() {
		t.Error("still signed in after logout")
	}
	// Logout must also record the revocation, which is how a token rotation
	// running concurrently in another process learns not to republish the
	// session it is midway through writing.
	if after := granola.RevocationEpoch(); after == epochBefore {
		t.Error("logout did not record a revocation; a concurrent refresh can restore the session after logout reports success")
	}
}
