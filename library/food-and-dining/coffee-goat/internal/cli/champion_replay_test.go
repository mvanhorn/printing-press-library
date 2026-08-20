// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestChampionReplayList verifies the curated static library is enumerable
// without any store setup and that --json returns a stable shape.
func TestChampionReplayList(t *testing.T) {
	out, err := runCmd(t, "champion-replay", "list", "--json")
	if err != nil {
		t.Fatalf("champion-replay list: %v\nout=%s", err, out)
	}
	var resp struct {
		Recipes []championRecipe `json:"recipes"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if resp.Count != len(championLibrary) {
		t.Fatalf("expected count=%d, got %d", len(championLibrary), resp.Count)
	}
	if len(resp.Recipes) == 0 {
		t.Fatal("expected at least one recipe in the library")
	}
	// Recipes should be returned newest-first.
	for i := 1; i < len(resp.Recipes); i++ {
		if resp.Recipes[i-1].Year < resp.Recipes[i].Year {
			t.Errorf("recipes not sorted year-desc: %d before %d", resp.Recipes[i-1].Year, resp.Recipes[i].Year)
		}
	}
}

// TestChampionReplayListYearFilter verifies --year narrows to one year.
func TestChampionReplayListYearFilter(t *testing.T) {
	out, err := runCmd(t, "champion-replay", "list", "--year", "2023", "--json")
	if err != nil {
		t.Fatalf("list --year: %v\nout=%s", err, out)
	}
	var resp struct {
		Recipes []championRecipe `json:"recipes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	for _, r := range resp.Recipes {
		if r.Year != 2023 {
			t.Errorf("recipe %s has year %d, expected 2023", r.Slug, r.Year)
		}
	}
}

// TestChampionReplayShowUnknownSlug verifies the not-found path returns a
// helpful error mentioning the list subcommand.
func TestChampionReplayShowUnknownSlug(t *testing.T) {
	out, err := runCmd(t, "champion-replay", "show", "definitely-not-a-real-slug")
	if err == nil {
		t.Fatalf("expected error for unknown slug, got success: %s", out)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// TestChampionReplayShop verifies that a recipe matches seeded products
// whose origin/process align with the recipe, and that a recipe with no
// matching corpus returns an empty result rather than erroring.
func TestChampionReplayShop(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()

	// Find an Ethiopia/anaerobic recipe to match. The 2024 entry is one.
	var slug string
	for _, r := range championLibrary {
		if strings.EqualFold(r.Origin, "Ethiopia") && strings.Contains(r.Process, "anaerobic") {
			slug = r.Slug
			break
		}
	}
	if slug == "" {
		t.Skip("no Ethiopian anaerobic recipe in fixture; skipping match test")
	}

	// Seed a matching product.
	seedProduct(t, s, "onyx", "ethiopia-anaerobic-natural", map[string]any{
		"title":    "Ethiopia Anaerobic Natural Worka",
		"origin":   "Ethiopia",
		"process":  "anaerobic-natural",
		"varietal": "heirloom",
		"in_stock": 1,
	})
	// And one that should NOT match (different origin).
	seedProduct(t, s, "sey", "colombia-washed", map[string]any{
		"title":    "Colombia Washed",
		"origin":   "Colombia",
		"process":  "washed",
		"varietal": "caturra",
		"in_stock": 1,
	})

	out, err := runCmd(t, "champion-replay", "shop", slug, "--in-stock", "--json")
	if err != nil {
		t.Fatalf("shop: %v\nout=%s", err, out)
	}
	var resp struct {
		Matches []championMatch `json:"matches"`
		Count   int             `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	// At least the Ethiopia one should match with origin+process score = 0.8.
	if resp.Count < 1 {
		t.Fatalf("expected at least one match for %s, got %d (matches=%+v)", slug, resp.Count, resp.Matches)
	}
	for _, m := range resp.Matches {
		if !strings.EqualFold(strings.TrimSpace(m.Origin), "Ethiopia") {
			t.Errorf("match %s has non-Ethiopia origin %q; should have been filtered by score", m.Handle, m.Origin)
		}
	}
}

// TestChampionLibraryIntegrity guards the curated library against accidental
// breakage: every recipe must have a slug, year, and origin so list/show/shop
// can rely on those fields.
func TestChampionLibraryIntegrity(t *testing.T) {
	if len(championLibrary) == 0 {
		t.Fatal("championLibrary is empty; check the static reference data")
	}
	seen := map[string]bool{}
	for _, r := range championLibrary {
		if r.Slug == "" {
			t.Errorf("recipe with empty slug: %+v", r)
		}
		if seen[r.Slug] {
			t.Errorf("duplicate slug %q", r.Slug)
		}
		seen[r.Slug] = true
		if r.Year == 0 {
			t.Errorf("recipe %q has zero Year", r.Slug)
		}
		if r.Origin == "" {
			t.Errorf("recipe %q has empty Origin", r.Slug)
		}
		if r.Method == "" {
			t.Errorf("recipe %q has empty Method", r.Slug)
		}
	}
}
