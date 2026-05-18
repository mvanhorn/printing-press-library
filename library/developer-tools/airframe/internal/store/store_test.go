// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestOpenAndMigrate verifies that a fresh database is created, migrations run
// cleanly, the schema version is stamped, and every expected table and index
// exists. Anchored against StoreSchemaVersion so a schema change without a
// version bump fails this test.
func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != StoreSchemaVersion {
		t.Fatalf("schema version = %d, want %d", v, StoreSchemaVersion)
	}

	wantTables := []string{
		"aircraft", "make_model", "engine", "dereg",
		"events", "event_aircraft", "narratives", "sync_meta",
	}
	got := listTables(t, s)
	for _, name := range wantTables {
		if !contains(got, name) {
			t.Errorf("table %q missing; have %v", name, got)
		}
	}
}

// TestReopenIsIdempotent verifies that opening an already-migrated DB does
// not error and the schema version remains correct.
func TestReopenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s.Close()

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	v, err := s2.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion after reopen: %v", err)
	}
	if v != StoreSchemaVersion {
		t.Fatalf("schema version after reopen = %d, want %d", v, StoreSchemaVersion)
	}
}

// TestListSyncMetaEmpty confirms the ListSyncMeta query works against a fresh
// (empty) sync_meta table and returns no rows.
func TestListSyncMetaEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	rows, err := s.ListSyncMeta(context.Background())
	if err != nil {
		t.Fatalf("ListSyncMeta: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

// TestOpenReadOnlyRejectsNewerSchema verifies that OpenReadOnly returns the
// upgrade-the-CLI error when the database's user_version exceeds
// StoreSchemaVersion, instead of silently returning a Store that would emit
// raw SQLite errors on every query.
func TestOpenReadOnlyRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatalf("bump user_version: %v", err)
	}
	db.Close()

	if _, err := OpenReadOnly(dbPath); err == nil {
		t.Fatal("OpenReadOnly succeeded against a newer-schema DB; want version error")
	} else if !strings.Contains(err.Error(), "upgrade the CLI binary") {
		t.Fatalf("OpenReadOnly error = %v; want substring 'upgrade the CLI binary'", err)
	}
}

// TestOpenReadOnlySameVersionSucceeds verifies the happy path: a fresh
// same-version DB still opens read-only without error.
func TestOpenReadOnlySameVersionSucceeds(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Close()

	s2, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly on same-version DB: %v", err)
	}
	defer s2.Close()

	v, err := s2.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != StoreSchemaVersion {
		t.Fatalf("schema version = %d, want %d", v, StoreSchemaVersion)
	}
}

func listTables(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.DB().Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatalf("listing tables: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
