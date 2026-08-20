// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
)

// TestSyncBrowseIndex_PaginatesAndStoresWithIndexesID is the regression test
// for the bug objects_gaps/objects_diff were built on: browse must actually
// populate the typed browse table (indexes_id set), across multiple pages,
// or the audit commands silently see nothing.
func TestSyncBrowseIndex_PaginatesAndStoresWithIndexesID(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/1/indexes/my_index/browse" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests = append(requests, body)

		w.Header().Set("Content-Type", "application/json")
		if body["cursor"] == nil {
			// First page: two hits, more to come.
			_, _ = w.Write([]byte(`{"hits":[{"objectID":"obj1","title":"one"},{"objectID":"obj2","title":"two"}],"cursor":"page2"}`))
			return
		}
		// Second (final) page: one hit, no cursor.
		_, _ = w.Write([]byte(`{"hits":[{"objectID":"obj3","title":"three"}]}`))
	}))
	t.Cleanup(server.Close)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true

	stored, capHit, err := syncBrowseIndex(context.Background(), c, db, "my_index", 0)
	if err != nil {
		t.Fatalf("syncBrowseIndex: %v", err)
	}
	if capHit {
		t.Fatalf("capHit = true, want false")
	}
	if stored != 3 {
		t.Fatalf("stored = %d, want 3", stored)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2 (paginated)", len(requests))
	}
	if requests[1]["cursor"] != "page2" {
		t.Fatalf("second request cursor = %v, want %q", requests[1]["cursor"], "page2")
	}

	rows, err := db.DB().QueryContext(context.Background(), `SELECT id, indexes_id, data FROM browse ORDER BY id`)
	if err != nil {
		t.Fatalf("query browse: %v", err)
	}
	defer rows.Close()

	var gotIDs []string
	for rows.Next() {
		var id, indexesID, data string
		if err := rows.Scan(&id, &indexesID, &data); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// browse is a per-index dependent resource, so the store composites
		// the primary key as "<objectID>\x00<indexName>" to avoid collisions
		// across indices (see resourceParentKeyColumns["browse"]).
		bareID := store.BareResourceID(id)
		gotIDs = append(gotIDs, bareID)
		if indexesID != "my_index" {
			t.Fatalf("row %q: indexes_id = %q, want %q", id, indexesID, "my_index")
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err != nil {
			t.Fatalf("unmarshal stored data: %v", err)
		}
		if obj["objectID"] != bareID {
			t.Fatalf("row %q: stored objectID = %v, want %q", id, obj["objectID"], bareID)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	want := []string{"obj1", "obj2", "obj3"}
	if len(gotIDs) != len(want) {
		t.Fatalf("stored ids = %v, want %v", gotIDs, want)
	}
	for i, id := range want {
		if gotIDs[i] != id {
			t.Fatalf("stored ids = %v, want %v", gotIDs, want)
		}
	}

	// This is the exact query objects_gaps.go and objects_diff.go run —
	// confirm it actually finds the rows this sync just stored.
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM browse WHERE indexes_id = ?`, "my_index").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 3 {
		t.Fatalf("objects_gaps-style query found %d rows, want 3", count)
	}
}

// TestSyncBrowseIndex_FullReplace confirms a re-run wholesale replaces the
// index's prior rows rather than accumulating duplicates or leaving stale
// records search can no longer return.
func TestSyncBrowseIndex_FullReplace(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"hits":[{"objectID":"stale1"},{"objectID":"stale2"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"hits":[{"objectID":"fresh1"}]}`))
	}))
	t.Cleanup(server.Close)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true

	if _, _, err := syncBrowseIndex(context.Background(), c, db, "my_index", 0); err != nil {
		t.Fatalf("first syncBrowseIndex: %v", err)
	}
	if _, _, err := syncBrowseIndex(context.Background(), c, db, "my_index", 0); err != nil {
		t.Fatalf("second syncBrowseIndex: %v", err)
	}

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM browse WHERE indexes_id = ?`, "my_index").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("browse rows after second sync = %d, want 1 (stale rows should be replaced)", count)
	}
	var id string
	if err := db.DB().QueryRow(`SELECT id FROM browse WHERE indexes_id = ?`, "my_index").Scan(&id); err != nil {
		t.Fatalf("scan surviving row: %v", err)
	}
	if bareID := store.BareResourceID(id); bareID != "fresh1" {
		t.Fatalf("surviving row id = %q, want %q", bareID, "fresh1")
	}
}

// TestSyncBrowseIndex_MaxPagesCap confirms --max-pages actually bounds a
// pathological or very large index rather than looping until exhaustion.
func TestSyncBrowseIndex_MaxPagesCap(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[{"objectID":"obj` + strconv.Itoa(page) + `"}],"cursor":"next` + strconv.Itoa(page) + `"}`))
	}))
	t.Cleanup(server.Close)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true

	stored, capHit, err := syncBrowseIndex(context.Background(), c, db, "my_index", 2)
	if err != nil {
		t.Fatalf("syncBrowseIndex: %v", err)
	}
	if !capHit {
		t.Fatalf("capHit = false, want true")
	}
	if stored != 2 {
		t.Fatalf("stored = %d, want 2 (capped at max-pages=2)", stored)
	}
}

// TestBrowseSyncIndexNames_ReadsFromIndexesTable confirms the dependent
// fan-out only enumerates indices "indexes" itself has already synced.
func TestBrowseSyncIndexNames_ReadsFromIndexesTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	names, err := browseSyncIndexNames(context.Background(), db)
	if err != nil {
		t.Fatalf("browseSyncIndexNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty before any index is synced", names)
	}

	if err := db.UpsertIndexes(json.RawMessage(`{"name":"idx_a"}`)); err != nil {
		t.Fatalf("UpsertIndexes: %v", err)
	}
	if err := db.UpsertIndexes(json.RawMessage(`{"name":"idx_b"}`)); err != nil {
		t.Fatalf("UpsertIndexes: %v", err)
	}

	names, err = browseSyncIndexNames(context.Background(), db)
	if err != nil {
		t.Fatalf("browseSyncIndexNames: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("names = %v, want 2 entries", names)
	}
}
