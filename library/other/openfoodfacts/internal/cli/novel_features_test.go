// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Real table-driven tests for the novel-feature pure logic.

package cli

import (
	"reflect"
	"testing"
)

func sampleProduct() map[string]any {
	return map[string]any{
		"code":             "3017620422003",
		"product_name":     "Nutella",
		"nutriscore_grade": "e",
		"nova_group":       float64(4),
		"serving_quantity": float64(15),
		"categories_tags":  []any{"en:breakfasts", "en:spreads", "en:sweet-spreads"},
		"allergens_tags":   []any{"en:milk", "en:nuts", "en:soybeans"},
		"traces_tags":      []any{"en:peanuts"},
		"nutriments": map[string]any{
			"energy-kcal_100g":   float64(539),
			"proteins_100g":      float64(6.3),
			"fat_100g":           float64(30.9),
			"saturated-fat_100g": float64(10.6),
			"carbohydrates_100g": float64(57.5),
			"sugars_100g":        float64(56.3),
			"salt_100g":          float64(0.107),
		},
	}
}

func TestNutrFromObjectAndScale(t *testing.T) {
	n := nutrFromObject(sampleProduct())
	if n.Kcal != 539 || n.Protein != 6.3 || n.Sugars != 56.3 {
		t.Fatalf("unexpected nutriments: %+v", n)
	}
	got := n.scaled(30.0 / 100.0)
	if want := 539 * 0.3; got.Kcal < want-0.01 || got.Kcal > want+0.01 {
		t.Errorf("scaled kcal = %.3f, want %.3f", got.Kcal, want)
	}
}

func TestNutrAddAndPerServing(t *testing.T) {
	a := nutriments{Kcal: 100, Protein: 5}
	b := nutriments{Kcal: 200, Protein: 15}
	sum := a.add(b)
	if sum.Kcal != 300 || sum.Protein != 20 {
		t.Fatalf("add = %+v", sum)
	}
	per := sum.scaled(1.0 / 4.0)
	if per.Kcal != 75 {
		t.Errorf("per-serving kcal = %.1f, want 75", per.Kcal)
	}
}

func TestResolveGrams(t *testing.T) {
	prod := sampleProduct() // serving_quantity 15
	noServing := map[string]any{"nutriments": map[string]any{}}
	var discard = devNull{}

	cases := []struct {
		name            string
		prod            map[string]any
		grams, servings string
		want            float64
		wantErr         bool
	}{
		{"grams wins", prod, "30", "2", 30, false},
		{"servings with serving size", prod, "", "2", 30, false},
		{"servings no serving size -> 100g", noServing, "", "2", 200, false},
		{"default 100g", prod, "", "", 100, false},
		{"bad grams", prod, "-5", "", 0, true},
	}
	for _, tc := range cases {
		got, err := resolveGrams(tc.prod, tc.grams, tc.servings, discard)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: grams = %.1f, want %.1f", tc.name, got, tc.want)
		}
	}
}

func TestFriendlyNutrientKey(t *testing.T) {
	cases := map[string]string{
		"kcal": "energy-kcal_100g", "sugar": "sugars_100g", "sugars": "sugars_100g",
		"protein": "proteins_100g", "fat": "fat_100g", "satfat": "saturated-fat_100g",
		"salt": "salt_100g", "carbs": "carbohydrates_100g", "fiber": "fiber_100g",
	}
	for in, want := range cases {
		got, ok := friendlyNutrientKey(in)
		if !ok || got != want {
			t.Errorf("friendlyNutrientKey(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := friendlyNutrientKey("vibes"); ok {
		t.Error("expected unknown nutrient to fail")
	}
}

func TestNormalizeAllergenList(t *testing.T) {
	got := normalizeAllergenList(" Milk , gluten,MILK, ,nuts")
	want := []string{"gluten", "milk", "nuts"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalize = %v, want %v", got, want)
	}
	if len(normalizeAllergenList("")) != 0 {
		t.Error("empty list should normalize to empty")
	}
}

func TestAllergenHits(t *testing.T) {
	prod := sampleProduct()
	allergens := prodTags(prod, "allergens_tags") // milk, nuts, soybeans
	traces := prodTags(prod, "traces_tags")       // peanuts

	if hits := allergenHits([]string{"milk", "gluten"}, allergens, traces); !reflect.DeepEqual(hits, []string{"milk"}) {
		t.Errorf("hits = %v, want [milk]", hits)
	}
	if hits := allergenHits([]string{"gluten", "sesame"}, allergens, traces); len(hits) != 0 {
		t.Errorf("expected no hits, got %v", hits)
	}
	if hits := allergenHits([]string{"peanuts"}, allergens, traces); !reflect.DeepEqual(hits, []string{"peanuts"}) {
		t.Errorf("trace hit = %v, want [peanuts]", hits)
	}
}

func TestProdTagsAndFirstRaw(t *testing.T) {
	prod := sampleProduct()
	tags := prodTags(prod, "categories_tags")
	if len(tags) != 3 || tags[0] != "breakfasts" {
		t.Errorf("prodTags = %v", tags)
	}
	if raw := prodFirstRawTag(prod, "categories_tags"); raw != "en:sweet-spreads" {
		t.Errorf("prodFirstRawTag = %q, want en:sweet-spreads (most specific)", raw)
	}
	if raw := prodFirstRawTag(map[string]any{}, "categories_tags"); raw != "" {
		t.Errorf("missing tags should return empty, got %q", raw)
	}
}

func TestProdFirstRawTagSkipsLocalized(t *testing.T) {
	prod := map[string]any{"categories_tags": []any{
		"en:breakfasts", "en:sweet-spreads", "en:confectionary-based-spreads",
		"en:Petit-déjeuners", "en:Pâtes à tartiner",
	}}
	got := prodFirstRawTag(prod, "categories_tags")
	if got != "en:confectionary-based-spreads" {
		t.Errorf("prodFirstRawTag = %q, want en:confectionary-based-spreads (last canonical)", got)
	}
	if isCanonicalTag("en:Pâtes à tartiner") || !isCanonicalTag("en:sweet-spreads") {
		t.Error("isCanonicalTag misclassified")
	}
}

func TestCategoryMatches(t *testing.T) {
	prod := sampleProduct()
	if !categoryMatches(prod, "spreads") {
		t.Error("expected spreads to match")
	}
	if categoryMatches(prod, "beverages") {
		t.Error("beverages should not match")
	}
}

func TestIsProductNotFound(t *testing.T) {
	if !isProductNotFound([]byte(`{"status":0,"status_verbose":"no code or invalid code"}`)) {
		t.Error("status 0 should be not-found")
	}
	if isProductNotFound([]byte(`{"status":1,"product":{"code":"x"}}`)) {
		t.Error("status 1 should be found")
	}
	if isProductNotFound([]byte(`not json`)) {
		t.Error("unparseable should not be treated as not-found")
	}
}

func TestCoerceFloat(t *testing.T) {
	if coerceFloat(float64(5)) != 5 || coerceFloat("3.5") != 3.5 || coerceFloat(nil) != 0 {
		t.Error("coerceFloat conversions wrong")
	}
}

// devNull discards writes (stand-in for a stderr warn sink in tests).
type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }
