// Package syncstate persists the CLI's last-sync state to disk so doctor
// can report status without re-invoking sync (which would hit the API).
//
// Lesson from granola's post-print patches: doctor must distinguish
// "never synced", "synced cleanly", and "sync failed with class X" without
// triggering a fresh API call or keychain prompt. Writing a small JSON file
// on every sync exit captures the answer.
package syncstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State captures everything doctor needs to summarize sync health.
type State struct {
	LastSyncedAt     time.Time `json:"last_synced_at,omitempty"`
	LastAttemptedAt  time.Time `json:"last_attempted_at"`
	OK               bool      `json:"ok"`
	ErrorClass       string    `json:"error_class,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	TokenSource      string    `json:"token_source,omitempty"`
	RunsHydrated     int       `json:"runs_hydrated"`
	ActorsHydrated   int       `json:"actors_hydrated"`
	DatasetsHydrated int       `json:"datasets_hydrated"`
	ItemsHydrated    int       `json:"items_hydrated"`
}

// DefaultPath returns ~/.local/share/apify-pp-cli/sync_state.json,
// matching the cache_path doctor surfaces.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "sync_state.json"
	}
	return filepath.Join(home, ".local", "share", "apify-pp-cli", "sync_state.json")
}

// Load reads sync state from disk. Missing file returns (zero-value State, nil)
// so the caller can treat "never synced" as a first-class state without
// special error handling.
func Load(path string) (*State, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("reading sync state: %w", err)
	}
	s := &State{}
	if err := json.Unmarshal(data, s); err != nil {
		// Corrupt state file — treat as never synced rather than failing the
		// caller, since sync state is advisory.
		return &State{}, nil
	}
	return s, nil
}

// Save writes sync state atomically (tmp + rename) so doctor never reads
// a half-written file.
func Save(path string, s *State) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "sync_state.*.json")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	defer os.Remove(tmp.Name())
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return fmt.Errorf("encoding state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Status returns a short human-readable verdict for doctor.
func (s *State) Status() string {
	if s.LastAttemptedAt.IsZero() {
		return "never synced"
	}
	if !s.OK {
		if s.ErrorClass != "" {
			return fmt.Sprintf("last sync failed (%s) at %s", s.ErrorClass, s.LastAttemptedAt.Format(time.RFC3339))
		}
		return fmt.Sprintf("last sync failed at %s", s.LastAttemptedAt.Format(time.RFC3339))
	}
	return fmt.Sprintf("ok (%d runs, %d items) at %s",
		s.RunsHydrated, s.ItemsHydrated, s.LastSyncedAt.Format(time.RFC3339))
}
