// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
)

func TestDiffVariables(t *testing.T) {
	a := []variable{
		{ID: "v1", Name: "color/brand/primary", ValuesByMode: map[string]any{"light": "#000"}},
		{ID: "v2", Name: "spacing/md", ValuesByMode: map[string]any{"default": float64(8)}},
		{ID: "v3", Name: "color/legacy", ValuesByMode: map[string]any{"light": "#fff"}},
	}
	b := []variable{
		{ID: "v1", Name: "color/brand/primary", ValuesByMode: map[string]any{"light": "#111"}},  // value changed
		{ID: "v2", Name: "spacing/medium", ValuesByMode: map[string]any{"default": float64(8)}}, // renamed
		{ID: "v4", Name: "spacing/lg", ValuesByMode: map[string]any{"default": float64(16)}},    // added
	}

	res := diffVariables(a, b)

	if len(res.Added) != 1 || res.Added[0].ID != "v4" {
		t.Errorf("Added: got %+v, want [v4]", res.Added)
	}
	if len(res.Removed) != 1 || res.Removed[0].ID != "v3" {
		t.Errorf("Removed: got %+v, want [v3]", res.Removed)
	}
	if len(res.Renamed) != 1 || res.Renamed[0].ID != "v2" || res.Renamed[0].NewName != "spacing/medium" {
		t.Errorf("Renamed: got %+v, want [v2 medium]", res.Renamed)
	}
	if len(res.ValueChanged) != 1 || res.ValueChanged[0].ID != "v1" {
		t.Errorf("ValueChanged: got %+v, want [v1]", res.ValueChanged)
	}
}

func TestDiffVariables_ModeAware(t *testing.T) {
	// Same ids, same names, but a new mode appears in b.
	a := []variable{{ID: "v1", Name: "color/x", ValuesByMode: map[string]any{"light": "#000"}}}
	b := []variable{{ID: "v1", Name: "color/x", ValuesByMode: map[string]any{"light": "#000", "dark": "#fff"}}}
	res := diffVariables(a, b)
	if len(res.ValueChanged) != 1 {
		t.Errorf("expected ValueChanged on new mode, got %+v", res)
	}
}

func TestDiffVariables_Stable(t *testing.T) {
	// Reversing input order must not affect output ordering.
	a := []variable{{ID: "z", Name: "z"}, {ID: "a", Name: "a"}}
	b := []variable{{ID: "a", Name: "a"}, {ID: "z", Name: "z"}}
	r1 := diffVariables(a, b)
	r2 := diffVariables(b, a)
	if len(r1.Added) != 0 || len(r1.Removed) != 0 || len(r2.Added) != 0 || len(r2.Removed) != 0 {
		t.Errorf("expected no diffs, got %+v / %+v", r1, r2)
	}
}

// Regression test for the greptile P1 + Champworks review on PR #380:
// loadVariablesSnapshot must observe its version argument so a tokens diff
// cannot silently compare identical HEAD data for two distinct version
// inputs and report an empty diff while looking polished. HEAD (and "")
// load the current snapshot; any other version returns an explicit error
// until version-tagged snapshot storage is wired up.
func TestLoadVariablesSnapshot_VersionArgumentIsConsulted(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tokens-test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	// Seed via raw SQL rather than UpsertVariables so the test isn't coupled
	// to the typed-upsert helper's column wiring; what loadVariablesSnapshot
	// reads is what we want to exercise here.
	seed := func(id, name, light string) {
		data, err := json.Marshal(map[string]any{
			"id":           id,
			"name":         name,
			"resolvedType": "COLOR",
			"valuesByMode": map[string]any{"light": light},
		})
		if err != nil {
			t.Fatalf("marshal seed %s: %v", id, err)
		}
		if _, err := db.DB().Exec(
			`INSERT INTO variables (id, files_id, data) VALUES (?, ?, ?)`,
			id, "FILEKEY", string(data),
		); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	seed("v1", "color/brand/primary", "#000")
	seed("v2", "color/brand/secondary", "#fff")

	// HEAD and empty both read the current snapshot.
	for _, ver := range []string{"HEAD", "head", ""} {
		got, err := loadVariablesSnapshot(db, "FILEKEY", ver)
		if err != nil {
			t.Fatalf("version=%q: unexpected error: %v", ver, err)
		}
		if len(got) != 2 {
			t.Errorf("version=%q: expected 2 variables, got %d", ver, len(got))
		}
	}

	// Any non-HEAD version must error rather than silently return HEAD data.
	// Without this, `tokens diff --from a --to b` would always return an
	// empty diff because both calls hit the same query.
	_, err = loadVariablesSnapshot(db, "FILEKEY", "v123:rebrand")
	if err == nil {
		t.Fatalf("non-HEAD version: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "version-tagged") {
		t.Errorf("non-HEAD version: expected version-tagged error, got %v", err)
	}
}

// Composes loadVariablesSnapshot with diffVariables to prove that two
// distinct snapshot inputs produce a non-empty diff end-to-end. If
// loadVariablesSnapshot ever silently ignored its version argument, the
// two calls would return identical slices and this assertion would fail.
func TestTokensDiff_TwoDistinctSnapshotsProduceNonEmptyDiff(t *testing.T) {
	from := []variable{
		{ID: "v1", Name: "color/brand/primary", ValuesByMode: map[string]any{"light": "#000"}},
		{ID: "v2", Name: "spacing/md", ValuesByMode: map[string]any{"default": float64(8)}},
	}
	to := []variable{
		{ID: "v1", Name: "color/brand/primary", ValuesByMode: map[string]any{"light": "#111"}}, // value changed
		{ID: "v3", Name: "color/accent", ValuesByMode: map[string]any{"light": "#0af"}},        // added
	}

	diff := diffVariables(from, to)
	if len(diff.Added)+len(diff.Removed)+len(diff.Renamed)+len(diff.ValueChanged) == 0 {
		t.Fatalf("expected non-empty diff for distinct snapshots, got %+v", diff)
	}
	if len(diff.ValueChanged) != 1 || diff.ValueChanged[0].ID != "v1" {
		t.Errorf("expected v1 in ValueChanged, got %+v", diff.ValueChanged)
	}
	if len(diff.Added) != 1 || diff.Added[0].ID != "v3" {
		t.Errorf("expected v3 in Added, got %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].ID != "v2" {
		t.Errorf("expected v2 in Removed, got %+v", diff.Removed)
	}

	// Sanity-check the seed-side mismatch matches what the human reviewer
	// asked for: identical inputs MUST produce empty, distinct inputs MUST
	// not. Pin both to defend against regressions in either direction.
	noop := diffVariables(from, from)
	if len(noop.Added)+len(noop.Removed)+len(noop.Renamed)+len(noop.ValueChanged) != 0 {
		t.Errorf("identical inputs must produce empty diff, got %+v", noop)
	}
}
