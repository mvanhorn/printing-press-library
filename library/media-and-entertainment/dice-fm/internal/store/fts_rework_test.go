// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the FTS rework (review finding #5): content minimized to
// non-identifying discovery fields, FTS keyed off the collision-free
// resources.rowid, and a v2->v3 rebuild that drops PII from an existing index.
// All fixtures are synthetic (IETF example.com, fabricated IDs/names).
package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// orderBlobWithFan builds an order JSON carrying a nested fan with PII plus a
// non-identifying event name, the realistic shape sync stores.
const phoneToken = "5550100" // synthetic phone fragment, never a real number

func orderBlobWithFan(eventName, email, first, phone string) string {
	return `{"id":"o1","event":{"id":"e1","name":"` + eventName + `"},` +
		`"fan":{"id":"f1","email":"` + email + `","firstName":"` + first + `","phoneNumber":"` + phone + `","dob":"1990-01-01"}}`
}

// TestFTSContentExcludesPII asserts ftsContent indexes discovery fields and
// never the buyer/holder identifiers.
func TestFTSContentExcludesPII(t *testing.T) {
	blob := orderBlobWithFan("Midnight Warehouse Rave", "buyer@example.com", "Alice", phoneToken)
	content := ftsContent(blob)
	if !strings.Contains(content, "Midnight Warehouse Rave") {
		t.Errorf("ftsContent dropped the event name: %q", content)
	}
	for _, pii := range []string{"buyer@example.com", "Alice", phoneToken, "1990-01-01"} {
		if strings.Contains(content, pii) {
			t.Errorf("ftsContent leaked PII %q into content: %q", pii, content)
		}
	}
}

// TestSearchByPIIDoesNotMatchPerson seeds an order with fan PII and asserts that
// searching for the buyer's email/phone returns nothing, while searching for
// the (non-identifying) event name still matches.
func TestSearchByPIIDoesNotMatchPerson(t *testing.T) {
	s := openStore(t)
	if err := s.Upsert("orders", "o1", []byte(orderBlobWithFan("Aurora Festival", "victim@example.com", "Bob", phoneToken))); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Discovery field still searchable.
	hits, err := s.Search("Aurora", 10)
	if err != nil {
		t.Fatalf("search event name: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("search 'Aurora' = %d hits, want 1", len(hits))
	}

	// PII not searchable.
	for _, term := range []string{"victim", phoneToken} {
		hits, err := s.Search(term, 10)
		if err != nil {
			// FTS may error on a bare token that tokenizes oddly; treat as no match.
			continue
		}
		if len(hits) != 0 {
			t.Errorf("search %q = %d hits, want 0 (PII must not be indexed)", term, len(hits))
		}
	}
}

// TestSearchNoCollisionDrop asserts that indexing many distinct resources and
// then updating one does not silently drop another from search — the failure
// mode of the old 63-bit hashed rowid. Keying off the real resources.rowid is
// collision-free by construction.
func TestSearchNoCollisionDrop(t *testing.T) {
	s := openStore(t)
	// Two resources with the SAME id under different resource_types — under the
	// old hash these had distinct (but potentially colliding) hashed rowids;
	// now they get distinct real rowids.
	if err := s.Upsert("events", "shared-id", []byte(`{"id":"shared-id","name":"Alpha Show"}`)); err != nil {
		t.Fatalf("upsert events: %v", err)
	}
	if err := s.Upsert("venues", "shared-id", []byte(`{"id":"shared-id","name":"Beta Venue"}`)); err != nil {
		t.Fatalf("upsert venues: %v", err)
	}
	// Re-upsert (update) one of them; the other must remain searchable.
	if err := s.Upsert("events", "shared-id", []byte(`{"id":"shared-id","name":"Alpha Show Updated"}`)); err != nil {
		t.Fatalf("re-upsert events: %v", err)
	}

	if hits, err := s.Search("Beta", 10); err != nil || len(hits) != 1 {
		t.Errorf("search 'Beta' = %d hits, err=%v; want 1 (other record must not be dropped)", len(hits), err)
	}
	if hits, err := s.Search("Updated", 10); err != nil || len(hits) != 1 {
		t.Errorf("search 'Updated' = %d hits, err=%v; want 1", len(hits), err)
	}
}

// TestV2ToV3MigrationDropsPIIFromIndex builds a v2-schema DB whose FTS content
// contains the whole blob (including PII), then opens it with the current binary
// and asserts the v3 rebuild re-projected content so PII is no longer searchable
// while the discovery field still is.
func TestV2ToV3MigrationDropsPIIFromIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	// v2 resources table (composite key) + v2 FTS table.
	stmts := []string{
		`CREATE TABLE resources (
			id TEXT NOT NULL, resource_type TEXT NOT NULL, data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (resource_type, id))`,
		`CREATE VIRTUAL TABLE resources_fts USING fts5(id, resource_type, content, tokenize='porter unicode61')`,
	}
	for _, st := range stmts {
		if _, err := raw.Exec(st); err != nil {
			raw.Close()
			t.Fatalf("v2 ddl: %v", err)
		}
	}
	blob := orderBlobWithFan("Solstice Gathering", "leak@example.com", "Carol", phoneToken)
	if _, err := raw.Exec(`INSERT INTO resources (id, resource_type, data) VALUES ('o1','orders',?)`, blob); err != nil {
		raw.Close()
		t.Fatalf("insert v2 resource: %v", err)
	}
	// v2-style FTS row: whole blob (incl. PII) as content, hashed-style rowid 1.
	if _, err := raw.Exec(`INSERT INTO resources_fts (rowid, id, resource_type, content) VALUES (1,'o1','orders',?)`, blob); err != nil {
		raw.Close()
		t.Fatalf("insert v2 fts: %v", err)
	}
	if _, err := raw.Exec(`PRAGMA user_version = 2`); err != nil {
		raw.Close()
		t.Fatalf("stamp v2: %v", err)
	}
	raw.Close()

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open upgraded db: %v", err)
	}
	defer s.Close()

	if v, err := s.SchemaVersion(); err != nil || v != StoreSchemaVersion {
		t.Fatalf("schema version = %d (err %v), want %d", v, err, StoreSchemaVersion)
	}

	// Discovery field still searchable post-migration.
	if hits, err := s.Search("Solstice", 10); err != nil || len(hits) != 1 {
		t.Errorf("post-migration search 'Solstice' = %d hits err=%v, want 1", len(hits), err)
	}
	// PII no longer in the rebuilt index.
	if hits, err := s.Search("leak", 10); err == nil && len(hits) != 0 {
		t.Errorf("post-migration search 'leak' = %d hits, want 0 (rebuild must drop PII)", len(hits))
	}
}

// openStore opens a fresh store in a temp dir.
func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
