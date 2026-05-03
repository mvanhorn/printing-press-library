package openmeteo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotDir returns the directory where novel commands persist their
// own per-place snapshots (forecast diff baselines, accuracy back-tests).
// Distinct from the generated client's HTTP response cache so that
// `--no-cache` does not wipe forecast-diff history.
func SnapshotDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "open-meteo-pp-cli", "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// snapshotKey returns a filesystem-safe key for (kind, place, lat, lon).
// kind distinguishes "forecast", "marine", etc. so multiple endpoint
// snapshots for the same place can coexist.
func snapshotKey(kind string, p Place) string {
	name := strings.ReplaceAll(strings.ToLower(p.Name), " ", "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, name)
	if name == "" {
		name = fmt.Sprintf("%.2f-%.2f", p.Latitude, p.Longitude)
	}
	return fmt.Sprintf("%s-%s.json", kind, name)
}

// Snapshot persists raw JSON for a (kind, place) pair. Callers pass the
// data as the unmarshaled response body so consumers can store the
// payload verbatim and re-load it later for diffing.
type Snapshot struct {
	Kind      string          `json:"kind"`
	Place     Place           `json:"place"`
	StoredAt  time.Time       `json:"stored_at"`
	Payload   json.RawMessage `json:"payload"`
	ParamsKey string          `json:"params_key,omitempty"`
}

// SaveSnapshot writes a snapshot to disk, overwriting any existing snapshot for
// the same kind+place tuple.
func SaveSnapshot(kind string, p Place, paramsKey string, payload json.RawMessage) error {
	dir, err := SnapshotDir()
	if err != nil {
		return err
	}
	snap := Snapshot{
		Kind:      kind,
		Place:     p,
		StoredAt:  time.Now().UTC(),
		Payload:   payload,
		ParamsKey: paramsKey,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, snapshotKey(kind, p)), data, 0o644)
}

// LoadSnapshot reads the most-recently-saved snapshot for a kind+place pair.
// Returns os.ErrNotExist semantics when no snapshot has ever been saved.
func LoadSnapshot(kind string, p Place) (*Snapshot, error) {
	dir, err := SnapshotDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, snapshotKey(kind, p)))
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}
