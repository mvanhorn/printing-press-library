// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package source defines the aggregator contract for the nutrition CLI's two
// peer data sources (USDA FoodData Central and NutritionValue.org). It exposes
// a small registry so the `sources` command tree can describe what is wired in
// without importing each source's implementation package directly.
package source

import "sort"

// Source describes a registered upstream data source.
type Source struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	AuthRequired bool   `json:"auth_required"`
	AuthNote     string `json:"auth_note,omitempty"`
	BaseURL      string `json:"base_url"`
}

var registry = map[string]Source{}

// Register records a source. Called from package init() of each source.
func Register(s Source) { registry[s.Name] = s }

// All returns the registered sources sorted by name.
func All() []Source {
	out := make([]Source, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup returns a source by name.
func Lookup(name string) (Source, bool) {
	s, ok := registry[name]
	return s, ok
}

func init() {
	Register(Source{
		Name:         "usda",
		Description:  "USDA FoodData Central: ~600K foods across Foundation, SR Legacy, Survey (FNDDS), and Branded datasets. Official REST API.",
		AuthRequired: true,
		AuthNote:     "Free api.data.gov key via FDC_API_KEY or USDA_API_KEY; falls back to public DEMO_KEY (rate-limited ~30/hr).",
		BaseURL:      "https://api.nal.usda.gov/fdc",
	})
	Register(Source{
		Name:         "nutritionvalue",
		Description:  "NutritionValue.org: USDA-derived analytics the FDC API does not expose (net carbs, omega-6/omega-3 ratio, per-nutrient %DV) and precomputed nutrient rankings. Server-rendered HTML.",
		AuthRequired: false,
		AuthNote:     "No key required.",
		BaseURL:      "https://www.nutritionvalue.org",
	})
}
