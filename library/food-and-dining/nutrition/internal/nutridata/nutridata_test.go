// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package nutridata

import (
	"encoding/json"
	"testing"
)

func TestNormalizeFoundationShape(t *testing.T) {
	raw := json.RawMessage(`{
		"fdcId": 173414,
		"description": "Cheese, cheddar",
		"dataType": "SR Legacy",
		"publishedDate": "2019-04-01",
		"foodNutrients": [
			{"nutrient":{"number":"208","name":"Energy","unitName":"KCAL"},"amount":403},
			{"nutrient":{"number":"203","name":"Protein","unitName":"G"},"amount":22.9},
			{"nutrient":{"number":"204","name":"Total lipid (fat)","unitName":"G"},"amount":33.3},
			{"nutrient":{"number":"205","name":"Carbohydrate, by difference","unitName":"G"},"amount":3.1}
		]
	}`)
	f, err := Normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if f.FdcID != 173414 || f.Description != "Cheese, cheddar" {
		t.Errorf("bad id/desc: %+v", f)
	}
	if f.Calories() != 403 || f.Protein() != 22.9 || f.Fat() != 33.3 || f.Carbs() != 3.1 {
		t.Errorf("bad macros: kcal=%v protein=%v fat=%v carbs=%v", f.Calories(), f.Protein(), f.Fat(), f.Carbs())
	}
}

func TestNormalizeAbridgedShape(t *testing.T) {
	raw := json.RawMessage(`{
		"fdcId": 171287,
		"description": "Chicken breast",
		"dataType": "Survey (FNDDS)",
		"foodNutrients": [
			{"number":"203","name":"Protein","unitName":"G","amount":31.0},
			{"number":"208","name":"Energy","unitName":"KCAL","amount":165}
		]
	}`)
	f, err := Normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if f.Protein() != 31.0 || f.Calories() != 165 {
		t.Errorf("abridged parse failed: %+v", f.Nutrients)
	}
}

func TestNormalizeBrandedLabelNutrients(t *testing.T) {
	raw := json.RawMessage(`{
		"fdcId": 999999,
		"description": "Branded cheddar",
		"dataType": "Branded",
		"servingSize": 28,
		"servingSizeUnit": "g",
		"labelNutrients": {
			"calories": {"value": 110},
			"protein": {"value": 7},
			"fat": {"value": 9}
		}
	}`)
	f, err := Normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	// 110 kcal per 28 g -> ~392.86 per 100 g.
	if got := f.Calories(); got < 390 || got > 396 {
		t.Errorf("branded calories rescale wrong: %v", got)
	}
}

func TestCaloriesAtwaterFallback(t *testing.T) {
	// Foundation foods omit nutrient 208 and report energy via Atwater factors.
	raw := json.RawMessage(`{
		"fdcId": 2646170,
		"description": "Chicken, breast, boneless, skinless, raw",
		"dataType": "Foundation",
		"foodNutrients": [
			{"nutrient":{"number":"203","name":"Protein","unitName":"G"},"amount":22.5},
			{"nutrient":{"number":"957","name":"Energy (Atwater General Factors)","unitName":"KCAL"},"amount":106.0},
			{"nutrient":{"number":"958","name":"Energy (Atwater Specific Factors)","unitName":"KCAL"},"amount":112.2}
		]
	}`)
	f, err := Normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got := f.Calories(); got != 106.0 {
		t.Errorf("Atwater energy fallback: Calories() = %v, want 106.0 (nutrient 957)", got)
	}
	// 208 present should still win over Atwater.
	raw2 := json.RawMessage(`{"fdcId":1,"foodNutrients":[
		{"nutrient":{"number":"208","name":"Energy","unitName":"KCAL"},"amount":250},
		{"nutrient":{"number":"957","name":"Energy (Atwater General Factors)","unitName":"KCAL"},"amount":106}
	]}`)
	f2, _ := Normalize(raw2)
	if got := f2.Calories(); got != 250 {
		t.Errorf("nutrient 208 should win: Calories() = %v, want 250", got)
	}
}

func TestResolveNutrient(t *testing.T) {
	cases := map[string]string{
		"protein":  NutrNumProtein,
		"kcal":     NutrNumEnergyKcal,
		"calories": NutrNumEnergyKcal,
		"carbs":    NutrNumCarb,
		"208":      "208",
	}
	for in, want := range cases {
		got, ok := ResolveNutrient(in)
		if !ok || got != want {
			t.Errorf("ResolveNutrient(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := ResolveNutrient("unobtainium"); ok {
		t.Error("expected unknown nutrient to fail")
	}
}
