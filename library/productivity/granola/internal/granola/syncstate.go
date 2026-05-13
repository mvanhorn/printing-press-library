// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

package granola

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SyncState records the outcome of the most recent sync attempt so that
// doctor can report on it without itself invoking safestorage.Decrypt
// (which would trigger the Keychain prompt - inappropriate for a
// diagnostic command).
//
// The file lives at $XDG_DATA_HOME/granola-pp-cli/sync_state.json (or
// ~/.local/share/granola-pp-cli/sync_state.json on macOS by default).
// Path is overridable via GRANOLA_SYNC_STATE_PATH for tests.
type SyncState struct {
	LastSyncAt           time.Time `json:"last_sync_at"`
	LastDecryptStatus    string    `json:"last_decrypt_status"` // "ok" | "failed" | "skipped"
	LastDecryptErrorClass string   `json:"last_decrypt_error_class,omitempty"`
	LastDecryptErrorMsg  string    `json:"last_decrypt_error_msg,omitempty"`
	LastTokenSource      string    `json:"last_token_source,omitempty"`
	LastDocumentsFetched int       `json:"last_documents_fetched,omitempty"`
	// LastHydrateErrorMsg carries an error from the /v2/get-documents API
	// hydration step, distinct from decrypt failures. The two have different
	// remediation paths (auth/network vs. Keychain) and should not be
	// surfaced through a single field.
	LastHydrateErrorMsg string `json:"last_hydrate_error_msg,omitempty"`
}

// DecryptStatus enum (string-typed to keep the JSON stable).
const (
	DecryptStatusOK      = "ok"
	DecryptStatusFailed  = "failed"
	DecryptStatusSkipped = "skipped"
)

// SyncStatePath returns the file path the helpers read and write. The
// directory is created on first write (WriteSyncState).
func SyncStatePath() string {
	if v := os.Getenv("GRANOLA_SYNC_STATE_PATH"); v != "" {
		return v
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "granola-pp-cli", "sync_state.json")
}

// ReadSyncState loads the most recent sync state, or returns
// (SyncState{}, os.ErrNotExist) if the file does not exist. Other read
// errors are wrapped. Malformed JSON is treated as "no record": callers
// should display "no recent sync" rather than crash.
func ReadSyncState() (SyncState, error) {
	path := SyncStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncState{}, err
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		// Malformed file - treat as missing rather than failing doctor.
		return SyncState{}, os.ErrNotExist
	}
	return s, nil
}

// WriteSyncState writes the current state atomically (tmp file + rename
// so a concurrent reader never sees a half-written JSON blob). Creates
// the parent directory if it does not exist.
func WriteSyncState(s SyncState) error {
	path := SyncStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("syncstate: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("syncstate: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("syncstate: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("syncstate: rename: %w", err)
	}
	return nil
}

// IsSyncStateMissing returns true if err signals the file does not yet
// exist (i.e. no sync has run on this machine yet).
func IsSyncStateMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
