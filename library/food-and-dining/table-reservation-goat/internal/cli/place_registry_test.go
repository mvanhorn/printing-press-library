// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// TestStaticRegistry_Lookup pins the curated registry's Slug + alias
// resolution. The same casing/trim tolerance applies as the pre-U3
// Metro registry — aliases are case-insensitive after trim.
func TestStaticRegistry_Lookup(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		input    string
		wantSlug string
	}{
		{"seattle", "seattle"},
		{"Seattle", "seattle"},     // case insensitive
		{"  seattle  ", "seattle"}, // whitespace tolerated
		{"sf", "san-francisco"},
		{"SF", "san-francisco"},
		{"nyc", "new-york-city"},
		{"new-york", "new-york-city"},
		// U17: "manhattan" used to be an alias of new-york-city but now
		// has its own borough entry (10 km radius). The slug-then-alias
		// order in lookupIn means the dedicated entry wins.
		{"manhattan", "manhattan"},
		{"la", "los-angeles"},
		{"bellevue-wa", "bellevue-wa"},
		{"bellevue-ne", "bellevue-ne"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			p, ok := r.Lookup(tc.input)
			if !ok {
				t.Fatalf("Lookup(%q) !ok; want slug %q", tc.input, tc.wantSlug)
			}
			if p.Slug != tc.wantSlug {
				t.Errorf("Lookup(%q).Slug = %q; want %q", tc.input, p.Slug, tc.wantSlug)
			}
			if p.Lat == 0 && p.Lng == 0 {
				t.Errorf("Lookup(%q) centroid zero", tc.input)
			}
			if p.Name == "" {
				t.Errorf("Lookup(%q) Name empty", tc.input)
			}
			if p.RadiusKm <= 0 {
				t.Errorf("Lookup(%q) RadiusKm not set: %v", tc.input, p.RadiusKm)
			}
		})
	}
}

// TestStaticRegistry_PopulationPopulated guards against accidentally
// dropping the Population field. R14 fixtures use Seattle as a known
// large-population reference; >700k pins us to the curated value
// (753675) without coupling the test to the exact figure.
func TestStaticRegistry_PopulationPopulated(t *testing.T) {
	r := staticPlaceRegistry{}
	p, ok := r.Lookup("seattle")
	if !ok {
		t.Fatal("seattle missing from curated registry")
	}
	if p.Population <= 700000 {
		t.Errorf("seattle Population = %d; want > 700000", p.Population)
	}
	if p.Name != "Seattle" {
		t.Errorf("seattle Name = %q; want %q", p.Name, "Seattle")
	}
}

// TestStaticRegistry_LookupEmpty verifies the empty / whitespace
// input path returns (zero, false) rather than the first registry
// entry. Issue-#406 callers' "did you mean" UX depends on this
// signal.
func TestStaticRegistry_LookupEmpty(t *testing.T) {
	r := staticPlaceRegistry{}
	for _, in := range []string{"", "  ", "made-up-slug-xyz"} {
		if _, ok := r.Lookup(in); ok {
			t.Errorf("Lookup(%q) returned ok=true", in)
		}
	}
}

// TestStaticRegistry_LookupByName_Bellevue verifies the
// ambiguous-name fixture: three Bellevues (WA, NE, KY) must all come
// back regardless of casing.
func TestStaticRegistry_LookupByName_Bellevue(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []string{"bellevue", "Bellevue", "BELLEVUE", "  bellevue  "}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			hits, ok := r.LookupByName(in)
			if !ok {
				t.Fatalf("LookupByName(%q) !ok", in)
			}
			gotStates := make([]string, 0, len(hits))
			for _, p := range hits {
				gotStates = append(gotStates, p.State)
			}
			sort.Strings(gotStates)
			wantStates := []string{"KY", "NE", "WA"}
			if !slices.Equal(gotStates, wantStates) {
				t.Errorf("Bellevue states = %v; want %v", gotStates, wantStates)
			}
		})
	}
}

// TestStaticRegistry_LookupByName_OtherAmbiguous covers the rest of
// the R14 ambiguous-name fixture set. Each case asserts the expected
// state list order-independently.
func TestStaticRegistry_LookupByName_OtherAmbiguous(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		query      string
		wantStates []string
	}{
		{"portland", []string{"ME", "OR"}},
		{"springfield", []string{"IL", "MA", "MO", "OR"}},
		{"columbia", []string{"MD", "MO", "SC"}},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			hits, ok := r.LookupByName(tc.query)
			if !ok {
				t.Fatalf("LookupByName(%q) !ok", tc.query)
			}
			gotStates := make([]string, 0, len(hits))
			for _, p := range hits {
				gotStates = append(gotStates, p.State)
			}
			sort.Strings(gotStates)
			if !slices.Equal(gotStates, tc.wantStates) {
				t.Errorf("%s states = %v; want %v", tc.query, gotStates, tc.wantStates)
			}
		})
	}
}

// TestStaticRegistry_LookupByName_None verifies the empty + unknown
// paths.
func TestStaticRegistry_LookupByName_None(t *testing.T) {
	r := staticPlaceRegistry{}
	for _, in := range []string{"", "  ", "nonexistent"} {
		t.Run("empty_"+in, func(t *testing.T) {
			hits, ok := r.LookupByName(in)
			if ok || hits != nil {
				t.Errorf("LookupByName(%q) = (%v, %v); want (nil, false)", in, hits, ok)
			}
		})
	}
}

// TestStaticRegistry_AliasResolution covers the alias chain for
// canonical slugs. NYC → New York City and SF → San Francisco are
// the highest-traffic shorthands.
func TestStaticRegistry_AliasResolution(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		input    string
		wantName string
	}{
		{"nyc", "New York City"},
		{"sf", "San Francisco"},
		{"la", "Los Angeles"},
		{"new-york", "New York City"},
		// U17: "manhattan" was an alias of NYC pre-U17; it now resolves
		// to the dedicated borough entry.
		{"manhattan", "Manhattan"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			p, ok := r.Lookup(tc.input)
			if !ok {
				t.Fatalf("Lookup(%q) !ok", tc.input)
			}
			if p.Name != tc.wantName {
				t.Errorf("Lookup(%q).Name = %q; want %q", tc.input, p.Name, tc.wantName)
			}
		})
	}
}

