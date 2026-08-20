package opentable

// The RestaurantsAvailability persisted-query hash rotates on OpenTable
// frontend bundle releases. The baked-in RestaurantsAvailabilityHash const is
// only a bootstrap default: once a live scrape captures the current hash (see
// hash_refresh.go), it is persisted here and takes precedence, so the CLI
// self-heals across rotations instead of shipping a dead constant.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var availHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type availHashState struct {
	Hash string `json:"hash"`
}

// availHashPath mirrors cooldownPath: honors $TABLE_RESERVATION_GOAT_CONFIG_DIR
// for parity with auth.SessionPath, else falls back to the per-user cache dir.
func availHashPath() (string, error) {
	if env := os.Getenv("TABLE_RESERVATION_GOAT_CONFIG_DIR"); env != "" {
		return filepath.Join(env, "opentable-avail-hash.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "table-reservation-goat-pp-cli", "opentable-avail-hash.json"), nil
}

// loadPersistedAvailabilityHash returns the persisted hash, or "" when absent,
// unreadable, corrupt, or malformed. A corrupt file is removed so it does not
// keep shadowing the const default on every call. nil-safe on every request.
func loadPersistedAvailabilityHash() string {
	path, err := availHashPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s availHashState
	if err := json.Unmarshal(data, &s); err != nil {
		_ = os.Remove(path)
		return ""
	}
	if !availHashPattern.MatchString(s.Hash) {
		return ""
	}
	return s.Hash
}

// savePersistedAvailabilityHash atomically writes a scraped hash. Rejects any
// value that is not 64 lowercase hex chars so a bad scrape can never poison
// the store (the caller surfaces the rejection per R5).
func savePersistedAvailabilityHash(hash string) error {
	if !availHashPattern.MatchString(hash) {
		return fmt.Errorf("opentable: refusing to persist invalid availability hash %q (want 64 hex chars)", hash)
	}
	path, err := availHashPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating avail-hash directory: %w", err)
	}
	js, err := json.MarshalIndent(availHashState{Hash: hash}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling avail hash: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, js, 0o600); err != nil {
		return fmt.Errorf("writing avail hash: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming avail hash file: %w", err)
	}
	return nil
}

// currentAvailabilityHash resolves the hash the availability path should send:
// the persisted (scraped) value when present, else the bootstrap const.
func currentAvailabilityHash() string {
	if h := loadPersistedAvailabilityHash(); h != "" {
		return h
	}
	return RestaurantsAvailabilityHash
}
