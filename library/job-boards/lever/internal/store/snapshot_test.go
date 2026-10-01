// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestUpsertAuthoritativeSnapshotRemovesMissingRows(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	initial := []json.RawMessage{
		json.RawMessage(`{"id":"open-1","text":"one"}`),
		json.RawMessage(`{"id":"open-2","text":"two"}`),
		json.RawMessage(`{"id":"open-3","text":"three"}`),
	}
	if _, _, err := s.UpsertBatch("postings:acme", initial); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stored, err := s.UpsertAuthoritativeSnapshot("postings:acme", []json.RawMessage{
		json.RawMessage(`{"id":"open-1","text":"updated"}`),
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if stored != 1 {
		t.Fatalf("stored = %d, want 1", stored)
	}

	rows, err := s.List("postings:acme", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || string(rows[0]) != `{"id":"open-1","text":"updated"}` {
		t.Fatalf("rows = %s, want only updated open-1", rows)
	}
	var indexed int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM resources_fts WHERE resource_type = ?", "postings:acme").Scan(&indexed); err != nil {
		t.Fatalf("count search entries: %v", err)
	}
	if indexed != 1 {
		t.Fatalf("search entries = %d, want only the current posting", indexed)
	}
}

func TestUpsertAuthoritativeSnapshotValidationPreservesPreviousRows(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if _, _, err := s.UpsertBatch("postings:acme", []json.RawMessage{
		json.RawMessage(`{"id":"open-1"}`),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.UpsertAuthoritativeSnapshot("postings:acme", []json.RawMessage{
		json.RawMessage(`{"text":"missing id"}`),
	}); err == nil {
		t.Fatal("replace unexpectedly accepted an unkeyed item")
	}

	rows, err := s.List("postings:acme", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || string(rows[0]) != `{"id":"open-1"}` {
		t.Fatalf("rows = %s, want previous snapshot preserved", rows)
	}
}