// TestStaticRegistry_ReverseLookup covers radius containment + the
// city-beats-metro tiebreak.
//
// Math sanity (curated coords + 25 km / 75 km radii):
//
//   - Bellevue-WA centroid (47.6101, -122.2015) → inside Bellevue's
//     own 25 km (dist=0) AND inside Seattle's 75 km (~9.8 km
//     centroid-to-centroid). Smallest RadiusKm wins → Bellevue WA.
//
//   - West-of-Bainbridge (47.6262, -122.65) → ~24 km west of
//     Seattle's centroid (inside Seattle's 75 km) and ~33 km from
//     Bellevue's centroid (outside Bellevue's 25 km). Only Seattle
//     qualifies → Seattle. Picked further west than the Bainbridge
//     centroid itself because Bellevue's 25 km radius just barely
//     reaches Bainbridge proper (~24 km from Bellevue centroid).
//
//   - Space Needle (47.6205, -122.3493) is intentionally NOT used —
//     it sits ~11 km from Bellevue's centroid (inside Bellevue's
//     25 km radius), so the tiebreak picks Bellevue, not Seattle.
func TestStaticRegistry_ReverseLookup(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		name     string
		lat, lng float64
		wantSlug string
	}{
		{"bellevue-centroid", 47.6101, -122.2015, "bellevue-wa"},
		{"west-of-bainbridge", 47.6262, -122.65, "seattle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := r.ReverseLookup(tc.lat, tc.lng)
			if !ok {
				t.Fatalf("ReverseLookup(%v, %v) !ok", tc.lat, tc.lng)
			}
			if p.Slug != tc.wantSlug {
				t.Errorf("ReverseLookup(%v, %v) = %q; want %q", tc.lat, tc.lng, p.Slug, tc.wantSlug)
			}
		})
	}
}

// TestStaticRegistry_ReverseLookup_NoMatch verifies that points
// outside every registry radius return (zero, false). (0, 0) is in
// the Atlantic — well clear of every curated US place.
func TestStaticRegistry_ReverseLookup_NoMatch(t *testing.T) {
	r := staticPlaceRegistry{}
	p, ok := r.ReverseLookup(0, 0)
	if ok {
		t.Errorf("ReverseLookup(0,0) = (%+v, true); want (Place{}, false)", p)
	}
	if p.Slug != "" {
		t.Errorf("zero-result Slug = %q; want empty", p.Slug)
	}
}

// TestReverseLookup_RadiusTiebreak constructs a synthetic 3-place
// scenario where the lookup point is inside two overlapping radii.
// Verifies the smallest RadiusKm wins regardless of input order or
// haversine distance.
func TestReverseLookup_RadiusTiebreak(t *testing.T) {
	// Same centroid, three radii. The smallest-radius Place must win.
	pts := []Place{
		{Slug: "big", Name: "Big", Lat: 40.0, Lng: -100.0, RadiusKm: 100, Tier: PlaceTierMetroCentroid},
		{Slug: "med", Name: "Med", Lat: 40.0, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
		{Slug: "small", Name: "Small", Lat: 40.0, Lng: -100.0, RadiusKm: 10, Tier: PlaceTierNeighborhood},
	}
	p, ok := reverseLookupIn(pts, 40.0, -100.0)
	if !ok {
		t.Fatal("reverseLookupIn !ok for in-radius point")
	}
	if p.Slug != "small" {
		t.Errorf("tiebreak Slug = %q; want %q (smallest RadiusKm)", p.Slug, "small")
	}
}

// TestReverseLookup_DistanceTiebreak verifies the secondary tiebreak:
// equal RadiusKm, smaller haversine distance wins.
func TestReverseLookup_DistanceTiebreak(t *testing.T) {
	pts := []Place{
		// Both have RadiusKm=50. Lookup point is at (40, -100).
		// "far" centroid is at (40.3, -100) — about 33 km away.
		// "near" centroid is at (40.1, -100) — about 11 km away.
		// Same RadiusKm so the distance tiebreak picks "near".
		{Slug: "far", Name: "Far", Lat: 40.3, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
		{Slug: "near", Name: "Near", Lat: 40.1, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
	}
	p, ok := reverseLookupIn(pts, 40.0, -100.0)
	if !ok {
		t.Fatal("reverseLookupIn !ok")
	}
	if p.Slug != "near" {
		t.Errorf("distance tiebreak Slug = %q; want %q", p.Slug, "near")
	}
}

// TestReverseLookup_AlphaTiebreak verifies the tertiary tiebreak:
// equal RadiusKm + equal distance, alphabetical Slug wins.
func TestReverseLookup_AlphaTiebreak(t *testing.T) {
	pts := []Place{
		{Slug: "zebra", Name: "Z", Lat: 40.0, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
		{Slug: "alpha", Name: "A", Lat: 40.0, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
		{Slug: "mango", Name: "M", Lat: 40.0, Lng: -100.0, RadiusKm: 50, Tier: PlaceTierCity},
	}
	p, ok := reverseLookupIn(pts, 40.0, -100.0)
	if !ok {
		t.Fatal("reverseLookupIn !ok")
	}
	if p.Slug != "alpha" {
		t.Errorf("alpha tiebreak Slug = %q; want %q", p.Slug, "alpha")
	}
}

// TestChainedRegistry_SlugMatchEnrichesNotOverrides — U13 merge
// semantics. A dynamic entry whose slug matches a curated entry must
// ENRICH (overwrite only the provider-coverage / parent-metro fields
// from live data); it must NOT replace the curated Name, State,
// Population, Lat, Lng, RadiusKm, Tier, Aliases, ContextHints. Codex
// adversarial review P1-C: chain-override let dynamic Bellevue (no
// State, 75 km radius, no ContextHints) shadow curated Bellevue WA
// (State="WA", 25 km radius, ContextHints=["Seattle metro",...]).
func TestChainedRegistry_SlugMatchEnrichesNotOverrides(t *testing.T) {
	dyn := Place{
		Slug:             "seattle",
		Name:             "Seattle (dyn)",
		Lat:              47.7,
		Lng:              -122.4,
		RadiusKm:         75,
		Tier:             PlaceTierMetroCentroid,
		ProviderCoverage: map[string]int{"tock": 120},
		ParentMetro:      map[string]string{"tock": "seattle"},
	}
	chain := chainedPlaceRegistry{merged: mergeRegistry(curatedPlaces, []Place{dyn})}
	p, ok := chain.Lookup("seattle")
	if !ok {
		t.Fatal("seattle Lookup !ok")
	}
	// Curated fields must survive — dynamic doesn't get to overwrite
	// Name/centroid/Population/State.
	if p.Name != "Seattle" {
		t.Errorf("Name = %q; want curated %q (dynamic must not overwrite)", p.Name, "Seattle")
	}
	if p.State != "WA" {
		t.Errorf("State = %q; want curated %q", p.State, "WA")
	}
	if p.Lat != 47.6062 || p.Lng != -122.3321 {
		t.Errorf("centroid = %v,%v; want curated 47.6062,-122.3321 (dynamic must not overwrite)", p.Lat, p.Lng)
	}
	if p.Population <= 700000 {
		t.Errorf("Population = %d; want curated >700000 preserved", p.Population)
	}
	// Provider-coverage hints must be enriched from dynamic.
	if p.ProviderCoverage["tock"] != 120 {
		t.Errorf("ProviderCoverage[tock] = %d; want 120 (enriched from dynamic)", p.ProviderCoverage["tock"])
	}
	if p.ParentMetro["tock"] != "seattle" {
		t.Errorf("ParentMetro[tock] = %q; want %q (enriched from dynamic)", p.ParentMetro["tock"], "seattle")
	}
}

// TestChainedRegistry_StaticFallback verifies entries the dynamic
// source doesn't cover still resolve via the curated fallback.
func TestChainedRegistry_StaticFallback(t *testing.T) {
	chain := chainedPlaceRegistry{merged: mergeRegistry(curatedPlaces, []Place{
		{Slug: "tock-only-metro", Name: "Tock Only", Lat: 1, Lng: 1, RadiusKm: 75},
	})}
	if p, ok := chain.Lookup("chicago"); !ok || p.Slug != "chicago" {
		t.Errorf("chicago should fall through to curated; got (%+v, %v)", p, ok)
	}
	if p, ok := chain.Lookup("tock-only-metro"); !ok || p.Slug != "tock-only-metro" {
		t.Errorf("tock-only-metro should resolve from dynamic; got (%+v, %v)", p, ok)
	}
}

// TestChainedRegistry_All verifies the merged All() shape: a curated
// entry shadowed by a dynamic same-slug match appears exactly once
// (enriched in place), dynamic-only entries appear separately, and
// every curated entry that wasn't matched stays present.
func TestChainedRegistry_All(t *testing.T) {
	chain := chainedPlaceRegistry{merged: mergeRegistry(curatedPlaces, []Place{
		{Slug: "tock-x", Name: "Tock X", Lat: 1, Lng: 1, RadiusKm: 75},
		{Slug: "seattle", Name: "Seattle (dyn)", Lat: 47.6, Lng: -122.3, RadiusKm: 75,
			ProviderCoverage: map[string]int{"tock": 99}},
	})}
	all := chain.All()
	seenSeattle := 0
	for _, p := range all {
		if p.Slug == "seattle" {
			seenSeattle++
			// Curated Name wins on slug-match enrichment.
			if p.Name != "Seattle" {
				t.Errorf("seattle Name = %q; want curated %q", p.Name, "Seattle")
			}
			if p.ProviderCoverage["tock"] != 99 {
				t.Errorf("seattle ProviderCoverage[tock] = %d; want 99 enriched", p.ProviderCoverage["tock"])
			}
		}
	}
	if seenSeattle != 1 {
		t.Errorf("seattle duplicated in chain.All(); count = %d", seenSeattle)
	}
	hasChicago := slices.ContainsFunc(all, func(p Place) bool { return p.Slug == "chicago" })
	if !hasChicago {
		t.Error("curated chicago missing from chain.All()")
	}
	hasTockX := slices.ContainsFunc(all, func(p Place) bool { return p.Slug == "tock-x" })
	if !hasTockX {
		t.Error("dynamic-only tock-x missing from chain.All()")
	}
}

// TestChainedRegistry_LookupByName_Union verifies that the merged
// registry surfaces ambiguous-name matches across both dynamic and
// curated sources. A dynamic entry whose slug matches a curated entry
// enriches in place; a dynamic entry with a truly new slug appears as
// a separate row.
func TestChainedRegistry_LookupByName_Union(t *testing.T) {
	chain := chainedPlaceRegistry{merged: mergeRegistry(curatedPlaces, []Place{
		// Dynamic enriching curated Springfield IL — same slug.
		// Curated centroid wins; dynamic's coverage enriches.
		{Slug: "springfield-il", Name: "Springfield", Lat: 39.8, Lng: -89.7, RadiusKm: 75,
			ProviderCoverage: map[string]int{"tock": 7}},
		// Dynamic new Springfield (NJ) — adds an alternate to the union.
		{Slug: "springfield-nj", Name: "Springfield", Lat: 40.7, Lng: -74.3, RadiusKm: 75},
	})}
	hits, ok := chain.LookupByName("springfield")
	if !ok {
		t.Fatal("LookupByName(springfield) !ok")
	}
	// Should have 5: springfield-il (enriched), springfield-nj
	// (dynamic-only), springfield-ma, springfield-mo, springfield-or.
	if len(hits) != 5 {
		t.Errorf("Springfield hit count = %d; want 5 (enriched + dynamic-only + curated)", len(hits))
	}
	// Verify dynamic enriched the curated springfield-il (curated Lat
	// 39.7817 wins; coverage from dynamic enriches).
	for _, p := range hits {
		if p.Slug == "springfield-il" {
			if p.Lat != 39.7817 {
				t.Errorf("springfield-il Lat = %v; want curated 39.7817 (dynamic must not overwrite)", p.Lat)
			}
			if p.ProviderCoverage["tock"] != 7 {
				t.Errorf("springfield-il ProviderCoverage[tock] = %d; want 7 enriched", p.ProviderCoverage["tock"])
			}
		}
	}
}

// TestMergeRegistry_SlugMatchEnriches — synthetic-static unit for the
// merge helper. Curated Bellevue WA has State="WA", Population=151854,
// ContextHints, RadiusKm=25. Dynamic carries the Tock projection shape
// (75 km radius, no State, no Population, no hints). Merge result:
// curated fields preserved, ProviderCoverage["tock"] and
// ParentMetro["tock"] enriched from dynamic.
func TestMergeRegistry_SlugMatchEnriches(t *testing.T) {
	static := []Place{
		{
			Slug:         "bellevue-wa",
			Name:         "Bellevue",
			State:        "WA",
			Lat:          47.6101,
			Lng:          -122.2015,
			RadiusKm:     25,
			Population:   151854,
			ContextHints: []string{"Seattle metro", "Eastside", "tech hub"},
			Tier:         PlaceTierCity,
		},
	}
	dynamic := []Place{
		{
			Slug:             "bellevue-wa",
			Name:             "Bellevue, WA",
			Lat:              47.6101,
			Lng:              -122.2015,
			RadiusKm:         75,
			Tier:             PlaceTierMetroCentroid,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1 (slug match must not add a row)", len(merged))
	}
	p, ok := lookupIn(merged, "bellevue-wa")
	if !ok {
		t.Fatal("bellevue-wa missing after merge")
	}
	// Curated fields preserved.
	if p.State != "WA" {
		t.Errorf("State = %q; want curated WA", p.State)
	}
	if p.Population != 151854 {
		t.Errorf("Population = %d; want curated 151854", p.Population)
	}
	if p.RadiusKm != 25 {
		t.Errorf("RadiusKm = %v; want curated 25 (NOT 75 from dynamic)", p.RadiusKm)
	}
	if p.Tier != PlaceTierCity {
		t.Errorf("Tier = %v; want curated PlaceTierCity", p.Tier)
	}
	if p.Name != "Bellevue" {
		t.Errorf("Name = %q; want curated %q", p.Name, "Bellevue")
	}
	if !slices.Equal(p.ContextHints, []string{"Seattle metro", "Eastside", "tech hub"}) {
		t.Errorf("ContextHints = %v; want curated set preserved", p.ContextHints)
	}
	// Enrichment from dynamic.
	if p.ProviderCoverage["tock"] != 42 {
		t.Errorf("ProviderCoverage[tock] = %d; want 42 enriched", p.ProviderCoverage["tock"])
	}
	if p.ParentMetro["tock"] != "bellevue" {
		t.Errorf("ParentMetro[tock] = %q; want %q enriched", p.ParentMetro["tock"], "bellevue")
	}
	// Source static slice must not have been mutated.
	if static[0].ProviderCoverage != nil {
		t.Error("mergeRegistry mutated the static source slice")
	}
}

// TestMergeRegistry_NameAndCoordsMatchEnriches — rule 2. Dynamic slug
// is different from any curated slug, but same Name (case-insensitive)
// and centroid within 5 km. Enrich the curated entry; do NOT add a new
// row.
func TestMergeRegistry_NameAndCoordsMatchEnriches(t *testing.T) {
	static := []Place{
		{Slug: "seattle", Name: "Seattle", State: "WA", Lat: 47.6062, Lng: -122.3321, RadiusKm: 75, Tier: PlaceTierMetroCentroid},
	}
	dynamic := []Place{
		// Different slug, same name (case-insensitive), ~1 km away.
		{Slug: "different-slug", Name: "Seattle", Lat: 47.61, Lng: -122.34, RadiusKm: 75,
			ProviderCoverage: map[string]int{"tock": 200},
			ParentMetro:      map[string]string{"tock": "different-slug"}},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1 (name+coords match must not add a row)", len(merged))
	}
	p := merged[0]
	if p.Slug != "seattle" {
		t.Errorf("Slug = %q; want curated %q preserved", p.Slug, "seattle")
	}
	if p.ProviderCoverage["tock"] != 200 {
		t.Errorf("ProviderCoverage[tock] = %d; want 200 enriched", p.ProviderCoverage["tock"])
	}
	if p.ParentMetro["tock"] != "different-slug" {
		t.Errorf("ParentMetro[tock] = %q; want %q enriched", p.ParentMetro["tock"], "different-slug")
	}
}

// TestMergeRegistry_TrulyNewAddsRow — rule 3. Dynamic slug doesn't
// match curated AND name+coords doesn't either. Entry is added as a
// separate dynamic-only row.
func TestMergeRegistry_TrulyNewAddsRow(t *testing.T) {
	static := []Place{
		{Slug: "seattle", Name: "Seattle", State: "WA", Lat: 47.6062, Lng: -122.3321, RadiusKm: 75, Tier: PlaceTierMetroCentroid},
		{Slug: "new-york-city", Name: "New York City", State: "NY", Lat: 40.7128, Lng: -74.0060, RadiusKm: 75, Tier: PlaceTierMetroCentroid},
	}
	dynamic := []Place{
		{
			Slug:             "louisville",
			Name:             "Louisville",
			Lat:              38.25,
			Lng:              -85.76,
			RadiusKm:         75,
			Tier:             PlaceTierMetroCentroid,
			ProviderCoverage: map[string]int{"tock": 8},
			ParentMetro:      map[string]string{"tock": "louisville"},
		},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 3 {
		t.Fatalf("merged len = %d; want 3 (truly new must add a row)", len(merged))
	}
	p, ok := lookupIn(merged, "louisville")
	if !ok {
		t.Fatal("louisville missing after merge")
	}
	if p.RadiusKm != 75 {
		t.Errorf("Louisville RadiusKm = %v; want dynamic 75 preserved", p.RadiusKm)
	}
	if p.ProviderCoverage["tock"] != 8 {
		t.Errorf("Louisville ProviderCoverage[tock] = %d; want 8", p.ProviderCoverage["tock"])
	}
}

// TestMergeRegistry_NameMatchOutsideRadius — name matches but coords
// are far apart. Rule 2 must NOT fire (the 5 km cutoff catches
// same-name-different-city cases like Bellevue WA vs Bellevue NE).
// Dynamic is added as a separate row.
func TestMergeRegistry_NameMatchOutsideRadius(t *testing.T) {
	static := []Place{
		{Slug: "bellevue-wa", Name: "Bellevue", State: "WA", Lat: 47.6101, Lng: -122.2015, RadiusKm: 25, Population: 151854, Tier: PlaceTierCity},
	}
	dynamic := []Place{
		// Name matches "Bellevue" but coords are Bellevue NE — far from WA.
		{Slug: "bellevue", Name: "Bellevue", Lat: 41.14, Lng: -95.91, RadiusKm: 75,
			ProviderCoverage: map[string]int{"tock": 5}},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 2 {
		t.Fatalf("merged len = %d; want 2 (name-only match outside 5km radius must NOT enrich)", len(merged))
	}
	// Curated Bellevue WA must be untouched (no ProviderCoverage set).
	p, ok := lookupIn(merged, "bellevue-wa")
	if !ok {
		t.Fatal("bellevue-wa missing after merge")
	}
	if p.ProviderCoverage != nil {
		t.Errorf("bellevue-wa ProviderCoverage = %v; want nil (out-of-radius dynamic must NOT enrich)", p.ProviderCoverage)
	}
	// Dynamic Bellevue (NE coords) added as separate row.
	if _, ok := lookupIn(merged, "bellevue"); !ok {
		t.Error("dynamic bellevue must appear as a separate row when out-of-radius")
	}
}

// TestMergeRegistry_NilDynamicCoverage — defensive case. Static lacks
// ProviderCoverage. Dynamic carries nil ProviderCoverage too. Merge
// must not panic; the resulting Place may have nil or empty
// ProviderCoverage but the merge runs to completion.
func TestMergeRegistry_NilDynamicCoverage(t *testing.T) {
	static := []Place{
		{Slug: "seattle", Name: "Seattle", State: "WA", Lat: 47.6062, Lng: -122.3321, RadiusKm: 75, Tier: PlaceTierMetroCentroid},
	}
	dynamic := []Place{
		{Slug: "seattle", Name: "Seattle", Lat: 47.6062, Lng: -122.3321, RadiusKm: 75,
			ProviderCoverage: nil,
			ParentMetro:      nil},
	}
	// Must not panic.
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1", len(merged))
	}
	// Curated entry intact.
	if merged[0].Slug != "seattle" || merged[0].State != "WA" {
		t.Errorf("seattle curated fields drifted after nil-coverage enrich: %+v", merged[0])
	}
}

// TestMergeRegistry_HydrationPath — integration. Hand-built tock
// MetroArea slice through projectTockMetros → mergeRegistry against
// the real curated table. Curated Bellevue WA keeps its State="WA"
// and gets enriched (matched by name+coords since dynamic slug is
// "bellevue"). A brand-new metro appears separately.
func TestMergeRegistry_HydrationPath(t *testing.T) {
	in := []tock.MetroArea{
		// Tock's "bellevue" — same coords as curated Bellevue WA. The
		// projection makes Slug="bellevue" (different from curated
		// "bellevue-wa"), so this exercises the name+coords rule.
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6101, Lng: -122.2015, BusinessCount: 42},
		// Brand-new metro Tock surfaces that curated doesn't have.
		{Slug: "providence", Name: "Providence", Lat: 41.824, Lng: -71.4128, BusinessCount: 17},
	}
	projected := projectTockMetros(in)
	merged := mergeRegistry(curatedPlaces, projected)

	// Curated Bellevue WA must keep its State and gain Tock coverage.
	bellevueWA, ok := lookupIn(merged, "bellevue-wa")
	if !ok {
		t.Fatal("bellevue-wa missing after merge")
	}
	if bellevueWA.State != "WA" {
		t.Errorf("bellevue-wa State = %q; want WA", bellevueWA.State)
	}
	if bellevueWA.ProviderCoverage["tock"] != 42 {
		t.Errorf("bellevue-wa ProviderCoverage[tock] = %d; want 42 (enriched)", bellevueWA.ProviderCoverage["tock"])
	}
	// A 4th separate "bellevue" row must NOT appear — it should have
	// enriched curated bellevue-wa instead.
	bellevueHits, _ := lookupByNameIn(merged, "bellevue")
	if len(bellevueHits) != 3 {
		t.Errorf("Bellevue rows in merged = %d; want 3 (WA/NE/KY, no dynamic-only row)", len(bellevueHits))
	}
	// Brand-new providence appears separately.
	if _, ok := lookupIn(merged, "providence"); !ok {
		t.Error("providence missing after merge (truly new must add)")
	}
}

// TestMergeRegistry_DecideTier_Smoke — after hydration with a dynamic
// "Bellevue" that name+coord-matches curated Bellevue WA, the by-name
// lookup still returns the 3 curated Bellevues. The merge must not
// drop any of them.
func TestMergeRegistry_DecideTier_Smoke(t *testing.T) {
	defer setDynamicMetros(nil, 0)
	in := []tock.MetroArea{
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6101, Lng: -122.2015, BusinessCount: 42},
	}
	setDynamicMetros(projectTockMetros(in), 1)
	hits, ok := getRegistry().LookupByName("bellevue")
	if !ok {
		t.Fatal("LookupByName(bellevue) !ok after hydration")
	}
	if len(hits) != 3 {
		t.Errorf("Bellevue hits after hydration = %d; want 3 (WA/NE/KY preserved)", len(hits))
	}
	gotStates := make([]string, 0, len(hits))
	for _, p := range hits {
		gotStates = append(gotStates, p.State)
	}
	sort.Strings(gotStates)
	if !slices.Equal(gotStates, []string{"KY", "NE", "WA"}) {
		t.Errorf("Bellevue states = %v; want [KY NE WA]", gotStates)
	}
}

// TestSetDynamicMetros_Concurrency verifies the registry singleton
// upgrades under racing goroutines without panicking. Last writer
// wins — the assertion is "post-race lookup succeeds," not "specific
// goroutine won."
func TestSetDynamicMetros_Concurrency(t *testing.T) {
	defer setDynamicMetros(nil, 0)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			setDynamicMetros([]Place{
				{Slug: "test-place", Name: "Test", Lat: float64(i), Lng: float64(i), RadiusKm: 50},
			}, int64(i))
		}(i)
	}
	wg.Wait()

	if _, ok := getRegistry().Lookup("test-place"); !ok {
		t.Error("post-race lookup failed; race may have corrupted the registry")
	}
}

// TestMetroLatLng_LegacyShape verifies the (lat, lng, ok) legacy
// wrapper still works for any pre-U3 callers that haven't migrated.
func TestMetroLatLng_LegacyShape(t *testing.T) {
	lat, lng, ok := metroLatLng("seattle")
	if !ok || lat == 0 || lng == 0 {
		t.Errorf("legacy wrapper broken: (%v, %v, %v)", lat, lng, ok)
	}
	_, _, ok = metroLatLng("nonexistent-place")
	if ok {
		t.Error("legacy wrapper should report ok=false on unknown slug")
	}
}

// TestKnownMetros_SnapshotIncludesMajors guards the curated baseline.
// Drops to staticPlaceRegistry-only state (no dynamic) before
// asserting so a leaked dynamic state from another test doesn't mask
// a real regression.
func TestKnownMetros_SnapshotIncludesMajors(t *testing.T) {
	defer setDynamicMetros(nil, 0)
	setDynamicMetros(nil, 0)

	all := knownMetros()
	want := []string{"seattle", "new-york-city", "san-francisco", "chicago", "los-angeles"}
	for _, w := range want {
		if !slices.Contains(all, w) {
			t.Errorf("known places missing %q: %v", w, strings.Join(all, ","))
		}
	}
}

// TestHydrateMetroRegistry_NoOpOnFailure verifies a failing or empty
// load function doesn't downgrade the registry — the dynamic source
// in place before the call survives.
func TestHydrateMetroRegistry_NoOpOnFailure(t *testing.T) {
	defer setDynamicMetros(nil, 0)

	setDynamicMetros([]Place{{Slug: "preexisting", Name: "Pre", Lat: 1, Lng: 1, RadiusKm: 50}}, 100)
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Fatal("setup: dynamic place not loaded")
	}

	hydrateMetroRegistry(context.Background(), func(context.Context) ([]Place, int64, error) {
		return nil, 0, errSentinel{}
	})
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Error("error-returning hydrate wiped the dynamic registry")
	}

	hydrateMetroRegistry(context.Background(), func(context.Context) ([]Place, int64, error) {
		return []Place{}, 0, nil
	})
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Error("empty-return hydrate wiped the dynamic registry")
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel test error" }

// TestCityHintFor_NeighborhoodsOnly covers the curated neighborhood
// hint map. The Bellevue/Portland/Springfield/Columbia cases moved
// to the Place registry in U3 — cityHints now only carries the
// neighborhoods that don't deserve their own Place row.
func TestCityHintFor_NeighborhoodsOnly(t *testing.T) {
	cases := []struct {
		input    string
		wantHint string
	}{
		{"redmond", "seattle"},
		{"REDMOND", "seattle"},
		{"  kirkland  ", "seattle"},
		{"oakland", "san-francisco"},
		{"brooklyn", "new-york-city"},
		{"cambridge", "boston"},
		{"arlington", "washington-dc"},
		{"unknown-city", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := cityHintFor(tc.input); got != tc.wantHint {
				t.Errorf("cityHintFor(%q) = %q; want %q", tc.input, got, tc.wantHint)
			}
		})
	}
}

// TestFormatUnknownMetroError_GibberishFallsBack verifies the count +
// sample fallback fires when an input has no hint and no token match.
func TestFormatUnknownMetroError_GibberishFallsBack(t *testing.T) {
	got := formatUnknownMetroError("xyz12345-not-a-place")
	if !strings.Contains(got, "unknown metro") {
		t.Errorf("missing 'unknown metro' prefix: %s", got)
	}
	if !strings.Contains(got, "metros known") {
		t.Errorf("missing count signal: %s", got)
	}
}

// TestFormatUnknownMetroError_DidYouMean verifies the suggester layer
// fires for an input that shares tokens with a real registry entry
// but has no hint mapping.
func TestFormatUnknownMetroError_DidYouMean(t *testing.T) {
	got := formatUnknownMetroError("san-nowhere")
	if !strings.Contains(got, "did you mean") && !strings.Contains(got, "lumped under") {
		t.Errorf("expected 'did you mean' or 'lumped under' branch; got: %s", got)
	}
}

// TestPlaceData_NeighborhoodsPresent — U17. Boroughs / neighborhoods
// must each resolve to a curated entry with the right State and a
// tight RadiusKm so the radius-only ReverseLookup mechanic stops
// rounding "Times Square" up to "New York City" and "Santa Monica"
// up to "Los Angeles". Table-driven so a future entry tweak fails
// loudly in one place.
func TestPlaceData_NeighborhoodsPresent(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		slug         string
		wantState    string
		wantRadiusKm float64
	}{
		{"manhattan", "NY", 10},
		{"brooklyn", "NY", 10},
		{"queens", "NY", 12},
		{"west-hollywood", "CA", 5},
		{"santa-monica", "CA", 8},
		{"beverly-hills", "CA", 5},
		{"washington-dc-city", "DC", 12},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			p, ok := r.Lookup(tc.slug)
			if !ok {
				t.Fatalf("Lookup(%q) !ok", tc.slug)
			}
			if p.State != tc.wantState {
				t.Errorf("State = %q; want %q", p.State, tc.wantState)
			}
			if p.RadiusKm != tc.wantRadiusKm {
				t.Errorf("RadiusKm = %v; want %v", p.RadiusKm, tc.wantRadiusKm)
			}
			if p.Population <= 0 {
				t.Errorf("Population = %d; want > 0", p.Population)
			}
			if len(p.ContextHints) == 0 {
				t.Errorf("ContextHints empty; want at least one hint for disambiguation UX")
			}
			if p.Tier == PlaceTierUnknown {
				t.Errorf("Tier unset (PlaceTierUnknown) — every curated entry must declare its tier")
			}
		})
	}
}

