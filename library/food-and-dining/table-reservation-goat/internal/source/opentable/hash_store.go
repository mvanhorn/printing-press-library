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
	"strings"
)

var availHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var gqlOperationPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const restaurantsAvailabilityOperationName = "RestaurantsAvailability"

type availHashState struct {
	Hash          string `json:"hash"`
	OperationName string `json:"operation_name,omitempty"`
}

type availabilityQueryIdentity struct {
	Hash          string
	OperationName string
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
	identity, ok := loadPersistedAvailabilityIdentity()
	if !ok {
		return ""
	}
	return identity.Hash
}

func loadPersistedAvailabilityIdentity() (availabilityQueryIdentity, bool) {
	path, err := availHashPath()
	if err != nil {
		return availabilityQueryIdentity{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return availabilityQueryIdentity{}, false
	}
	var s availHashState
	if err := json.Unmarshal(data, &s); err != nil {
		_ = os.Remove(path)
		return availabilityQueryIdentity{}, false
	}
	if !availHashPattern.MatchString(s.Hash) {
		return availabilityQueryIdentity{}, false
	}
	opName := strings.TrimSpace(s.OperationName)
	if opName == "" {
		opName = restaurantsAvailabilityOperationName
	}
	if !gqlOperationPattern.MatchString(opName) {
		_ = os.Remove(path)
		return availabilityQueryIdentity{}, false
	}
	return availabilityQueryIdentity{Hash: s.Hash, OperationName: opName}, true
}

// savePersistedAvailabilityHash atomically writes a scraped hash. Rejects any
// value that is not 64 lowercase hex chars so a bad scrape can never poison
// the store (the caller surfaces the rejection per R5).
func savePersistedAvailabilityHash(hash string) error {
	return savePersistedAvailabilityIdentity(availabilityQueryIdentity{
		Hash:          hash,
		OperationName: restaurantsAvailabilityOperationName,
	})
}

func savePersistedAvailabilityIdentity(identity availabilityQueryIdentity) error {
	if !availHashPattern.MatchString(identity.Hash) {
		return fmt.Errorf("opentable: refusing to persist invalid availability hash %q (want 64 hex chars)", identity.Hash)
	}
	opName := strings.TrimSpace(identity.OperationName)
	if opName == "" {
		opName = restaurantsAvailabilityOperationName
	}
	if !gqlOperationPattern.MatchString(opName) {
		return fmt.Errorf("opentable: refusing to persist invalid availability operation name %q", identity.OperationName)
	}
	path, err := availHashPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating avail-hash directory: %w", err)
	}
	js, err := json.MarshalIndent(availHashState{Hash: identity.Hash, OperationName: opName}, "", "  ")
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
	return currentAvailabilityIdentity().Hash
}

func currentAvailabilityIdentity() availabilityQueryIdentity {
	if identity, ok := loadPersistedAvailabilityIdentity(); ok {
		return identity
	}
	return availabilityQueryIdentity{
		Hash:          RestaurantsAvailabilityHash,
		OperationName: restaurantsAvailabilityOperationName,
	}
}
