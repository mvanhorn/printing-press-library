// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"sort"
	"testing"
)

// The package init() registers the two peer sources. These tests exercise the
// registry contract the `sources` command tree relies on: both sources are
// present, All() returns them sorted by name, and Lookup distinguishes a hit
// from a miss.

func TestAllReturnsRegisteredSourcesSorted(t *testing.T) {
	all := All()
	if len(all) != 2 {
		t.Fatalf("All() = %d sources, want 2 (usda, nutritionvalue)", len(all))
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i].Name < all[j].Name }) {
		names := make([]string, len(all))
		for i, s := range all {
			names[i] = s.Name
		}
		t.Errorf("All() not sorted by name: %v", names)
	}
	// nutritionvalue sorts before usda.
	if all[0].Name != "nutritionvalue" || all[1].Name != "usda" {
		t.Errorf("All() order = [%s, %s], want [nutritionvalue, usda]", all[0].Name, all[1].Name)
	}
}

func TestLookupHitAndMiss(t *testing.T) {
	usda, ok := Lookup("usda")
	if !ok {
		t.Fatal("Lookup(usda) missing; expected registered source")
	}
	if !usda.AuthRequired {
		t.Errorf("usda.AuthRequired = false, want true (FDC requires an api.data.gov key)")
	}
	if usda.BaseURL == "" {
		t.Error("usda.BaseURL is empty")
	}

	nv, ok := Lookup("nutritionvalue")
	if !ok {
		t.Fatal("Lookup(nutritionvalue) missing; expected registered source")
	}
	if nv.AuthRequired {
		t.Errorf("nutritionvalue.AuthRequired = true, want false (no key required)")
	}

	if _, ok := Lookup("does-not-exist"); ok {
		t.Error("Lookup(does-not-exist) returned ok=true, want false")
	}
}

func TestRegisterOverwritesByName(t *testing.T) {
	// Register is keyed by Name, so re-registering the same name replaces the
	// entry rather than duplicating it. Restore afterward so other tests see
	// the init() state.
	orig, _ := Lookup("usda")
	t.Cleanup(func() { Register(orig) })

	Register(Source{Name: "usda", Description: "replaced", BaseURL: "https://example.test"})
	got, ok := Lookup("usda")
	if !ok {
		t.Fatal("Lookup(usda) missing after re-register")
	}
	if got.Description != "replaced" {
		t.Errorf("Description = %q, want %q", got.Description, "replaced")
	}
	if len(All()) != 2 {
		t.Errorf("All() = %d after re-register, want 2 (no duplicate)", len(All()))
	}
}