// TestPlaceData_NeighborhoodAliases — U17. Human shorthands must hit
// the right neighborhood: `weho` is West Hollywood, `bk` is Brooklyn,
// `dc` is the city entry (not the metro), so an agent asking "any DC
// reservations" gets the tighter geography by default.
func TestPlaceData_NeighborhoodAliases(t *testing.T) {
	r := staticPlaceRegistry{}
	cases := []struct {
		alias    string
		wantSlug string
	}{
		{"weho", "west-hollywood"},
		{"bk", "brooklyn"},
		{"dc", "washington-dc-city"},
	}
	for _, tc := range cases {
		t.Run(tc.alias, func(t *testing.T) {
			p, ok := r.Lookup(tc.alias)
			if !ok {
				t.Fatalf("Lookup(%q) !ok", tc.alias)
			}
			if p.Slug != tc.wantSlug {
				t.Errorf("Lookup(%q).Slug = %q; want %q", tc.alias, p.Slug, tc.wantSlug)
			}
		})
	}
}

// TestReverseLookup_TimesSquareIsManhattan — U17. Times Square sits
// inside both Manhattan's 10 km radius and NYC's 75 km radius. The
// smallest-radius-wins tiebreak must pick Manhattan; otherwise the
// neighborhood entries do nothing useful.
func TestReverseLookup_TimesSquareIsManhattan(t *testing.T) {
	r := staticPlaceRegistry{}
	p, ok := r.ReverseLookup(40.7589, -73.9851)
	if !ok {
		t.Fatal("ReverseLookup(Times Square) !ok")
	}
	if p.Slug != "manhattan" {
		t.Errorf("ReverseLookup(Times Square).Slug = %q; want %q", p.Slug, "manhattan")
	}
}

