// Copyright 2026 The plane-pp-cli authors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/store"
)

// TestModuleIssuesCascadeRegistered guards that the module_issues junction is
// registered for cleanup under BOTH per_parent module resources. archived_modules
// also reconciles per_parent, so without its registration a swept archived module
// would orphan its module_issues rows. The package init() performs the wiring.
func TestModuleIssuesCascadeRegistered(t *testing.T) {
	want := store.CascadeJunction{Table: "module_issues", FKColumn: "module_id"}
	for _, resourceType := range []string{"modules", "archived_modules"} {
		got := store.CascadeJunctionsFor(resourceType)
		found := false
		for _, j := range got {
			if j == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("CascadeJunctionsFor(%q) = %+v, want it to include %+v", resourceType, got, want)
		}
	}
}

// TestModulesForIssue_ResolvesCompositeModuleName is the regression for `module
// of` printing a blank name for every module. modules rows are stored under the
// engine-4.27 NUL-composite key (`<uuid>\x00<projects_id>`) while the
// module_issues junction carries the BARE uuid, so the former
// `LEFT JOIN modules m ON m.id = mi.module_id` matched nothing and the
// COALESCE fallback silently rendered an empty name. The bare-id row also
// covers the nullable project_id fallback, and both rows together pin the
// name-then-id ordering.
func TestModulesForIssue_ResolvesCompositeModuleName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if _, _, err := s.UpsertBatch("projects", []json.RawMessage{
		json.RawMessage(`{"id": "p1", "workspace": "ws-a"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch projects: %v", err)
	}
	if _, _, err := s.UpsertBatch("modules", []json.RawMessage{
		json.RawMessage(`{"id": "m1", "projects_id": "p1", "name": "Sprint 1"}`),
		json.RawMessage(`{"id": "m2", "projects_id": "p1", "name": "Alpha"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch modules: %v", err)
	}

	// Guard the premise: if modules ever stop being parent-keyed, this test
	// would pass for the wrong reason.
	var storedID string
	if err := s.DB().QueryRow(`SELECT id FROM modules WHERE id LIKE 'm1%'`).Scan(&storedID); err != nil {
		t.Fatalf("read stored module id: %v", err)
	}
	if !strings.ContainsRune(storedID, 0) {
		t.Fatalf("stored module id = %q, want a NUL-composite key; the premise of this regression no longer holds", storedID)
	}

	if err := ensureModuleIssuesTable(s); err != nil {
		t.Fatalf("ensureModuleIssuesTable: %v", err)
	}
	// The junction stores bare module ids; project_id is nullable, so cover
	// both the parent-qualified lookup and the bare-id fallback.
	if _, err := s.DB().Exec(
		`INSERT INTO module_issues (module_id, issue_id, project_id) VALUES (?, ?, ?), (?, ?, NULL)`,
		"m1", "issue-1", "p1", "m2", "issue-1",
	); err != nil {
		t.Fatalf("seed module_issues: %v", err)
	}

	got, err := modulesForIssue(s, "issue-1")
	if err != nil {
		t.Fatalf("modulesForIssue: %v", err)
	}
	want := []modRef{{ID: "m2", Name: "Alpha"}, {ID: "m1", Name: "Sprint 1"}}
	if len(got) != len(want) {
		t.Fatalf("modulesForIssue = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("modulesForIssue[%d] = %+v, want %+v (a blank name means the composite id was not resolved)", i, got[i], want[i])
		}
	}
}

// TestEnrichModuleMembership_PatchesCompositeStoredID is the regression for the
// engine-4.27 NUL-composite storage id. Dependent projects_issues rows are stored
// under `<uuid>\x00<parent>`, while the module-issues API returns bare uuids. The
// per-issue module_ids patch must map the bare API id back to the stored composite
// key before the UPDATE, otherwise `WHERE id = <bare>` matches zero rows and every
// issue is left un-enriched. Live A/B against the 4.27 binary caught this as a
// 95 -> 0 "issues patched" regression while the module_issues junction stayed full.
func TestEnrichModuleMembership_PatchesCompositeStoredID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if _, _, err := s.UpsertBatch("projects", []json.RawMessage{
		json.RawMessage(`{"id": "p1", "workspace": "ws-a"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch projects: %v", err)
	}
	if _, _, err := s.UpsertBatch("modules", []json.RawMessage{
		json.RawMessage(`{"id": "m1", "projects_id": "p1", "name": "M1"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch modules: %v", err)
	}

	// Store the issue under the engine-4.27 NUL-composite storage id (<uuid>\x00<parent>),
	// exactly as the reprinted dependent sync now writes projects_issues rows.
	compositeID := "issue-1\x00p1"
	if _, err := s.DB().Exec(
		`INSERT INTO projects_issues (id, projects_id, data) VALUES (?, ?, ?)`,
		compositeID, "p1", `{"id":"issue-1","name":"Issue 1"}`,
	); err != nil {
		t.Fatalf("seed issue: %v", err)
	}

	// The module-issues endpoint returns the BARE issue id.
	getter := fakeModuleGetter{resp: json.RawMessage(`[{"id":"issue-1"}]`)}

	res, err := enrichModuleMembership(context.Background(), getter, s, "ws-a", "", "")
	if err != nil {
		t.Fatalf("enrichModuleMembership: %v", err)
	}
	if res.Patched != 1 {
		t.Fatalf("res.Patched = %d, want 1 (a composite-stored issue must still be patched)", res.Patched)
	}

	var data string
	if err := s.DB().QueryRow(`SELECT data FROM projects_issues WHERE id = ?`, compositeID).Scan(&data); err != nil {
		t.Fatalf("read back: %v", err)
	}
	var obj struct {
		ModuleIDs []string `json:"module_ids"`
	}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		t.Fatalf("unmarshal patched data: %v", err)
	}
	if len(obj.ModuleIDs) != 1 || obj.ModuleIDs[0] != "m1" {
		t.Fatalf("module_ids = %v, want [m1]", obj.ModuleIDs)
	}
}
