// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package nutritionvalue

import (
	"os"
	"path/filepath"
	"testing"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}

func TestParseSearchRows(t *testing.T) {
	rows := parseSearchRows(readTestdata(t, "search_cheddar.html"))
	if len(rows) < 5 {
		t.Fatalf("expected several search rows, got %d", len(rows))
	}
	// The first generic result should be cheddar cheese with a real id.
	found := false
	for _, r := range rows {
		if r.ID == "173414" {
			found = true
			if r.Slug == "" || r.Name == "" {
				t.Errorf("row for 173414 missing slug/name: %+v", r)
			}
		}
	}
	if !found {
		t.Errorf("expected id 173414 among search rows; got %+v", rows)
	}
}

func TestParseFoodDetail(t *testing.T) {
	d := parseFoodDetail(readTestdata(t, "food_cheddar.html"), "/Cheese%2C_cheddar_nutritional_value.html")
	if d.Name == "" {
		t.Error("expected a food name")
	}
	// Net carbs is a NutritionValue.org-derived field the USDA API lacks.
	if d.NetCarbs == nil {
		t.Fatal("expected net carbs to be extracted")
	}
	if *d.NetCarbs <= 0 {
		t.Errorf("net carbs should be positive, got %v", *d.NetCarbs)
	}
	// Omega ratio table.
	if d.OmegaRatio == nil || d.Omega3 == nil || d.Omega6 == nil {
		t.Fatalf("expected omega fields: ratio=%v o3=%v o6=%v", d.OmegaRatio, d.Omega3, d.Omega6)
	}
	if *d.OmegaRatio <= 0 {
		t.Errorf("omega ratio should be positive, got %v", *d.OmegaRatio)
	}
	// The nutrient table should contain core macros.
	for _, key := range []string{"Carbohydrate", "Protein", "Fat"} {
		if _, ok := d.Nutrients[key]; !ok {
			t.Errorf("expected nutrient %q in detail table; keys present: %d", key, len(d.Nutrients))
		}
	}
}

func TestParseRankRows(t *testing.T) {
	rows := parseRankRows(readTestdata(t, "rank_protein.html"))
	if len(rows) < 10 {
		t.Fatalf("expected many ranked rows, got %d", len(rows))
	}
	if rows[0].Rank != 1 || rows[0].Name == "" || rows[0].Amount <= 0 {
		t.Errorf("first rank row malformed: %+v", rows[0])
	}
}

func TestNameOverlapCoverage(t *testing.T) {
	// Exact same multi-token name scores 1.0.
	if got := nameOverlap("Cheese, cheddar", "Cheese, cheddar"); got != 1.0 {
		t.Errorf("identical names: got %v, want 1.0", got)
	}
	// A close multi-token match with extra qualifiers still scores high on
	// coverage (all query tokens are present in the candidate).
	if got := nameOverlap("Bananas, raw", "Bananas, ripe and slightly ripe, raw"); got < 0.5 {
		t.Errorf("close match should have high coverage: got %v", got)
	}
	// No overlap scores 0.
	if got := nameOverlap("Cheddar cheese", "Grilled salmon"); got != 0 {
		t.Errorf("disjoint names: got %v, want 0", got)
	}
}

func TestSharedTokenGuard(t *testing.T) {
	// The two-token floor is what stops a single shared token from clearing the
	// fallback: "Fish" shares only one token with "Fish oil, cod liver".
	if got := sharedTokenCount(nameTokens("Fish"), nameTokens("Fish oil, cod liver, raw")); got >= 2 {
		t.Errorf("single-token query should share <2 tokens: got %d", got)
	}
	// A real multi-word match shares 2+ tokens.
	if got := sharedTokenCount(nameTokens("Bananas, raw"), nameTokens("Bananas, ripe and slightly ripe, raw")); got < 2 {
		t.Errorf("multi-word match should share >=2 tokens: got %d", got)
	}
}

func TestNutrientPageName(t *testing.T) {
	cases := map[string]string{
		"protein":   "Protein",
		"vitamin c": "Vitamin C",
		"carbs":     "Carbohydrate",
		"Potassium": "Potassium",
		"unknownX":  "unknownX",
	}
	for in, want := range cases {
		if got := NutrientPageName(in); got != want {
			t.Errorf("NutrientPageName(%q) = %q, want %q", in, got, want)
		}
	}
}