// TestReverseLookup_SantaMonicaIsSantaMonica — U17. Santa Monica's
// centroid sits inside LA's 75 km but is far inside Santa Monica's
// own 8 km. Tighter radius wins.
func TestReverseLookup_SantaMonicaIsSantaMonica(t *testing.T) {
	r := staticPlaceRegistry{}
	p, ok := r.ReverseLookup(34.0195, -118.4912)
	if !ok {
		t.Fatal("ReverseLookup(Santa Monica) !ok")
	}
	if p.Slug != "santa-monica" {
		t.Errorf("ReverseLookup(Santa Monica).Slug = %q; want %q", p.Slug, "santa-monica")
	}
}

// TestLookupByName_NewYorkVsManhattan — U17. Pin that bare `manhattan`
// by-name returns exactly one hit (the borough entry) and bare
// `new york` does NOT collide with the borough. lookupByNameIn does
// strict equality after lowercasing, so the existing curated NYC entry
// (Name "New York City") doesn't match "new york" — the test pins
// that behavior intentionally.
func TestLookupByName_NewYorkVsManhattan(t *testing.T) {
	r := staticPlaceRegistry{}

	hits, ok := r.LookupByName("manhattan")
	if !ok {
		t.Fatal("LookupByName(manhattan) !ok")
	}
	if len(hits) != 1 {
		t.Errorf("LookupByName(manhattan) count = %d; want 1 (only the borough)", len(hits))
	}
	if len(hits) > 0 && hits[0].Slug != "manhattan" {
		t.Errorf("LookupByName(manhattan)[0].Slug = %q; want %q", hits[0].Slug, "manhattan")
	}

	// "new york" doesn't equal "New York City" after lowercase, so the
	// strict by-name lookup returns nothing — pin it so a future loose
	// matcher doesn't accidentally claim manhattan under this query.
	nyHits, nyOK := r.LookupByName("new york")
	if nyOK {
		// If a future change introduces loose matching, the rule is
		// still: "new york" must NOT return the manhattan borough.
		for _, p := range nyHits {
			if p.Slug == "manhattan" {
				t.Errorf("LookupByName(new york) returned manhattan; want NYC metro only")
			}
		}
	}
}

