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
		{"manhattan", "new-york-city"},
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
		{"manhattan", "New York City"},
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

// TestChainedRegistry_DynamicOverridesStatic verifies the chain
// promise: a dynamic entry shadowing a curated slug wins. Tock's
// metroArea SSR is more current than the curated table, so the
// dynamic centroid should surface even when the curated table also
// covers the slug.
func TestChainedRegistry_DynamicOverridesStatic(t *testing.T) {
	dyn := Place{Slug: "seattle", Name: "Seattle (dyn)", Lat: 47.7, Lng: -122.4, RadiusKm: 75, Tier: PlaceTierMetroCentroid}
	chain := chainedPlaceRegistry{dynamic: []Place{dyn}}
	p, ok := chain.Lookup("seattle")
	if !ok {
		t.Fatal("seattle Lookup !ok")
	}
	if p.Name != "Seattle (dyn)" {
		t.Errorf("Name = %q; want dynamic %q", p.Name, "Seattle (dyn)")
	}
	if p.Lat != 47.7 || p.Lng != -122.4 {
		t.Errorf("centroid = %v,%v; want dynamic 47.7,-122.4", p.Lat, p.Lng)
	}
}

// TestChainedRegistry_StaticFallback verifies entries the dynamic
// source doesn't cover still resolve via the curated fallback.
func TestChainedRegistry_StaticFallback(t *testing.T) {
	chain := chainedPlaceRegistry{dynamic: []Place{
		{Slug: "tock-only-metro", Name: "Tock Only", Lat: 1, Lng: 1, RadiusKm: 75},
	}}
	if p, ok := chain.Lookup("chicago"); !ok || p.Slug != "chicago" {
		t.Errorf("chicago should fall through to curated; got (%+v, %v)", p, ok)
	}
	if p, ok := chain.Lookup("tock-only-metro"); !ok || p.Slug != "tock-only-metro" {
		t.Errorf("tock-only-metro should resolve from dynamic; got (%+v, %v)", p, ok)
	}
}

// TestChainedRegistry_All verifies dynamic-first union with dedup by
// canonical slug.
func TestChainedRegistry_All(t *testing.T) {
	chain := chainedPlaceRegistry{dynamic: []Place{
		{Slug: "tock-x", Name: "Tock X", Lat: 1, Lng: 1, RadiusKm: 75},
		{Slug: "seattle", Name: "Seattle (dyn)", Lat: 47.6, Lng: -122.3, RadiusKm: 75},
	}}
	all := chain.All()
	if all[0].Slug != "tock-x" || all[1].Slug != "seattle" {
		t.Errorf("dynamic should appear first; got %v + %v", all[0].Slug, all[1].Slug)
	}
	seenSeattle := 0
	for _, p := range all {
		if p.Slug == "seattle" {
			seenSeattle++
		}
	}
	if seenSeattle != 1 {
		t.Errorf("seattle duplicated in chain.All(); count = %d", seenSeattle)
	}
	hasChicago := slices.ContainsFunc(all, func(p Place) bool { return p.Slug == "chicago" })
	if !hasChicago {
		t.Error("curated chicago missing from chain.All()")
	}
}

// TestChainedRegistry_LookupByName_Union verifies that the chained
// registry surfaces ambiguous-name matches across both dynamic and
// curated sources, deduping by canonical slug.
func TestChainedRegistry_LookupByName_Union(t *testing.T) {
	chain := chainedPlaceRegistry{dynamic: []Place{
		// Dynamic shadowing curated Springfield IL — same slug, same name,
		// should not duplicate.
		{Slug: "springfield-il", Name: "Springfield", Lat: 39.8, Lng: -89.7, RadiusKm: 75},
		// Dynamic new Springfield (NJ) — adds an alternate to the union.
		{Slug: "springfield-nj", Name: "Springfield", Lat: 40.7, Lng: -74.3, RadiusKm: 75},
	}}
	hits, ok := chain.LookupByName("springfield")
	if !ok {
		t.Fatal("LookupByName(springfield) !ok")
	}
	// Should have 5: springfield-il (dynamic), springfield-nj (dynamic-only),
	// springfield-ma, springfield-mo, springfield-or (curated).
	if len(hits) != 5 {
		t.Errorf("Springfield hit count = %d; want 5 (dynamic + curated dedup)", len(hits))
	}
	// Verify dynamic shadowed the curated springfield-il.
	for _, p := range hits {
		if p.Slug == "springfield-il" && p.Lat != 39.8 {
			t.Errorf("springfield-il should show dynamic Lat; got %v", p.Lat)
		}
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
