// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

// TestHaversineKm pins the math against well-known distance pairs.
// Allows ~1% tolerance because the haversine formula assumes a
// spherical Earth (the real shape is an oblate spheroid).
func TestHaversineKm(t *testing.T) {
	cases := []struct {
		name         string
		lat1, lng1   float64
		lat2, lng2   float64
		wantKm       float64
		tolerancePct float64
	}{
		// Seattle ↔ Bellevue WA — the exact failure case from #406.
		// Reference: 13 km via straight-line distance.
		{"seattle ↔ bellevue WA", 47.6062, -122.3321, 47.6101, -122.2015, 9.8, 5.0},
		// Seattle ↔ NYC — 3850 km. Verifies sign handling and long-range
		// accuracy.
		{"seattle ↔ nyc", 47.6062, -122.3321, 40.7589, -73.9851, 3850, 1.0},
		// SF ↔ LA — 558 km.
		{"sf ↔ la", 37.7749, -122.4194, 34.0522, -118.2437, 558, 1.0},
		// Same point — 0 km.
		{"identity", 47.6062, -122.3321, 47.6062, -122.3321, 0, 0.1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := haversineKm(tc.lat1, tc.lng1, tc.lat2, tc.lng2)
			if tc.tolerancePct == 0 {
				if got != tc.wantKm {
					t.Errorf("got %.3f km; want exactly %.3f km", got, tc.wantKm)
				}
				return
			}
			tol := tc.wantKm * tc.tolerancePct / 100.0
			if tc.wantKm == 0 {
				tol = 0.001
			}
			if math.Abs(got-tc.wantKm) > tol {
				t.Errorf("got %.3f km; want %.3f km ±%.2f%%", got, tc.wantKm, tc.tolerancePct)
			}
		})
	}
}

// TestInferMetroFromSlug_DEPRECATED_ExactMatch covers the typical
// case from #406: agent composes `joey-bellevue` and we need to peel
// the `bellevue` suffix as the metro hint. Renamed in U5; the helper
// is slated for replacement once U7/U8 routes slug-suffix venues
// through ResolveLocation.
//
// U13: the dynamic-only Tock "bellevue" slug used by this test used
// to be reachable via Lookup("bellevue"); after U13's merge it is
// absorbed into curated `bellevue-wa` (name+coords match within 5
// km). The test now exercises the slug-suffix peel against entries
// that survive the merge: curated `bellevue-wa`, `new-york-city`, and
// `san-francisco` (via alias `sf`). The `joey-bellevue-wa` form is
// the agent-facing canonical shape the curated table answers to.
func TestInferMetroFromSlug_DEPRECATED_ExactMatch(t *testing.T) {
	defer setDynamicMetros(nil, 0)
	setDynamicMetros(nil, 0)

	reg := getRegistry()
	cases := []struct {
		input      string
		wantMetro  string
		wantPrefix string
	}{
		// Curated `bellevue-wa` is a 2-token suffix.
		{"joey-bellevue-wa", "bellevue-wa", "joey"},
		{"13-coins-bellevue-wa", "bellevue-wa", "13-coins"},
		{"daniels-broiler-bellevue-wa", "bellevue-wa", "daniels-broiler"},
		// Multi-token suffixes: "new-york-city" must beat "city" or "york".
		{"katz-new-york-city", "new-york-city", "katz"},
		// Alias (sf) as suffix → resolves via alias chain.
		{"tartine-sf", "san-francisco", "tartine"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			m, prefix, ok := inferMetroFromSlug_DEPRECATED(tc.input, reg)
			if !ok {
				t.Fatalf("expected match for %q; got !ok", tc.input)
			}
			if m.Slug != tc.wantMetro {
				t.Errorf("metro slug = %q; want %q", m.Slug, tc.wantMetro)
			}
			if prefix != tc.wantPrefix {
				t.Errorf("prefix = %q; want %q", prefix, tc.wantPrefix)
			}
		})
	}
}

// TestInferMetroFromSlug_DEPRECATED_NoMatch verifies we don't
// false-positive on slugs that happen to end in a token resembling a
// city. Agents using `wild-ginger` (no city suffix) should NOT trigger
// inference.
func TestInferMetroFromSlug_DEPRECATED_NoMatch(t *testing.T) {
	reg := getRegistry()
	cases := []string{
		"wild-ginger", // bare venue name, no city suffix
		"canlis",      // single-token slug
		"foo-bar-baz", // no token matches any metro
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			_, prefix, ok := inferMetroFromSlug_DEPRECATED(in, reg)
			if ok {
				t.Errorf("inferMetroFromSlug_DEPRECATED(%q) returned ok=true unexpectedly", in)
			}
			if prefix != in {
				t.Errorf("prefix on no-match should be input %q; got %q", in, prefix)
			}
		})
	}
}

// bellevueCtx is the shared GeoContext used across the post-filter
// tests. Pinned to Bellevue WA so the assertions match the original
// #406 fixture exactly.
func bellevueCtx() *GeoContext {
	return &GeoContext{
		Origin:     "bellevue",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   50.0,
	}
}

