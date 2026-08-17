// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: unit tests for the CookUnity SDUI menu client's pure
// extraction/conversion logic. Preserved on regen.

package cookunity

import "testing"

func TestToStr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"hello", "hello"},
		{float64(42), "42"},   // whole floats render without a decimal
		{float64(3.5), "3.5"}, // fractional floats keep precision
		{true, "true"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := toStr(c.in); got != c.want {
			t.Errorf("toStr(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{float64(12.5), 12.5},
		{"7.25", 7.25}, // JSON-string numbers are tolerated
		{" 3 ", 3},     // surrounding whitespace is trimmed
		{"not-a-number", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := toFloat(c.in); got != c.want {
			t.Errorf("toFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestToInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{float64(9), 9},
		{"15", 15},
		{"20.0", 20}, // float-shaped strings fall back to ParseFloat then truncate
		{"bad", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := toInt(c.in); got != c.want {
			t.Errorf("toInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToBool(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{true, true},
		{"true", true},
		{"1", true},
		{"false", false},
		{float64(1), true},
		{float64(0), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := toBool(c.in); got != c.want {
			t.Errorf("toBool(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestJoinStrings(t *testing.T) {
	if got := joinStrings([]any{"a", "b", "c"}); got != "a, b, c" {
		t.Errorf("joinStrings slice = %q", got)
	}
	if got := joinStrings("solo"); got != "solo" {
		t.Errorf("joinStrings scalar = %q", got)
	}
	if got := joinStrings(nil); got != "" {
		t.Errorf("joinStrings nil = %q, want empty", got)
	}
}

func TestJoinNamed(t *testing.T) {
	in := []any{
		map[string]any{"name": "Gluten"},
		map[string]any{"name": "Soy"},
		map[string]any{"other": "ignored"}, // missing field contributes nothing
	}
	if got := joinNamed(in, "name"); got != "Gluten, Soy" {
		t.Errorf("joinNamed = %q, want %q", got, "Gluten, Soy")
	}
	if got := joinNamed("not-an-array", "name"); got != "" {
		t.Errorf("joinNamed non-array = %q, want empty", got)
	}
}

func TestFlattenMeal(t *testing.T) {
	props := map[string]any{
		"id":    float64(101),
		"name":  "Grilled Chicken Bowl",
		"price": float64(13.99),
		"chef": map[string]any{
			"id":        float64(7),
			"firstname": "Ada",
			"lastname":  "Lovelace",
		},
		"nutritionalInfo": map[string]any{
			"protein": "40", // values may arrive as JSON strings
			"carbs":   float64(30),
			"fat":     float64(12),
		},
		"cuisines":   []any{"American", "Comfort"},
		"allergens":  []any{map[string]any{"name": "Soy"}},
		"isFavorite": true,
	}
	m := flattenMeal(props, "2026-07-28")

	if m.Id != 101 {
		t.Errorf("Id = %d, want 101", m.Id)
	}
	if m.Name != "Grilled Chicken Bowl" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Price != 13.99 {
		t.Errorf("Price = %v, want 13.99", m.Price)
	}
	if m.ChefId != 7 || m.ChefName != "Ada Lovelace" {
		t.Errorf("chef = %d/%q, want 7/Ada Lovelace", m.ChefId, m.ChefName)
	}
	if m.Protein != 40 {
		t.Errorf("Protein = %v, want 40 (string coercion)", m.Protein)
	}
	if m.Cuisines != "American, Comfort" {
		t.Errorf("Cuisines = %q", m.Cuisines)
	}
	if m.Allergens != "Soy" {
		t.Errorf("Allergens = %q, want Soy", m.Allergens)
	}
	if m.DeliveryDate != "2026-07-28" {
		t.Errorf("DeliveryDate = %q", m.DeliveryDate)
	}
	if !m.IsFavorite {
		t.Error("IsFavorite = false, want true")
	}
}

func TestFlattenMealNutritionFallback(t *testing.T) {
	// With no nutritionalInfo object, flattenMeal falls back to top-level ratio fields.
	props := map[string]any{
		"id":           float64(5),
		"ratioProtein": float64(25),
		"ratioCarb":    float64(50),
		"ratioFat":     float64(10),
	}
	m := flattenMeal(props, "2026-07-28")
	if m.Protein != 25 || m.Carbs != 50 || m.Fat != 10 {
		t.Errorf("fallback macros = %v/%v/%v, want 25/50/10", m.Protein, m.Carbs, m.Fat)
	}
}

func TestCollectClusters(t *testing.T) {
	tree := map[string]any{
		"components": []any{
			map[string]any{
				"type": "FULL_MENU_LAZY_CLUSTER",
				"attributes": map[string]any{
					"lazyCluster": map[string]any{
						"attributes": map[string]any{
							"path": "/web/view/menu/components/clustered-result",
							"params": map[string]any{
								"filterBy": "chicken",
							},
						},
					},
				},
			},
			map[string]any{"type": "SOMETHING_ELSE"}, // ignored
		},
	}
	clusters := collectClusters(tree)
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	if clusters[0].params["filterBy"] != "chicken" {
		t.Errorf("filterBy = %q, want chicken", clusters[0].params["filterBy"])
	}
}

func TestCollectMealProperties(t *testing.T) {
	tree := map[string]any{
		"rows": []any{
			map[string]any{
				"type":       "MEAL",
				"properties": map[string]any{"id": float64(1), "name": "A"},
			},
			map[string]any{
				"type":       "BANNER", // non-MEAL nodes are skipped
				"properties": map[string]any{"id": float64(999)},
			},
			map[string]any{
				"type":       "MEAL",
				"properties": map[string]any{"id": float64(2), "name": "B"},
			},
		},
	}
	props := collectMealProperties(tree)
	if len(props) != 2 {
		t.Fatalf("got %d meal props, want 2", len(props))
	}
}
