// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
)

// TestObjectsDiff_MatchesSameObjectIDAcrossIndices is the regression test for
// the composite-storage-key bug: browse is a per-index dependent resource,
// so the store composites its primary key as "<objectID>\x00<indexName>" to
// avoid collisions between indices. Before stripping that suffix with
// store.BareResourceID, objects_diff.go used the raw composite key as its
// diff map key, so an identical objectID present in both indices could never
// match — every record would show up as both added and removed instead of
// unchanged.
func TestObjectsDiff_MatchesSameObjectIDAcrossIndices(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Mirrors what mergeIndexesID (sync.go) produces: a raw hit's objectID
	// aliased to "id" so Store.extractObjectID recognizes it.
	// Same objectID ("shared1") present, unchanged, in both indices.
	if err := db.UpsertBrowse(json.RawMessage(`{"id":"shared1","objectID":"shared1","title":"same","indexes_id":"index_a"}`)); err != nil {
		t.Fatalf("UpsertBrowse index_a: %v", err)
	}
	if err := db.UpsertBrowse(json.RawMessage(`{"id":"shared1","objectID":"shared1","title":"same","indexes_id":"index_b"}`)); err != nil {
		t.Fatalf("UpsertBrowse index_b: %v", err)
	}
	// A record only in index_b: a genuine addition.
	if err := db.UpsertBrowse(json.RawMessage(`{"id":"onlyB","objectID":"onlyB","title":"new","indexes_id":"index_b"}`)); err != nil {
		t.Fatalf("UpsertBrowse onlyB: %v", err)
	}
	db.Close()

	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--json", "objects", "diff", "index_a", "index_b", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("objects diff error = %v, stdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	var result objectsDiffResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal output: %v, output:\n%s", err, out.String())
	}
	if result.Counts.Added != 1 || len(result.Added) != 1 || result.Added[0] != "onlyB" {
		t.Fatalf("added = %v (count %d), want [\"onlyB\"] (count 1) — output:\n%s", result.Added, result.Counts.Added, out.String())
	}
	if result.Counts.Removed != 0 {
		t.Fatalf("removed = %v, want none — the shared objectID must match across indices, output:\n%s", result.Removed, out.String())
	}
	if result.Counts.Changed != 0 {
		t.Fatalf("changed = %v, want none, output:\n%s", result.Changed, out.String())
	}
}
