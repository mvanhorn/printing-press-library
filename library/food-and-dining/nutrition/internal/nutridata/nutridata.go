// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package nutridata normalizes USDA FoodData Central food records into one
// canonical shape. FDC returns four inconsistent dataType shapes (Foundation,
// SR Legacy, Survey/FNDDS use foodNutrients[] with nutrient.number/name/amount;
// Branded foods carry labelNutrients keyed by name plus a servingSize). Every
// novel command that reads nutrient values goes through Normalize so the rest
// of the CLI never re-implements the per-dataType branching.
package nutridata

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Canonical USDA nutrient numbers used across the CLI. These are the "number"
// field on foodNutrients (strings in the API, e.g. "203"), not the internal
// nutrient id (1003). Kept as a typed table so callers resolve by meaning, not
// by magic string.
const (
	NutrNumEnergyKcal = "208"
	// Foundation and some Survey foods omit nutrient 208 and instead report
	// energy via Atwater factors (957 general, 958 specific). Energy() falls
	// back to these so calorie-derived math (protein density, meal totals)
	// works across every dataType, not just Branded/SR Legacy.
	NutrNumEnergyAtwaterGeneral  = "957"
	NutrNumEnergyAtwaterSpecific = "958"
	NutrNumProtein               = "203"
	NutrNumFat                   = "204"
	NutrNumCarb                  = "205"
	NutrNumFiber                 = "291"
	NutrNumSugars                = "269"
	NutrNumSodium                = "307"
	NutrNumCholest               = "601"
	NutrNumSatFat                = "606"
	NutrNumOmega6LA              = "675" // PUFA 18:2 n-6 (linoleic)
	NutrNumOmega3ALA             = "851" // PUFA 18:3 n-3 (alpha-linolenic)
)

// nutrientAliases resolves common user-facing nutrient names to USDA nutrient
// numbers for the filterable macro set used by `find` and `meal`.
var nutrientAliases = map[string]string{
	"kcal":          NutrNumEnergyKcal,
	"calories":      NutrNumEnergyKcal,
	"energy":        NutrNumEnergyKcal,
	"protein":       NutrNumProtein,
	"fat":           NutrNumFat,
	"carbs":         NutrNumCarb,
	"carbohydrate":  NutrNumCarb,
	"carbohydrates": NutrNumCarb,
	"fiber":         NutrNumFiber,
	"sugars":        NutrNumSugars,
	"sugar":         NutrNumSugars,
	"sodium":        NutrNumSodium,
	"cholesterol":   NutrNumCholest,
	"satfat":        NutrNumSatFat,
	"saturated":     NutrNumSatFat,
}

// ResolveNutrient maps a user nutrient name (or a bare nutrient number) to a
// USDA nutrient number. Returns false when unknown.
func ResolveNutrient(name string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if num, ok := nutrientAliases[key]; ok {
		return num, true
	}
	// Accept a raw nutrient number directly.
	if _, err := strconv.Atoi(key); err == nil {
		return key, true
	}
	return "", false
}