// TestLookupByName_WashingtonAmbiguity — U17. Pins what bare
// "washington" does after we add the city entry. `washington-dc-city`
// has Name "Washington", and the curated metro `washington-dc` has
// Name "Washington DC" — strict equality means only the city entry
// matches "washington". If a future contributor renames the metro to
// "Washington" too, the test will catch the new collision so the
// disambiguation UX can be updated deliberately.
func TestLookupByName_WashingtonAmbiguity(t *testing.T) {
	r := staticPlaceRegistry{}
	hits, ok := r.LookupByName("washington")
	if !ok {
		t.Fatal("LookupByName(washington) !ok")
	}
	if len(hits) != 1 {
		var slugs []string
		for _, p := range hits {
			slugs = append(slugs, p.Slug)
		}
		t.Errorf("LookupByName(washington) count = %d (%v); want 1 (washington-dc-city only, since metro Name is %q)",
			len(hits), slugs, "Washington DC")
	}
	if len(hits) > 0 && hits[0].Slug != "washington-dc-city" {
		t.Errorf("LookupByName(washington)[0].Slug = %q; want %q", hits[0].Slug, "washington-dc-city")
	}
}

// TestProjectTockMetros_Coverage verifies the Tock→Place projection
// preserves the four core fields and populates the U3 additions:
// ProviderCoverage["tock"] from BusinessCount, ParentMetro["tock"] =
// the entry's own slug (Tock has a flat hierarchy so each place
// self-routes), RadiusKm = 75 (Tock's metros are metro-tier), Tier =
// PlaceTierMetroCentroid.
func TestProjectTockMetros_Coverage(t *testing.T) {
	in := []tock.MetroArea{
		{Slug: "seattle", Name: "Seattle", Lat: 47.6, Lng: -122.3, BusinessCount: 120},
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6, Lng: -122.2, BusinessCount: 38},
		// Tock sometimes omits BusinessCount on emerging metros — make
		// sure ProviderCoverage stays nil rather than carrying a zero.
		{Slug: "emerging", Name: "Emerging", Lat: 0, Lng: 0, BusinessCount: 0},
	}
	got := projectTockMetros(in)
	if len(got) != 3 {
		t.Fatalf("got %d; want 3", len(got))
	}

	seattle := got[0]
	if seattle.Slug != "seattle" || seattle.Name != "Seattle" {
		t.Errorf("slug/name not preserved: %+v", seattle)
	}
	if seattle.Lat != 47.6 || seattle.Lng != -122.3 {
		t.Errorf("centroid not preserved: %+v", seattle)
	}
	if seattle.RadiusKm != 75 {
		t.Errorf("seattle RadiusKm = %v; want 75", seattle.RadiusKm)
	}
	if seattle.Tier != PlaceTierMetroCentroid {
		t.Errorf("seattle Tier = %v; want PlaceTierMetroCentroid", seattle.Tier)
	}
	if seattle.ProviderCoverage["tock"] != 120 {
		t.Errorf("seattle ProviderCoverage[tock] = %v; want 120", seattle.ProviderCoverage["tock"])
	}
	if seattle.ParentMetro["tock"] != "seattle" {
		t.Errorf("seattle ParentMetro[tock] = %q; want %q", seattle.ParentMetro["tock"], "seattle")
	}

	bellevue := got[1]
	if bellevue.ProviderCoverage["tock"] != 38 {
		t.Errorf("bellevue ProviderCoverage[tock] = %v; want 38", bellevue.ProviderCoverage["tock"])
	}
	if bellevue.ParentMetro["tock"] != "bellevue" {
		t.Errorf("bellevue ParentMetro[tock] = %q; want %q", bellevue.ParentMetro["tock"], "bellevue")
	}

	emerging := got[2]
	if emerging.ProviderCoverage != nil {
		t.Errorf("zero BusinessCount should leave ProviderCoverage nil; got %v", emerging.ProviderCoverage)
	}
	if emerging.ParentMetro["tock"] != "emerging" {
		t.Errorf("emerging ParentMetro[tock] = %q; want %q", emerging.ParentMetro["tock"], "emerging")
	}
}

