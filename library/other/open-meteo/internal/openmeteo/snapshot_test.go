package openmeteo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotKey(t *testing.T) {
	cases := []struct {
		name string
		p    Place
		want string
	}{
		{"basic", Place{Name: "Seattle"}, "forecast-seattle.json"},
		{"with whitespace", Place{Name: "New York"}, "forecast-new-york.json"},
		{"with punctuation", Place{Name: "Mavericks, CA"}, "forecast-mavericks-ca.json"},
		{"empty falls back to coords", Place{Latitude: 47.6, Longitude: -122.3}, "forecast-47.60--122.30.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := snapshotKey("forecast", c.p)
			if got != c.want {
				t.Errorf("snapshotKey(%+v) = %q, want %q", c.p, got, c.want)
			}
		})
	}
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	// Override snapshot dir to a tempdir so the test never touches ~.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	p := Place{Name: "Testville", Latitude: 1, Longitude: 2}
	payload := json.RawMessage(`{"hello":"world","temperature_2m":21.5}`)
	if err := SaveSnapshot("forecast", p, "key1", payload); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	got, err := LoadSnapshot("forecast", p)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if got.Kind != "forecast" {
		t.Errorf("Kind = %q, want forecast", got.Kind)
	}
	if got.Place.Name != "Testville" {
		t.Errorf("Place.Name = %q", got.Place.Name)
	}
	if got.ParamsKey != "key1" {
		t.Errorf("ParamsKey = %q", got.ParamsKey)
	}
	// Snapshot stores the payload verbatim, but the wrapping struct is
	// re-encoded with indentation, so the json.RawMessage round-trips
	// through formatting. Compare parsed values rather than bytes.
	var gotMap, wantMap map[string]any
	_ = json.Unmarshal(got.Payload, &gotMap)
	_ = json.Unmarshal(payload, &wantMap)
	if gotMap["hello"] != wantMap["hello"] || gotMap["temperature_2m"] != wantMap["temperature_2m"] {
		t.Errorf("Payload mismatch after round-trip: got %+v, want %+v", gotMap, wantMap)
	}
	// Confirm the file actually lives under the snapshot dir.
	dir, err := SnapshotDir()
	if err != nil {
		t.Fatalf("SnapshotDir: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Errorf("no entries in snapshot dir %q", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, snapshotKey("forecast", p))); err != nil {
		t.Errorf("snapshot file missing: %v", err)
	}
}

func TestLoadSnapshotMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if _, err := LoadSnapshot("forecast", Place{Name: "Nowhere"}); err == nil {
		t.Error("expected error loading missing snapshot")
	}
}
