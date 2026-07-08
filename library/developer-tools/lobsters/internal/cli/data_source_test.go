package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/lobsters/internal/store"
)

func TestResolveLocalListEndpointReturnsSyncedRows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	db, err := store.Open(defaultDBPath("lobsters-pp-cli"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, _, err := db.UpsertBatch("hottest-json", []json.RawMessage{
		json.RawMessage(`{"short_id":"abc123","title":"Cached story"}`),
	}); err != nil {
		t.Fatalf("upsert hottest-json: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	data, _, err := resolveLocal(context.Background(), "hottest-json", true, "/hottest.json", nil, "test")
	if err != nil {
		t.Fatalf("resolveLocal: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("unmarshal local rows: %v", err)
	}
	if len(rows) != 1 || rows[0]["short_id"] != "abc123" {
		t.Fatalf("rows = %#v, want cached story", rows)
	}
}

func TestWriteThroughCacheStoresShortIDStoryDetail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeThroughCache(context.Background(), "s", json.RawMessage(`{"short_id":"story42","title":"Story detail"}`))

	db, err := store.OpenReadOnly(defaultDBPath("lobsters-pp-cli"))
	if err != nil {
		t.Fatalf("open read-only store: %v", err)
	}
	defer db.Close()

	data, err := db.Get("s", "story42")
	if err != nil {
		t.Fatalf("get cached story: %v", err)
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatalf("unmarshal cached story: %v", err)
	}
	if row["short_id"] != "story42" {
		t.Fatalf("row = %#v, want story42", row)
	}
}
