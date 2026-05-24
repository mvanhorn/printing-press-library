// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestListIDs_HappyPath_DomainTable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Insert items into a domain table via UpsertBatch
	items := []json.RawMessage{
		json.RawMessage(`{"id": "v-001", "domains_id": "dom-001"}`),
		json.RawMessage(`{"id": "v-002", "domains_id": "dom-002"}`),
	}
	if _, _, err := s.UpsertBatch("verify", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	ids, err := s.ListIDs("verify")
	if err != nil {
		t.Fatalf("ListIDs(verify): %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["v-001"] || !idSet["v-002"] {
		t.Fatalf("expected IDs v-001 and v-002, got %v", ids)
	}
}

func TestListIDs_HappyPath_ResourcesTable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Insert directly into the generic resources table
	_, err = s.db.Exec(
		`INSERT INTO resources (id, resource_type, data, synced_at) VALUES (?, ?, ?, datetime('now'))`,
		"res-001", "resources", `{"id":"res-001"}`,
	)
	if err != nil {
		t.Fatalf("insert into resources: %v", err)
	}

	ids, err := s.ListIDs("resources")
	if err != nil {
		t.Fatalf("ListIDs(resources): %v", err)
	}
	if len(ids) != 1 || ids[0] != "res-001" {
		t.Fatalf("expected [res-001], got %v", ids)
	}
}

func TestListIDs_SQLInjection_Rejected(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Insert a row into verify so we can tell if injection worked
	items := []json.RawMessage{
		json.RawMessage(`{"id": "v-001", "domains_id": "dom-001"}`),
	}
	if _, _, err := s.UpsertBatch("verify", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	// Attempt SQL injection — should NOT execute, should fall through to
	// generic resources query (which returns 0 rows for this type)
	ids, err := s.ListIDs("verify; DROP TABLE verify")
	if err != nil {
		t.Fatalf("ListIDs with injection payload should not error, got: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 IDs from injection payload, got %d", len(ids))
	}

	// Verify the verify table still exists and has data
	idsAfter, err := s.ListIDs("verify")
	if err != nil {
		t.Fatalf("ListIDs(verify) after injection attempt: %v", err)
	}
	if len(idsAfter) != 1 {
		t.Fatalf("verify table should still have 1 row, got %d", len(idsAfter))
	}
}

func TestListIDs_NonexistentTable_FallsThrough(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// A table that doesn't exist should fall through to generic resources query
	ids, err := s.ListIDs("nonexistent_table")
	if err != nil {
		t.Fatalf("ListIDs(nonexistent_table) should not error, got: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 IDs for nonexistent table, got %d", len(ids))
	}
}