// TestMergeRegistry_PreservesDynamicSlugAsAlias — U18 P1 fix. When a
// dynamic entry matches a curated entry via name+coords (not slug),
// the dynamic slug must be preserved as an alias on the merged entry
// so `Lookup(dynamicSlug)` keeps returning the merged place. Before
// the fix this lookup regressed to false, which broke
// inferMetroFromSlug_DEPRECATED's suffix-peel logic on venue slugs
// like `joey-bellevue`.
func TestMergeRegistry_PreservesDynamicSlugAsAlias(t *testing.T) {
	static := []Place{
		{
			Slug:         "bellevue-wa",
			Name:         "Bellevue",
			State:        "WA",
			Lat:          47.6101,
			Lng:          -122.2015,
			RadiusKm:     25,
			Population:   151854,
			ContextHints: []string{"Seattle metro", "Eastside", "tech hub"},
			Tier:         PlaceTierCity,
			// No Aliases preset — fix must populate one.
		},
	}
	dynamic := []Place{
		{
			Slug:             "bellevue",
			Name:             "Bellevue",
			Lat:              47.6105, // within 5 km of curated centroid
			Lng:              -122.2020,
			RadiusKm:         75,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1 (name+coords match must not add a row)", len(merged))
	}

	// Canonical slug still resolves.
	canonical, ok := lookupIn(merged, "bellevue-wa")
	if !ok {
		t.Fatal("bellevue-wa missing after merge")
	}
	if canonical.Slug != "bellevue-wa" {
		t.Errorf("canonical slug drifted: got %q", canonical.Slug)
	}

	// Dynamic slug now resolves to the SAME merged place via alias.
	viaAlias, ok := lookupIn(merged, "bellevue")
	if !ok {
		t.Fatal("Lookup(bellevue) returned false; dynamic slug must be preserved as alias")
	}
	if viaAlias.Slug != "bellevue-wa" {
		t.Errorf("alias lookup returned slug %q; want %q (alias must point at canonical row)", viaAlias.Slug, "bellevue-wa")
	}

	// Aliases on the merged entry include "bellevue".
	if !slices.Contains(canonical.Aliases, "bellevue") {
		t.Errorf("Aliases = %v; want it to contain %q", canonical.Aliases, "bellevue")
	}
}

// TestMergeRegistry_IdempotentAliasAppend — U18 P1 fix. Running merge
// twice with the same dynamic entry must not duplicate the alias.
func TestMergeRegistry_IdempotentAliasAppend(t *testing.T) {
	static := []Place{
		{
			Slug:     "bellevue-wa",
			Name:     "Bellevue",
			State:    "WA",
			Lat:      47.6101,
			Lng:      -122.2015,
			RadiusKm: 25,
			Tier:     PlaceTierCity,
		},
	}
	dynamic := []Place{
		{
			Slug:             "bellevue",
			Name:             "Bellevue",
			Lat:              47.6101,
			Lng:              -122.2015,
			RadiusKm:         75,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	first := mergeRegistry(static, dynamic)
	// Feed the first merge back in as the new static, simulating a
	// second hydration cycle.
	second := mergeRegistry(first, dynamic)

	if len(second) != 1 {
		t.Fatalf("second merged len = %d; want 1", len(second))
	}
	got, ok := lookupIn(second, "bellevue-wa")
	if !ok {
		t.Fatal("bellevue-wa missing after second merge")
	}
	count := 0
	for _, a := range got.Aliases {
		if a == "bellevue" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("alias %q appears %d times; want 1 (idempotent append)", "bellevue", count)
	}
}

// TestMergeRegistry_SlugMatchSkipsAliasAppend — U18 P1 fix. When the
// dynamic slug exactly equals the curated slug, no alias is appended
// (a self-reference alias would be noise).
func TestMergeRegistry_SlugMatchSkipsAliasAppend(t *testing.T) {
	static := []Place{
		{
			Slug:     "bellevue-wa",
			Name:     "Bellevue",
			State:    "WA",
			Lat:      47.6101,
			Lng:      -122.2015,
			RadiusKm: 25,
			Tier:     PlaceTierCity,
		},
	}
	dynamic := []Place{
		{
			Slug:             "bellevue-wa", // exact match
			Name:             "Bellevue, WA",
			Lat:              47.6101,
			Lng:              -122.2015,
			RadiusKm:         75,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1", len(merged))
	}
	got := merged[0]
	for _, a := range got.Aliases {
		if a == "bellevue-wa" {
			t.Errorf("Aliases contain self-reference %q; slug-exact match must not append an alias", a)
		}
	}
}

// TestMergeRegistry_NoStaticMapMutation — U18 P2 fix. mergeRegistry's
// shallow `copy(out, static)` carries map references through. Without
// deep-copying the maps before enrichment, writes leak back into the
// static source slice. This test pins that the curated entry's
// ProviderCoverage and ParentMetro stay untouched after merge.
func TestMergeRegistry_NoStaticMapMutation(t *testing.T) {
	staticParentMetro := map[string]string{"opentable": "seattle"}
	static := []Place{
		{
			Slug:         "bellevue-wa",
			Name:         "Bellevue",
			State:        "WA",
			Lat:          47.6101,
			Lng:          -122.2015,
			RadiusKm:     25,
			Population:   151854,
			ParentMetro:  staticParentMetro, // pre-populated curated routing
			ContextHints: []string{"Seattle metro", "Eastside", "tech hub"},
			Tier:         PlaceTierCity,
		},
	}
	// Capture the pointers/identities BEFORE merge.
	beforeParent := static[0].ParentMetro
	beforeCoverage := static[0].ProviderCoverage // nil

	dynamic := []Place{
		{
			Slug:             "bellevue",
			Name:             "Bellevue",
			Lat:              47.6101,
			Lng:              -122.2015,
			RadiusKm:         75,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	merged := mergeRegistry(static, dynamic)
	if len(merged) != 1 {
		t.Fatalf("merged len = %d; want 1", len(merged))
	}

	// Static slice's ProviderCoverage must remain nil (or at least lack
	// the "tock" key written by enrichment).
	if _, hasTock := static[0].ProviderCoverage["tock"]; hasTock {
		t.Errorf("static[0].ProviderCoverage was mutated; got %v", static[0].ProviderCoverage)
	}
	if beforeCoverage == nil && static[0].ProviderCoverage != nil {
		t.Errorf("static[0].ProviderCoverage went from nil to %v; static slice must not be mutated", static[0].ProviderCoverage)
	}

	// Static slice's ParentMetro must still ONLY carry "opentable" — the
	// "tock" key must NOT have been written into the shared map.
	if _, hasTock := static[0].ParentMetro["tock"]; hasTock {
		t.Errorf("static[0].ParentMetro was mutated; got %v", static[0].ParentMetro)
	}
	if static[0].ParentMetro["opentable"] != "seattle" {
		t.Errorf("static[0].ParentMetro[opentable] = %q; want curated value preserved", static[0].ParentMetro["opentable"])
	}
	// Identity: the captured pointer should still equal the source.
	if &beforeParent == nil || len(beforeParent) != 1 {
		t.Errorf("staticParentMetro backing was disturbed: %v", beforeParent)
	}
	if _, hasTock := beforeParent["tock"]; hasTock {
		t.Errorf("beforeParent (captured static ref) was mutated to %v", beforeParent)
	}

	// Static slice's Aliases must not have grown either (the alias-
	// append from P1 must happen on the merged copy, not the source).
	if len(static[0].Aliases) != 0 {
		t.Errorf("static[0].Aliases was mutated to %v; want empty", static[0].Aliases)
	}

	// The merged registry's bellevue-wa SHOULD carry the enriched fields.
	got := merged[0]
	if got.ProviderCoverage["tock"] != 42 {
		t.Errorf("merged[0].ProviderCoverage[tock] = %d; want 42", got.ProviderCoverage["tock"])
	}
	if got.ParentMetro["tock"] != "bellevue" {
		t.Errorf("merged[0].ParentMetro[tock] = %q; want %q", got.ParentMetro["tock"], "bellevue")
	}
	if got.ParentMetro["opentable"] != "seattle" {
		t.Errorf("merged[0].ParentMetro[opentable] = %q; want %q preserved from curated", got.ParentMetro["opentable"], "seattle")
	}
}

// TestSuffixPeelStillWorks_AfterHydration — U18 P1 regression pin.
// Before the fix, hydrating a dynamic `bellevue` entry into curated
// `bellevue-wa` dropped the `bellevue` slug, which made
// inferMetroFromSlug_DEPRECATED("joey-bellevue", ...) return ok=false
// (no suffix peeled). With the alias preserved, the lookup walks the
// alias and the suffix peel succeeds.
func TestSuffixPeelStillWorks_AfterHydration(t *testing.T) {
	defer setDynamicMetros(nil, 0)

	dynamic := []Place{
		{
			Slug:             "bellevue",
			Name:             "Bellevue",
			Lat:              47.6101,
			Lng:              -122.2015,
			RadiusKm:         75,
			ProviderCoverage: map[string]int{"tock": 42},
			ParentMetro:      map[string]string{"tock": "bellevue"},
		},
	}
	setDynamicMetros(dynamic, 1)

	reg := getRegistry()
	metro, prefix, ok := inferMetroFromSlug_DEPRECATED("joey-bellevue", reg)
	if !ok {
		t.Fatal("inferMetroFromSlug_DEPRECATED(joey-bellevue) ok=false; the dynamic bellevue alias must let the suffix-peel succeed")
	}
	if prefix != "joey" {
		t.Errorf("prefix = %q; want %q", prefix, "joey")
	}
	// Suffix peel resolves to Bellevue WA via the alias.
	if metro.Slug != "bellevue-wa" {
		t.Errorf("metro.Slug = %q; want %q (peel resolves via alias to canonical WA entry)", metro.Slug, "bellevue-wa")
	}
}
