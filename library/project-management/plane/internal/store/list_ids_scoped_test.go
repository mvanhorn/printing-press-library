// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	_ "modernc.org/sqlite"
)

// TestListIDsScoped_FiltersByTenant is the regression for the cross-workspace
// 403 bug: dependent-resource sync enumerates parent project IDs from the local
// store, but the store is multi-tenant (one DB, several workspaces). Before the
// fix, enumeration returned every project regardless of workspace, so the
// dependent phase dialed /workspaces/<active-slug>/projects/<other-workspace-id>/...
// and got 403. ListIDsScoped must return only the active workspace's projects.
func TestListIDsScoped_FiltersByTenant(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Two workspaces share one local DB — exactly the dogfood situation
	// (bbm + doctor-school enrolled against the same data.db).
	items := []json.RawMessage{
		json.RawMessage(`{"id": "p-bbm-1", "workspace": "ws-bbm"}`),
		json.RawMessage(`{"id": "p-bbm-2", "workspace": "ws-bbm"}`),
		json.RawMessage(`{"id": "p-ds-1", "workspace": "ws-doctor-school"}`),
		json.RawMessage(`{"id": "p-ds-2", "workspace": "ws-doctor-school"}`),
		json.RawMessage(`{"id": "p-ds-3", "workspace": "ws-doctor-school"}`),
	}
	if _, _, err := s.UpsertBatch("projects", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	got, err := s.ListIDsScoped("projects", "workspace", "ws-bbm")
	if err != nil {
		t.Fatalf("ListIDsScoped: %v", err)
	}
	sort.Strings(got)
	want := []string{"p-bbm-1", "p-bbm-2"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("scoped ids = %v, want %v (must exclude the other workspace's projects)", got, want)
	}
}

// TestListIDsScoped_EmptyScopeFallsBackToAll documents the backward-compatible
// escape hatch: when the active workspace UUID is unknown (slug not enrolled),
// callers pass an empty scope value and ListIDsScoped behaves like ListIDs.
func TestListIDsScoped_EmptyScopeFallsBackToAll(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	items := []json.RawMessage{
		json.RawMessage(`{"id": "p-1", "workspace": "ws-a"}`),
		json.RawMessage(`{"id": "p-2", "workspace": "ws-b"}`),
	}
	if _, _, err := s.UpsertBatch("projects", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	got, err := s.ListIDsScoped("projects", "workspace", "")
	if err != nil {
		t.Fatalf("ListIDsScoped: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("empty-scope ids = %v, want both rows (unscoped fallback)", got)
	}
}