// TestApplyGeoFilter_HardReject verifies hard-reject mode drops
// results beyond the radius. The user's `joey-bellevue` case from
// #406 maps to: query for Bellevue venues, get a Tampa result back,
// drop it.
func TestApplyGeoFilter_HardReject(t *testing.T) {
	results := []goatResult{
		// Real Bellevue venue
		{Name: "Daniel's Broiler - Bellevue", Latitude: 47.6181, Longitude: -122.2007, MatchScore: 0.95},
		// JOEY Bellevue (real Bellevue, slightly outside city center)
		{Name: "JOEY Bellevue", Latitude: 47.6149, Longitude: -122.1959, MatchScore: 0.95},
		// Tampa, FL — the #406 wrong-city match
		{Name: "Joey's Bold Flavors", Latitude: 27.9506, Longitude: -82.4572, MatchScore: 0.65},
		// NYC — another wrong-city
		{Name: "Wildair", Latitude: 40.7128, Longitude: -74.0060, MatchScore: 0.65},
	}
	got := applyGeoFilter(results, bellevueCtx(), metroFilterHardReject)
	if len(got) != 2 {
		t.Fatalf("got %d in-radius results; want 2", len(got))
	}
	names := []string{got[0].Name, got[1].Name}
	for _, want := range []string{"Daniel's Broiler - Bellevue", "JOEY Bellevue"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("real Bellevue venue %q dropped; results: %v", want, names)
		}
	}
	// Verify distance is annotated on kept rows.
	if got[0].MetroCentroidDistanceKm <= 0 {
		t.Errorf("MetroCentroidDistanceKm should be set on kept rows; got %v", got[0].MetroCentroidDistanceKm)
	}
}

// TestApplyGeoFilter_SoftDemote verifies soft-demote mode keeps far
// results but slashes their match_score so they sort to the bottom.
// Issue #406 brainstorm: this is the inferred-metro path — we don't
// know for sure the user meant Bellevue, so we keep the results
// visible but make the geo mismatch loud in the score.
func TestApplyGeoFilter_SoftDemote(t *testing.T) {
	results := []goatResult{
		{Name: "JOEY Bellevue", Latitude: 47.6149, Longitude: -122.1959, MatchScore: 0.95},
		{Name: "Joey's Bold Flavors (Tampa)", Latitude: 27.9506, Longitude: -82.4572, MatchScore: 0.65},
	}
	got := applyGeoFilter(results, bellevueCtx(), metroFilterSoftDemote)
	if len(got) != 2 {
		t.Fatalf("got %d results; want 2 (no drops in soft-demote)", len(got))
	}
	if got[1].MatchScore >= 0.5 {
		t.Errorf("far result score = %.3f; want demoted (well below 0.5)", got[1].MatchScore)
	}
	if got[0].MatchScore != 0.95 {
		t.Errorf("near result score should be untouched; got %.3f", got[0].MatchScore)
	}
}

// TestApplyGeoFilter_PreservesNoLatLngRows verifies results missing
// lat/lng aren't dropped — we can't make a geo judgement on missing
// data. Common for newly-listed Tock venues.
func TestApplyGeoFilter_PreservesNoLatLngRows(t *testing.T) {
	results := []goatResult{
		{Name: "Venue with no geo", Latitude: 0, Longitude: 0, MatchScore: 0.95},
	}
	got := applyGeoFilter(results, bellevueCtx(), metroFilterHardReject)
	if len(got) != 1 {
		t.Errorf("no-geo row dropped; got %d rows", len(got))
	}
}

// TestApplyGeoFilter_OffMode verifies metroFilterOff is a true pass-
// through — no row mutation, no drops.
func TestApplyGeoFilter_OffMode(t *testing.T) {
	results := []goatResult{
		{Name: "X", Latitude: 1, Longitude: 1, MatchScore: 0.95},
		{Name: "Y", Latitude: 50, Longitude: 50, MatchScore: 0.4},
	}
	got := applyGeoFilter(results, bellevueCtx(), metroFilterOff)
	if len(got) != 2 {
		t.Errorf("off mode should preserve all rows; got %d", len(got))
	}
	if got[0].MetroCentroidDistanceKm != 0 || got[1].MetroCentroidDistanceKm != 0 {
		t.Error("off mode should NOT annotate distance")
	}
}

// TestApplyGeoFilter_NilContext verifies the U5 nil-ctx invariant:
// passing a nil *GeoContext yields a true pass-through, the same
// shape callers get when --location is not set (R13).
func TestApplyGeoFilter_NilContext(t *testing.T) {
	results := []goatResult{
		{Name: "X", Latitude: 1, Longitude: 1, MatchScore: 0.95},
		{Name: "Y", Latitude: 50, Longitude: 50, MatchScore: 0.4},
	}
	got := applyGeoFilter(results, nil, metroFilterHardReject)
	if len(got) != 2 {
		t.Errorf("nil ctx should preserve all rows; got %d", len(got))
	}
	if got[0].MetroCentroidDistanceKm != 0 || got[1].MetroCentroidDistanceKm != 0 {
		t.Error("nil ctx should NOT annotate distance")
	}
}
