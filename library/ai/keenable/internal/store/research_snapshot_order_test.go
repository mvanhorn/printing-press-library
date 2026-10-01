package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestListUsesInsertionOrderWhenTimestampsMatch(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "research.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, id := range []string{"z-oldest", "m-middle", "a-newest"} {
		data, err := json.Marshal(map[string]string{"id": id})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UpsertMany([]ResourceWrite{{ResourceType: "research_snapshots", ID: id, Data: data}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`UPDATE resources SET updated_at = '2026-10-01T00:00:00Z' WHERE resource_type = 'research_snapshots'`); err != nil {
		t.Fatal(err)
	}

	items, err := s.List("research_snapshots", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(items))
	}
	for i, want := range []string{"a-newest", "m-middle", "z-oldest"} {
		var snap struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(items[i], &snap); err != nil {
			t.Fatal(err)
		}
		if snap.ID != want {
			t.Fatalf("snapshot %d = %q, want %q", i, snap.ID, want)
		}
	}
}
