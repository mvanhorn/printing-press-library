// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// seedTwoAccountStore writes a tokens.json with two accounts whose
// LastUsedAt differ so resolver-related tests can assert deterministic
// step-3 (last-used) behaviour. The "older" account ends up with the smaller
// LastUsedAt timestamp.
func seedTwoAccountStore(t *testing.T, tokenStorePath, newer, older string) {
	t.Helper()
	store := auth.NewStoreAt(tokenStorePath)
	now := time.Now().UnixMilli()
	for email, lastUsed := range map[string]int64{
		newer: now,
		older: now - 10_000,
	} {
		if _, err := store.Upsert(email, auth.AccountTokens{
			Type:         "google",
			RefreshToken: "rt-" + email,
			SuperhumanToken: auth.SuperhumanToken{
				Token:   "id-" + email,
				Expires: now + int64((time.Hour).Milliseconds()),
			},
			LastUsedAt: lastUsed,
		}); err != nil {
			t.Fatalf("seed %s: %v", email, err)
		}
	}
}

// readConfigActiveEmail parses config.toml and returns its active_email
// field. The reader is a small TOML round-trip rather than re-using
// config.Load so the test asserts on the bytes on disk, not on the
// internal Config struct (which would also accept env-var overrides etc.).
func readConfigActiveEmail(t *testing.T, configPath string) string {
	t.Helper()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Failure paths don't write the config — missing file means
			// active_email is unset, which is what the caller is asserting.
			return ""
		}
		t.Fatalf("read config %s: %v", configPath, err)
	}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse config %s: %v\n%s", configPath, err, data)
	}
	if v, ok := raw["active_email"].(string); ok {
		return v
	}
	return ""
}

// TestAuthUse_SetsActiveEmail covers the happy path: the named account is in
// the store, the command writes Config.ActiveEmail to disk and prints success.
func TestAuthUse_SetsActiveEmail(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedTwoAccountStore(t, tokenStorePath, "user@example.com", "user2@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "use", "user@example.com")
	if err != nil {
		t.Fatalf("auth use: %v", err)
	}
	if !strings.Contains(stdout, "Active account: user@example.com") {
		t.Fatalf("expected success line, got: %s", stdout)
	}
	if got := readConfigActiveEmail(t, configPath); got != "user@example.com" {
		t.Fatalf("active_email on disk: got %q want %q", got, "user@example.com")
	}
}

// TestAuthUse_UnknownAccount_ListsValid covers the error path: the named
// account is missing from the store. The error must surface both the
// rejected email and the available emails so the user sees a fix in the
// same line.
func TestAuthUse_UnknownAccount_ListsValid(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedTwoAccountStore(t, tokenStorePath, "user@example.com", "user2@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "auth", "use", "unknown@example.com")
	if err == nil {
		t.Fatalf("auth use: expected error for unknown account")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown@example.com") {
		t.Fatalf("expected unknown email in error: %s", msg)
	}
	if !strings.Contains(msg, "user@example.com") || !strings.Contains(msg, "user2@example.com") {
		t.Fatalf("expected available emails in error: %s", msg)
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d want 4 (auth)", got)
	}
	// Active email must NOT be written on the failure path.
	if got := readConfigActiveEmail(t, configPath); got != "" {
		t.Fatalf("active_email should be empty on failure, got %q", got)
	}
}

// TestAuthUse_Clear writes a non-empty active email first, then clears it.
// The on-disk active_email must be empty after the clear.
func TestAuthUse_Clear(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedTwoAccountStore(t, tokenStorePath, "user@example.com", "user2@example.com")

	if _, _, err := executeCmd(t, "--config", configPath, "auth", "use", "user@example.com"); err != nil {
		t.Fatalf("initial auth use: %v", err)
	}
	if got := readConfigActiveEmail(t, configPath); got != "user@example.com" {
		t.Fatalf("pre-clear active_email: got %q want user@example.com", got)
	}

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "use", "--clear")
	if err != nil {
		t.Fatalf("auth use --clear: %v", err)
	}
	if !strings.Contains(stdout, "cleared") {
		t.Fatalf("expected clear acknowledgement, got: %s", stdout)
	}
	if got := readConfigActiveEmail(t, configPath); got != "" {
		t.Fatalf("post-clear active_email: got %q want empty", got)
	}
}

// TestAuthUse_NoArgsNoClear ensures a missing positional + missing --clear
// surfaces a usage error (exit code 2) rather than silently succeeding.
func TestAuthUse_NoArgsNoClear(t *testing.T) {
	configPath, _ := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")

	_, _, err := executeCmd(t, "--config", configPath, "auth", "use")
	if err == nil {
		t.Fatalf("auth use: expected error when no arg and no --clear")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode = %d want 2 (usage)", got)
	}
}

// TestAuthStatus_AfterAuthUse_MarksActive ensures auth status renders the
// active row with the `*` prefix and reflects the JSON `active: true` flag.
// This is the user-visible feedback loop after `auth use` — without it the
// user has no way to confirm the pin took effect short of opening
// config.toml directly.
func TestAuthStatus_AfterAuthUse_MarksActive(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedTwoAccountStore(t, tokenStorePath, "user@example.com", "user2@example.com")

	if _, _, err := executeCmd(t, "--config", configPath, "auth", "use", "user@example.com"); err != nil {
		t.Fatalf("auth use: %v", err)
	}

	// Human output: the line for user@example.com must have a `*` marker;
	// the inactive line must not.
	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "status")
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}
	lines := strings.Split(stdout, "\n")
	var activeLine, inactiveLine string
	for _, l := range lines {
		if strings.Contains(l, "user@example.com") {
			activeLine = l
		}
		if strings.Contains(l, "user2@example.com") {
			inactiveLine = l
		}
	}
	if activeLine == "" || inactiveLine == "" {
		t.Fatalf("expected both account lines, got: %s", stdout)
	}
	if !strings.Contains(activeLine, "* ") {
		t.Fatalf("expected `* ` marker on active row, got: %q", activeLine)
	}
	if strings.Contains(inactiveLine, "* ") {
		t.Fatalf("expected NO `* ` marker on inactive row, got: %q", inactiveLine)
	}

	// JSON output: active=true on the pinned row, false elsewhere.
	stdout2, _, err := executeCmd(t, "--config", configPath, "--json", "auth", "status")
	if err != nil {
		t.Fatalf("auth status --json: %v", err)
	}
	var rows []map[string]any
	if jerr := json.Unmarshal([]byte(stdout2), &rows); jerr != nil {
		t.Fatalf("parse JSON: %v\n%s", jerr, stdout2)
	}
	for _, r := range rows {
		email, _ := r["email"].(string)
		active, _ := r["active"].(bool)
		switch email {
		case "user@example.com":
			if !active {
				t.Fatalf("expected active=true for user@example.com: %v", r)
			}
		case "user2@example.com":
			if active {
				t.Fatalf("expected active=false for user2@example.com: %v", r)
			}
		}
	}
}
