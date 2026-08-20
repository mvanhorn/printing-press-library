// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func shellNode(t *testing.T, id, name string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"id": id, "name": name})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// A shell resource lives in two places: its typed table and the generic
// resources cache the promoted local reads answer from. Reconciling only the
// typed table leaves the cache serving an entity that is gone upstream, and
// reconciling them in two transactions leaves the same gap whenever the second
// one fails, so one pass covers both.
func TestPruneMissingWithMirrorRemovesBothCopies(t *testing.T) {
	t.Parallel()
	db, err := Open(filepath.Join(t.TempDir(), "linear.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	live := shellNode(t, "init-live", "Still there")
	dead := shellNode(t, "init-dead", "Deleted upstream")
	for _, node := range []json.RawMessage{live, dead} {
		var n struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(node, &n); err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertShellRow("initiatives", n.ID, node); err != nil {
			t.Fatalf("UpsertShellRow: %v", err)
		}
	}
	if err := db.UpsertBatch("initiatives", []json.RawMessage{live, dead}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	// A resource type sync does not enumerate must survive: this row proves
	// the delete is scoped by resource_type.
	if err := db.UpsertBatch("documents", []json.RawMessage{shellNode(t, "doc-1", "Untouched")}); err != nil {
		t.Fatalf("UpsertBatch documents: %v", err)
	}

	liveIDs := []string{"init-live"}

	// One stale typed row plus one stale mirrored row.
	stale, err := db.CountMissingWithMirror("initiatives", "initiatives", liveIDs)
	if err != nil {
		t.Fatalf("CountMissingWithMirror: %v", err)
	}
	if stale != 2 {
		t.Fatalf("CountMissingWithMirror = %d, want 2", stale)
	}
	if got := countResources(t, db, "initiatives"); got != 2 {
		t.Fatalf("counting is not deleting: %d cached initiatives, want 2", got)
	}

	removed, err := db.PruneMissingWithMirror("initiatives", "initiatives", liveIDs)
	if err != nil {
		t.Fatalf("PruneMissingWithMirror: %v", err)
	}
	if removed != 2 {
		t.Fatalf("PruneMissingWithMirror = %d, want 2", removed)
	}

	if got := countTyped(t, db, "initiatives"); got != 1 {
		t.Fatalf("typed initiatives = %d, want 1", got)
	}
	if got := countResources(t, db, "initiatives"); got != 1 {
		t.Fatalf("cached initiatives = %d, want 1", got)
	}
	if got := countResources(t, db, "documents"); got != 1 {
		t.Fatalf("prune crossed resource types: cached documents = %d, want 1", got)
	}
	if got, err := db.Get("initiatives", "init-dead"); err != nil || got != nil {
		t.Fatalf("deleted initiative still readable: %s (err %v)", got, err)
	}

	// resources has no delete trigger on its FTS index, so a stale index row
	// would keep the deleted entity answering local search.
	var ftsRows int
	if err := db.DB().QueryRow(`SELECT count(*) FROM resources_fts WHERE id = ?`, "init-dead").Scan(&ftsRows); err != nil {
		t.Fatalf("counting fts rows: %v", err)
	}
	if ftsRows != 0 {
		t.Fatalf("resources_fts kept %d row(s) for the pruned initiative", ftsRows)
	}
}

// A refused pass leaves both copies untouched: the guardrails cannot fire
// after the typed table has already been emptied.
func TestPruneMissingWithMirrorRefusesUnsafeInput(t *testing.T) {
	t.Parallel()
	db, err := Open(filepath.Join(t.TempDir(), "linear.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	node := shellNode(t, "init-1", "Present")
	if err := db.UpsertShellRow("initiatives", "init-1", node); err != nil {
		t.Fatalf("UpsertShellRow: %v", err)
	}
	if err := db.UpsertBatch("initiatives", []json.RawMessage{node}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	if _, err := db.PruneMissingWithMirror("initiatives", "initiatives", nil); err == nil {
		t.Fatal("pruning against an empty live set must fail")
	}
	if _, err := db.PruneMissingWithMirror("initiatives", "initiatives", []string{""}); err == nil {
		t.Fatal("pruning against a live set of blank ids must fail")
	}
	if _, err := db.PruneMissingWithMirror("initiatives", "issues", []string{"init-1"}); err == nil {
		t.Fatal("a resource type sync does not mirror must fail")
	}
	if _, err := db.PruneMissingWithMirror("pp_created", "", []string{"init-1"}); err == nil {
		t.Fatal("a table outside the prunable allowlist must fail")
	}

	if got := countTyped(t, db, "initiatives"); got != 1 {
		t.Fatalf("a refused pass deleted typed rows: %d left, want 1", got)
	}
	if got := countResources(t, db, "initiatives"); got != 1 {
		t.Fatalf("a refused pass deleted cached rows: %d left, want 1", got)
	}

	if MirroredResourceType("issues") {
		t.Fatal("issues is a typed table, not a mirrored resource type")
	}
	if !MirroredResourceType("project-milestones") {
		t.Fatal("project-milestones is mirrored by the shell resource sync")
	}
}

func countResources(t *testing.T, db *Store, resourceType string) int {
	t.Helper()
	var n int
	if err := db.DB().QueryRow(`SELECT count(*) FROM resources WHERE resource_type = ?`, resourceType).Scan(&n); err != nil {
		t.Fatalf("counting %s resources: %v", resourceType, err)
	}
	return n
}

func countTyped(t *testing.T, db *Store, table string) int {
	t.Helper()
	var n int
	if err := db.DB().QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("counting %s rows: %v", table, err)
	}
	return n
}
