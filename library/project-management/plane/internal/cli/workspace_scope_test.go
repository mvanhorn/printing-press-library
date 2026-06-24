// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/config"
	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/store"

	_ "modernc.org/sqlite"
)

func TestResolveActiveWorkspaceID(t *testing.T) {
	registry := []config.WorkspaceEntry{
		{Slug: "bbm", ID: "ws-bbm-uuid"},
		{Slug: "doctor-school", ID: "ws-ds-uuid"},
		{Slug: "no-id"}, // enrolled without a probed UUID
	}
	cases := []struct {
		name string
		slug string
		want string
	}{
		{"resolved", "bbm", "ws-bbm-uuid"},
		{"resolved-other", "doctor-school", "ws-ds-uuid"},
		{"enrolled-without-id", "no-id", ""},
		{"not-enrolled", "ghost", ""},
		{"sentinel", "my-workspace", ""},
		{"empty", "", ""},
		{"trims-whitespace", "  bbm  ", "ws-bbm-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveActiveWorkspaceID(tc.slug, registry); got != tc.want {
				t.Fatalf("resolveActiveWorkspaceID(%q) = %q, want %q", tc.slug, got, tc.want)
			}
		})
	}
}

// TestDependentParentRows_ScopedToWorkspace is the end-to-end regression for the
// cross-workspace 403 bug: with two workspaces' projects in one local store, the
// dependent-sync parent enumeration must return only the active workspace's
// project IDs, so the fan-out never dials a foreign project under the active slug.
func TestDependentParentRows_ScopedToWorkspace(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	items := []json.RawMessage{
		json.RawMessage(`{"id": "p-bbm-1", "workspace": "ws-bbm"}`),
		json.RawMessage(`{"id": "p-bbm-2", "workspace": "ws-bbm"}`),
		json.RawMessage(`{"id": "p-ds-1", "workspace": "ws-doctor-school"}`),
	}
	if _, _, err := s.UpsertBatch("projects", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	pathParams := []dependentPathParamDef{{Param: "project_id", Field: "id"}}

	// Scoped to bbm: only bbm projects.
	rows, err := dependentParentRows(s, "projects", pathParams, "workspace", "ws-bbm")
	if err != nil {
		t.Fatalf("dependentParentRows scoped: %v", err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r["id"])
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "p-bbm-1" || got[1] != "p-bbm-2" {
		t.Fatalf("scoped parent ids = %v, want [p-bbm-1 p-bbm-2]", got)
	}

	// Empty scope (unknown UUID): backward-compatible unscoped enumeration.
	all, err := dependentParentRows(s, "projects", pathParams, "workspace", "")
	if err != nil {
		t.Fatalf("dependentParentRows unscoped: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("unscoped parent rows = %d, want 3", len(all))
	}
}

// TestLocalModules_ScopedToWorkspace covers the second cross-workspace surface:
// the post-sync module-membership enrichment walked every local module, so it
// 403'd on (and aborted at) other workspaces' modules. localModules must scope
// to the active workspace's projects, still honor an explicit project filter,
// and fall back to all modules when no scope is supplied.
func TestLocalModules_ScopedToWorkspace(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	projects := []json.RawMessage{
		json.RawMessage(`{"id": "p-bbm", "workspace": "ws-bbm"}`),
		json.RawMessage(`{"id": "p-ds", "workspace": "ws-doctor-school"}`),
	}
	if _, _, err := s.UpsertBatch("projects", projects); err != nil {
		t.Fatalf("UpsertBatch projects: %v", err)
	}
	modules := []json.RawMessage{
		json.RawMessage(`{"id": "m-bbm", "projects_id": "p-bbm", "name": "BBM mod"}`),
		json.RawMessage(`{"id": "m-ds", "projects_id": "p-ds", "name": "DS mod"}`),
	}
	if _, _, err := s.UpsertBatch("modules", modules); err != nil {
		t.Fatalf("UpsertBatch modules: %v", err)
	}

	// Scoped to bbm: only bbm's module.
	scoped, err := localModules(s, "", "ws-bbm")
	if err != nil {
		t.Fatalf("localModules scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0].id != "m-bbm" {
		t.Fatalf("scoped modules = %+v, want only m-bbm", scoped)
	}

	// Unscoped fallback: both.
	all, err := localModules(s, "", "")
	if err != nil {
		t.Fatalf("localModules unscoped: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("unscoped modules = %d, want 2", len(all))
	}

	// Explicit project filter still wins.
	one, err := localModules(s, "p-ds", "")
	if err != nil {
		t.Fatalf("localModules project filter: %v", err)
	}
	if len(one) != 1 || one[0].id != "m-ds" {
		t.Fatalf("project-filtered modules = %+v, want only m-ds", one)
	}
}
