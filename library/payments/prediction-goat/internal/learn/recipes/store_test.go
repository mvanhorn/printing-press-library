// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// openTestStore opens a fresh v6 store under t.TempDir. Returns the
// open store; t.Cleanup closes it.
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "recipes.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleRecipe() recipes.Recipe {
	return recipes.Recipe{
		QueryTemplate:    "{entity} cup odds wins world",
		ResourceTemplate: "KXMENWORLDCUP-26-{entity:country_iso2}",
		ResourceType:     "kalshi_markets",
		Venue:            "kalshi",
		Strategy:         recipes.StrategySubstitute,
		EntityKind:       "country_iso2",
		Confidence:       2,
		Source:           recipes.SourceTaught,
		ExampleQuery:     "odds Portugal wins world cup",
		ExampleResource:  "KXMENWORLDCUP-26-PT",
	}
}

func TestUpsert_InsertsNewRow(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	id, inserted, err := recipes.Upsert(s.DB(), sampleRecipe())
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !inserted {
		t.Errorf("first upsert should be inserted=true")
	}
	if id == 0 {
		t.Errorf("first upsert should return non-zero id")
	}
}

func TestUpsert_DuplicateBumpsConfidence(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	r := sampleRecipe()
	if _, _, err := recipes.Upsert(s.DB(), r); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	id2, inserted, err := recipes.Upsert(s.DB(), r)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if inserted {
		t.Errorf("second upsert of the same triple should be inserted=false")
	}
	if id2 == 0 {
		t.Errorf("second upsert should still return the existing id")
	}
	// Confidence should be bumped to 3.
	got, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly 1 row, got %d", len(got))
	}
	if got[0].Confidence != 3 {
		t.Errorf("confidence after second upsert = %d, want 3", got[0].Confidence)
	}
}

func TestUpsert_RejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	cases := []struct {
		name  string
		mutate func(r *recipes.Recipe)
	}{
		{"empty query_template", func(r *recipes.Recipe) { r.QueryTemplate = "" }},
		{"empty resource_template", func(r *recipes.Recipe) { r.ResourceTemplate = "" }},
		{"empty strategy", func(r *recipes.Recipe) { r.Strategy = "" }},
		{"empty entity_kind", func(r *recipes.Recipe) { r.EntityKind = "" }},
		{"empty resource_type", func(r *recipes.Recipe) { r.ResourceType = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := sampleRecipe()
			tc.mutate(&r)
			if _, _, err := recipes.Upsert(s.DB(), r); err == nil {
				t.Errorf("expected error for case %q", tc.name)
			}
		})
	}
}

func TestList_FiltersByEntityKindAndStrategy(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	r1 := sampleRecipe() // country_iso2 + substitute
	r2 := sampleRecipe()
	r2.QueryTemplate = "{entity} odds wins"
	r2.ResourceTemplate = "will-{entity:lowercase}-win-the-2026-fifa-world-cup-*"
	r2.Strategy = recipes.StrategySubstituteThenSearchPrefix
	r2.EntityKind = "lowercase"
	r2.ResourceType = "markets"
	r2.Venue = "polymarket"
	r2.ExampleResource = "will-portugal-win-the-2026-fifa-world-cup-912"

	if _, _, err := recipes.Upsert(s.DB(), r1); err != nil {
		t.Fatalf("upsert r1: %v", err)
	}
	if _, _, err := recipes.Upsert(s.DB(), r2); err != nil {
		t.Fatalf("upsert r2: %v", err)
	}

	// Filter by entity_kind.
	rows, err := recipes.List(s.DB(), recipes.ListFilter{EntityKind: "country_iso2"})
	if err != nil {
		t.Fatalf("list by kind: %v", err)
	}
	if len(rows) != 1 || rows[0].EntityKind != "country_iso2" {
		t.Errorf("want exactly one country_iso2 row, got %+v", rows)
	}

	// Filter by strategy.
	rows, err = recipes.List(s.DB(), recipes.ListFilter{Strategy: recipes.StrategySubstituteThenSearchPrefix})
	if err != nil {
		t.Fatalf("list by strategy: %v", err)
	}
	if len(rows) != 1 || rows[0].Strategy != recipes.StrategySubstituteThenSearchPrefix {
		t.Errorf("want exactly one prefix-strategy row, got %+v", rows)
	}

	// Unfiltered returns both.
	rows, err = recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("want 2 rows total, got %d", len(rows))
	}
}

func TestForget_RequiresFilterOrAll(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	if _, err := recipes.Forget(s.DB(), recipes.ForgetFilter{}); err == nil {
		t.Errorf("Forget with empty filter should error")
	}
}

func TestForget_ByEntityKindDeletesMatching(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	r1 := sampleRecipe()
	r2 := sampleRecipe()
	r2.QueryTemplate = "different template"
	r2.EntityKind = "lowercase"

	if _, _, err := recipes.Upsert(s.DB(), r1); err != nil {
		t.Fatalf("upsert r1: %v", err)
	}
	if _, _, err := recipes.Upsert(s.DB(), r2); err != nil {
		t.Fatalf("upsert r2: %v", err)
	}

	n, err := recipes.Forget(s.DB(), recipes.ForgetFilter{EntityKind: "country_iso2"})
	if err != nil {
		t.Fatalf("forget: %v", err)
	}
	if n != 1 {
		t.Errorf("Forget removed %d rows, want 1", n)
	}
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].EntityKind != "lowercase" {
		t.Errorf("after Forget, want only the lowercase row, got %+v", rows)
	}
}

func TestForget_AllWipesEverything(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	if _, _, err := recipes.Upsert(s.DB(), sampleRecipe()); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	n, err := recipes.Forget(s.DB(), recipes.ForgetFilter{All: true})
	if err != nil {
		t.Fatalf("forget all: %v", err)
	}
	if n != 1 {
		t.Errorf("Forget All removed %d, want 1", n)
	}
	rows, err := recipes.List(s.DB(), recipes.ListFilter{})
	if err != nil {
		t.Fatalf("list after wipe: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("after Forget All want 0 rows, got %d", len(rows))
	}
}
