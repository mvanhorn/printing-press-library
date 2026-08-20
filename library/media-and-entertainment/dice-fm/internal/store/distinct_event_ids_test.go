// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// TestDistinctEventIDs seeds a few events resources (including a duplicate id)
// and asserts DistinctEventIDs returns the distinct ids sorted.
func TestDistinctEventIDs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	items := []json.RawMessage{
		json.RawMessage(`{"id": "evt-c", "name": "C"}`),
		json.RawMessage(`{"id": "evt-a", "name": "A"}`),
		json.RawMessage(`{"id": "evt-b", "name": "B"}`),
	}
	if _, _, err := s.UpsertBatch("events", items); err != nil {
		t.Fatalf("UpsertBatch events: %v", err)
	}

	// A non-events resource must not contribute.
	if _, _, err := s.UpsertBatch("orders", []json.RawMessage{
		json.RawMessage(`{"id": "ord-1"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch orders: %v", err)
	}

	got, err := s.DistinctEventIDs()
	if err != nil {
		t.Fatalf("DistinctEventIDs: %v", err)
	}
	want := []string{"evt-a", "evt-b", "evt-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DistinctEventIDs() = %v, want %v", got, want)
	}
}

// TestDistinctEventIDs_Empty returns an empty slice (not an error) when no
// events have been synced.
func TestDistinctEventIDs_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	got, err := s.DistinctEventIDs()
	if err != nil {
		t.Fatalf("DistinctEventIDs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DistinctEventIDs() = %v, want empty", got)
	}
}
