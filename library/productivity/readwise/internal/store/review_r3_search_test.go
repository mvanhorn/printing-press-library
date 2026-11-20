package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestR3DocumentSearchTies(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"z", "a", "m"} {
		if _, _, err := s.UpsertBatch("documents", []json.RawMessage{json.RawMessage(`{"id":"` + id + `","title":"same needle"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name   string
		search func(string, int) ([]json.RawMessage, error)
	}{{"all", s.SearchDocuments}, {"topLevel", s.SearchTopLevelDocuments}} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := tc.search("needle", 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("rows=%v", rows)
			}
			for i, id := range []string{"z", "a"} {
				obj, err := DecodeJSONObject(rows[i])
				if err != nil {
					t.Fatal(err)
				}
				if obj["id"] != id {
					t.Fatalf("row %d=%s", i, rows[i])
				}
			}
		})
	}
}