// Nutrient is one normalized nutrient value.
type Nutrient struct {
	Number string  `json:"number"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// Food is the canonical normalized record, independent of USDA dataType.
type Food struct {
	FdcID       int    `json:"fdc_id"`
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	BrandOwner  string `json:"brand_owner,omitempty"`
	PublishedAt string `json:"published_date,omitempty"`
	// Nutrients per 100 g (USDA foodNutrients are per 100 g for the
	// non-branded dataTypes; branded labelNutrients are per serving and are
	// rescaled to per-100 g when a serving size is known).
	Nutrients   map[string]Nutrient `json:"nutrients"`
	ServingSize float64             `json:"serving_size,omitempty"`
	ServingUnit string              `json:"serving_size_unit,omitempty"`
}

// Amount returns the per-100 g amount for a nutrient number, and whether it was
// present.
func (f Food) Amount(number string) (float64, bool) {
	n, ok := f.Nutrients[number]
	return n.Amount, ok
}

// Calories returns energy in kcal per 100 g, resolving across dataTypes:
// nutrient 208 (Branded, SR Legacy), then Atwater general (957), then Atwater
// specific (958) which Foundation foods use. Returns 0 only when none is present.
func (f Food) Calories() float64 {
	for _, num := range []string{NutrNumEnergyKcal, NutrNumEnergyAtwaterGeneral, NutrNumEnergyAtwaterSpecific} {
		if v, ok := f.Amount(num); ok && v > 0 {
			return v
		}
	}
	return 0
}

// Protein per 100 g.
func (f Food) Protein() float64 { v, _ := f.Amount(NutrNumProtein); return v }

// Fat per 100 g.
func (f Food) Fat() float64 { v, _ := f.Amount(NutrNumFat); return v }

// Carbs per 100 g.
func (f Food) Carbs() float64 { v, _ := f.Amount(NutrNumCarb); return v }

// Fiber per 100 g.
func (f Food) Fiber() float64 { v, _ := f.Amount(NutrNumFiber); return v }

// Normalize decodes a single USDA food JSON object (from /v1/food/{fdcId},
// an element of /v1/foods, or a foods search result) into a Food. It accepts
// both the foodNutrients[] and labelNutrients shapes.
func Normalize(raw json.RawMessage) (Food, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return Food{}, err
	}
	f := Food{Nutrients: map[string]Nutrient{}}
	f.FdcID = intField(m, "fdcId")
	f.Description = stringField(m, "description")
	f.DataType = stringField(m, "dataType")
	f.BrandOwner = stringField(m, "brandOwner")
	f.PublishedAt = firstNonEmpty(stringField(m, "publishedDate"), stringField(m, "publicationDate"))
	f.ServingSize = floatField(m, "servingSize")
	f.ServingUnit = stringField(m, "servingSizeUnit")

	if fn, ok := m["foodNutrients"]; ok {
		parseFoodNutrients(fn, f.Nutrients)
	}
	// Branded foods often carry labelNutrients (per serving) in addition to,
	// or instead of, foodNutrients. Only fold them in for numbers we did not
	// already get per-100 g, rescaling serving -> 100 g when possible.
	if ln, ok := m["labelNutrients"]; ok {
		parseLabelNutrients(ln, f)
	}
	return f, nil
}

func parseFoodNutrients(raw json.RawMessage, out map[string]Nutrient) {
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return
	}
	for _, item := range arr {
		var number, name, unit string
		var amount float64
		// Shape A (full): {"nutrient":{"number":"203","name":"Protein","unitName":"G"},"amount":21.4}
		if nutRaw, ok := item["nutrient"]; ok {
			var nut map[string]json.RawMessage
			if json.Unmarshal(nutRaw, &nut) == nil {
				number = stringField(nut, "number")
				name = stringField(nut, "name")
				unit = stringField(nut, "unitName")
			}
			amount = floatField(item, "amount")
		} else {
			// Shape B (abridged /foods/list): {"number":"203","name":"Protein","unitName":"G","amount":21.4}
			// Shape C (/foods/search):        {"nutrientNumber":"203","nutrientName":"Protein","unitName":"G","value":21.4}
			number = firstNonEmpty(stringField(item, "number"), stringField(item, "nutrientNumber"))
			name = firstNonEmpty(stringField(item, "name"), stringField(item, "nutrientName"))
			unit = stringField(item, "unitName")
			amount = floatField(item, "amount")
			if amount == 0 {
				amount = floatField(item, "value")
			}
		}
		if number == "" {
			continue
		}
		out[number] = Nutrient{Number: number, Name: name, Amount: amount, Unit: strings.ToLower(unit)}
	}
}

// labelNutrientMap maps USDA labelNutrients keys to canonical nutrient numbers.
var labelNutrientMap = map[string]struct {
	number string
	name   string
	unit   string
}{
	"calories":      {NutrNumEnergyKcal, "Energy", "kcal"},
	"protein":       {NutrNumProtein, "Protein", "g"},
	"fat":           {NutrNumFat, "Total lipid (fat)", "g"},
	"carbohydrates": {NutrNumCarb, "Carbohydrate, by difference", "g"},
	"fiber":         {NutrNumFiber, "Fiber, total dietary", "g"},
	"sugars":        {NutrNumSugars, "Sugars, total", "g"},
	"sodium":        {NutrNumSodium, "Sodium, Na", "mg"},
	"cholesterol":   {NutrNumCholest, "Cholesterol", "mg"},
	"saturatedFat":  {NutrNumSatFat, "Fatty acids, total saturated", "g"},
}

func parseLabelNutrients(raw json.RawMessage, f Food) {
	var m map[string]map[string]float64
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	// Rescale from per-serving to per-100 g only when the serving size is a
	// gram value. If the serving is in ml (or unknown), we cannot express these
	// per 100 g, and downstream commands (compare/meal) treat all stored
	// amounts as per-100 g — so storing raw per-serving values would silently
	// mis-scale. Skip label nutrients entirely in that case rather than lie
	// about the basis.
	if f.ServingSize <= 0 || !strings.EqualFold(f.ServingUnit, "g") {
		return
	}
	scale := 100.0 / f.ServingSize
	for key, canon := range labelNutrientMap {
		if _, present := f.Nutrients[canon.number]; present {
			continue
		}
		v, ok := m[key]
		if !ok {
			continue
		}
		amt, ok := v["value"]
		if !ok {
			continue
		}
		f.Nutrients[canon.number] = Nutrient{
			Number: canon.number,
			Name:   canon.name,
			Amount: amt * scale,
			Unit:   canon.unit,
		}
	}
}

func stringField(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

func intField(m map[string]json.RawMessage, key string) int {
	raw, ok := m[key]
	if !ok {
		return 0
	}
	var i int
	if json.Unmarshal(raw, &i) == nil {
		return i
	}
	// Some responses encode ids as strings.
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n
		}
	}
	return 0
}

func floatField(m map[string]json.RawMessage, key string) float64 {
	raw, ok := m[key]
	if !ok {
		return 0
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return v
		}
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
