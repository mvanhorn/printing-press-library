// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"strings"
	"testing"
)

// containsName reports whether any Candidate in alts has the given
// display name. Used by alternates assertions where the test cares
// about set membership, not order.
func containsName(alts []Candidate, want string) bool {
	for _, a := range alts {
		if a.Name == want {
			return true
		}
	}
	return false
}

// TestResolveLocation_HappyPaths covers the bread-and-butter resolves:
// city+state, bare city with a single registry match, coords, and
// metro qualifier. All should land on a non-nil *GeoContext with the
// resolved-name, centroid, and source propagated.
func TestResolveLocation_HappyPaths(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		source       Source
		wantResolved string
		wantLat      float64
		wantLng      float64
		minScore     float64
	}{
		{
			// Bellevue WA has Tier=City (no metro bonus), pop ~152k.
			// popularityPrior = 0.3*logNorm(152k) + 0 + 0 + 0.2*1.0
			// = ~0.22 + 0.2 = ~0.42. Worth pinning above 0.3 so a
			// regression to a zero-prior path is caught.
			name:         "city+state Bellevue WA",
			input:        "bellevue, wa",
			source:       SourceExplicitFlag,
			wantResolved: "Bellevue, WA",
			wantLat:      47.6101,
			wantLng:      -122.2015,
			minScore:     0.3,
		},
		{
			// Seattle: Tier=MetroCentroid (+0.2), pop ~754k (+0.3*logNorm),
			// exactMatchBonus=1.0 (+0.2). Lands ~0.6.
			name:         "bare city Seattle (single registry match)",
			input:        "seattle",
			source:       SourceExplicitFlag,
			wantResolved: "Seattle, WA",
			wantLat:      47.6062,
			wantLng:      -122.3321,
			minScore:     0.5,
		},
		{
			// LocKindCoords -> ReverseLookup hits Bellevue (city tier,
			// RadiusKm=25 contains the point). The coords path provides
			// no CityName -> exactMatchBonus=0; the prior collapses to
			// popTerm only. Verify > 0 isn't claimed.
			name:         "coords inside Bellevue radius",
			input:        "47.6101,-122.2015",
			source:       SourceExplicitFlag,
			wantResolved: "Bellevue, WA",
			wantLat:      47.6101,
			wantLng:      -122.2015,
			minScore:     0.0,
		},
		{
			// LocKindMetro looks up "seattle" by slug -> Seattle Place.
			// LocKindMetro provides no CityName -> exactMatchBonus=0; the
			// prior is 0.3*logNorm(754k) + 0.2*metroBonus = ~0.45.
			name:         "metro qualifier 'seattle metro'",
			input:        "seattle metro",
			source:       SourceExplicitFlag,
			wantResolved: "Seattle, WA",
			wantLat:      47.6062,
			wantLng:      -122.3321,
			minScore:     0.3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc, env, err := ResolveLocation(tc.input, ResolveOptions{Source: tc.source})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env != nil {
				t.Fatalf("expected GeoContext, got envelope: %+v", env)
			}
			if gc == nil {
				t.Fatal("expected non-nil GeoContext")
			}
			if gc.ResolvedTo != tc.wantResolved {
				t.Errorf("ResolvedTo = %q; want %q", gc.ResolvedTo, tc.wantResolved)
			}
			if math.Abs(gc.Centroid[0]-tc.wantLat) > 0.001 {
				t.Errorf("Centroid[0] = %v; want ~%v", gc.Centroid[0], tc.wantLat)
			}
			if math.Abs(gc.Centroid[1]-tc.wantLng) > 0.001 {
				t.Errorf("Centroid[1] = %v; want ~%v", gc.Centroid[1], tc.wantLng)
			}
			if gc.RadiusKm <= 0 {
				t.Errorf("RadiusKm = %v; want > 0", gc.RadiusKm)
			}
			if gc.Source != tc.source {
				t.Errorf("Source = %q; want %q", gc.Source, tc.source)
			}
			if gc.Score < tc.minScore {
				t.Errorf("Score = %v; want >= %v", gc.Score, tc.minScore)
			}
			if gc.Origin != tc.input {
				t.Errorf("Origin = %q; want %q", gc.Origin, tc.input)
			}
		})
	}
}

// TestResolveLocation_EmptyInput covers the no-constraint signal:
// empty and whitespace-only input return (nil, nil, nil) so callers
// know to skip both pre- and post-filter (R13).
func TestResolveLocation_EmptyInput(t *testing.T) {
	cases := []string{"", "   ", "\t\n"}
	for _, in := range cases {
		t.Run("empty="+in, func(t *testing.T) {
			gc, env, err := ResolveLocation(in, ResolveOptions{Source: SourceExplicitFlag})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gc != nil {
				t.Errorf("expected nil GeoContext; got %+v", gc)
			}
			if env != nil {
				t.Errorf("expected nil envelope; got %+v", env)
			}
		})
	}
}

// TestResolveLocation_UnknownLocation verifies the envelope path for
// bare-token inputs that don't match any registry entry. The agent
// gets a needs_clarification envelope with no candidates and an
// error_kind of location_unknown.
func TestResolveLocation_UnknownLocation(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"bare unknown token", "99999"},
		{"metro qualifier with unknown slug", "narnia metro"},
		{"city+state with unmatched state", "bellevue, zz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc, env, err := ResolveLocation(tc.input, ResolveOptions{Source: SourceExplicitFlag})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gc != nil {
				t.Errorf("expected nil GeoContext; got %+v", gc)
			}
			if env == nil {
				t.Fatalf("expected envelope for unknown location %q", tc.input)
			}
			if env.ErrorKind != ErrorKindLocationUnknown {
				t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationUnknown)
			}
			if len(env.Candidates) != 0 {
				t.Errorf("Candidates len = %d; want 0 for unknown", len(env.Candidates))
			}
			if !env.NeedsClarification {
				t.Error("NeedsClarification should be true")
			}
		})
	}
}

// TestResolveLocation_CoordParseError verifies parser errors are
// propagated as the (nil, nil, err) tuple rather than swallowed into
// an envelope. The lat/lng range check fires inside ParseLocation.
func TestResolveLocation_CoordParseError(t *testing.T) {
	gc, env, err := ResolveLocation("100.5,200.3", ResolveOptions{Source: SourceExplicitFlag})
	if err == nil {
		t.Fatal("expected parse error for out-of-range coords")
	}
	if gc != nil {
		t.Errorf("expected nil GeoContext on parse error; got %+v", gc)
	}
	if env != nil {
		t.Errorf("expected nil envelope on parse error; got %+v", env)
	}
	if !strings.Contains(err.Error(), "latitude") && !strings.Contains(err.Error(), "longitude") {
		t.Errorf("expected lat/lng range error; got %v", err)
	}
}

// TestResolveLocation_CoordsOutsideAllPlaces verifies the synthetic-
// Place fallback when ReverseLookup misses. Montreal (45.5, -73.6)
// is outside every curated US Place, so ResolveLocation synthesizes
// a single-candidate Place at the query point and returns HIGH tier
// (1 candidate is unambiguous by definition).
func TestResolveLocation_CoordsOutsideAllPlaces(t *testing.T) {
	gc, env, err := ResolveLocation("45.5,-73.6", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Fatalf("expected GeoContext, got envelope: %+v", env)
	}
	if gc == nil {
		t.Fatal("expected non-nil GeoContext")
	}
	if math.Abs(gc.Centroid[0]-45.5) > 0.001 {
		t.Errorf("Centroid[0] = %v; want ~45.5", gc.Centroid[0])
	}
	if math.Abs(gc.Centroid[1]-(-73.6)) > 0.001 {
		t.Errorf("Centroid[1] = %v; want ~-73.6", gc.Centroid[1])
	}
	// Synthetic Place has Population=0, no coverage, not a metro centroid,
	// and the LocKindCoords path provides no CityName -> exactMatchBonus
	// fires 0. Prior should be exactly 0.
	if gc.Score != 0 {
		t.Errorf("synthetic-coords Score = %v; want 0 (zero pop, no coverage)", gc.Score)
	}
	if gc.RadiusKm != defaultSyntheticRadiusKm {
		t.Errorf("synthetic-coords RadiusKm = %v; want %v", gc.RadiusKm, defaultSyntheticRadiusKm)
	}
}

// TestResolveLocation_AmbiguousBellevue covers the canonical 3-way
// ambiguity from R14 F1: "bellevue" matches Bellevue WA + NE + KY.
// Without AcceptAmbiguous, the envelope path fires with all three
// candidates ranked by popularityPrior (WA > NE > KY by population).
func TestResolveLocation_AmbiguousBellevue(t *testing.T) {
	gc, env, err := ResolveLocation("bellevue", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc != nil {
		t.Errorf("expected envelope for ambiguous bellevue; got GeoContext %+v", gc)
	}
	if env == nil {
		t.Fatal("expected envelope")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
	if env.WhatWasAsked != "bellevue" {
		t.Errorf("WhatWasAsked = %q; want %q", env.WhatWasAsked, "bellevue")
	}
	if len(env.Candidates) != 3 {
		t.Fatalf("Candidates len = %d; want 3 (WA, NE, KY)", len(env.Candidates))
	}
	// Top should be WA by population.
	if env.Candidates[0].State != "WA" {
		t.Errorf("top candidate state = %q; want %q (highest pop)", env.Candidates[0].State, "WA")
	}
	for _, want := range []string{"WA", "NE", "KY"} {
		found := false
		for _, c := range env.Candidates {
			if c.State == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("envelope candidates missing state %q; got %+v", want, env.Candidates)
		}
	}
}

// TestResolveLocation_AmbiguousBellevue_AcceptAmbiguous verifies the
// forced-pick path: AcceptAmbiguous=true on a LOW-tier result returns
// the top candidate as a GeoContext (with Alternates carrying NE and
// KY) instead of the envelope.
func TestResolveLocation_AmbiguousBellevue_AcceptAmbiguous(t *testing.T) {
	gc, env, err := ResolveLocation("bellevue", ResolveOptions{
		Source:          SourceExplicitFlag,
		AcceptAmbiguous: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Errorf("expected GeoContext on AcceptAmbiguous; got envelope %+v", env)
	}
	if gc == nil {
		t.Fatal("expected non-nil GeoContext")
	}
	if gc.ResolvedTo != "Bellevue, WA" {
		t.Errorf("ResolvedTo = %q; want %q (top by prior)", gc.ResolvedTo, "Bellevue, WA")
	}
	if len(gc.Alternates) != 2 {
		t.Errorf("Alternates len = %d; want 2 (NE and KY)", len(gc.Alternates))
	}
	if !containsName(gc.Alternates, "Bellevue, NE") {
		t.Errorf("Alternates missing Bellevue, NE; got %+v", gc.Alternates)
	}
	if !containsName(gc.Alternates, "Bellevue, KY") {
		t.Errorf("Alternates missing Bellevue, KY; got %+v", gc.Alternates)
	}
}

// TestResolveLocation_PortlandIsLow verifies the U14-revised behavior
// for bare ambiguous LocCity with 2 candidates: Portland OR vs Portland
// ME both surface as candidates, but the pipeline returns the envelope
// (not a MEDIUM-tier forced pick). Codex P2-F/P2-G flagged the prior
// MEDIUM "guess and warn" outcome as wrong-city UX for the minority-
// population case; the envelope path lets the agent disambiguate.
func TestResolveLocation_PortlandIsLow(t *testing.T) {
	gc, env, err := ResolveLocation("portland", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc != nil {
		t.Errorf("expected envelope for bare 2-candidate Portland; got GeoContext %+v", gc)
	}
	if env == nil {
		t.Fatal("expected envelope")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
	if env.WhatWasAsked != "portland" {
		t.Errorf("WhatWasAsked = %q; want %q", env.WhatWasAsked, "portland")
	}
	if len(env.Candidates) != 2 {
		t.Fatalf("Candidates len = %d; want 2 (OR, ME)", len(env.Candidates))
	}
	// OR ranks first by popularity prior.
	if env.Candidates[0].State != "OR" {
		t.Errorf("top candidate state = %q; want OR (higher pop)", env.Candidates[0].State)
	}
	if env.Candidates[1].State != "ME" {
		t.Errorf("second candidate state = %q; want ME", env.Candidates[1].State)
	}
}

// TestResolveLocation_SpringfieldIsAmbiguous verifies R14 F5: four
// Springfields (MA/IL/MO/OR) route to LOW -> envelope path. 4 >= 3
// with SpecificityLow forces TierLow regardless of population gaps.
func TestResolveLocation_SpringfieldIsAmbiguous(t *testing.T) {
	gc, env, err := ResolveLocation("springfield", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc != nil {
		t.Errorf("expected envelope for 4-way Springfield; got GeoContext %+v", gc)
	}
	if env == nil {
		t.Fatal("expected envelope")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
	if len(env.Candidates) != 4 {
		t.Errorf("Candidates len = %d; want 4 (MA/IL/MO/OR)", len(env.Candidates))
	}
}

// TestResolveLocation_CityStateNarrowsAmbiguity verifies that adding
// the state qualifier collapses the 3-way Bellevue ambiguity to a
// single HIGH-tier match without needing AcceptAmbiguous.
func TestResolveLocation_CityStateNarrowsAmbiguity(t *testing.T) {
	gc, env, err := ResolveLocation("bellevue, wa", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Fatalf("expected GeoContext after state narrowing; got envelope %+v", env)
	}
	if gc == nil {
		t.Fatal("expected non-nil GeoContext")
	}
	if gc.ResolvedTo != "Bellevue, WA" {
		t.Errorf("ResolvedTo = %q; want Bellevue, WA", gc.ResolvedTo)
	}
	if len(gc.Alternates) != 0 {
		t.Errorf("Alternates len = %d; want 0 (state filter eliminated NE/KY)", len(gc.Alternates))
	}
}

// TestResolveLocation_SourcePropagation verifies the Source field
// flows through unmodified for each of the three enum values. The
// caller's intent (explicit flag vs slug-suffix inference vs fallback)
// must survive the pipeline so post-filter mode selection can branch
// on it downstream.
func TestResolveLocation_SourcePropagation(t *testing.T) {
	cases := []Source{SourceExplicitFlag, SourceExtractedFromQuery, SourceDefault}
	for _, src := range cases {
		t.Run(string(src), func(t *testing.T) {
			gc, env, err := ResolveLocation("seattle", ResolveOptions{Source: src})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env != nil {
				t.Fatalf("expected GeoContext; got envelope %+v", env)
			}
			if gc.Source != src {
				t.Errorf("Source = %q; want %q", gc.Source, src)
			}
		})
	}
}

// TestResolveLocation_IntegratesWithApplyGeoFilter ties U5's two
// outputs together: a Seattle-anchored GeoContext from ResolveLocation
// must drive applyGeoFilter to drop NYC venues from a mixed result
// set under hard-reject. Verifies the GeoContext.Centroid / RadiusKm
// projection is consumable by the migrated filter.
func TestResolveLocation_IntegratesWithApplyGeoFilter(t *testing.T) {
	gc, env, err := ResolveLocation("seattle", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Fatalf("expected GeoContext; got envelope %+v", env)
	}
	results := []goatResult{
		{Name: "Canlis (Seattle)", Latitude: 47.6452, Longitude: -122.3766, MatchScore: 0.95},
		{Name: "Wildair (NYC)", Latitude: 40.7128, Longitude: -74.0060, MatchScore: 0.95},
	}
	got := applyGeoFilter(results, gc, metroFilterHardReject)
	if len(got) != 1 {
		t.Fatalf("got %d results; want 1 (NYC dropped)", len(got))
	}
	if got[0].Name != "Canlis (Seattle)" {
		t.Errorf("kept result name = %q; want Canlis (Seattle)", got[0].Name)
	}
}

// TestIsCanonicalMetro_PrimarySlug pins the U21 "primary slug match"
// branch: an input that equals the resolved Place's primary Slug field
// is unambiguously canonical. Seattle has Slug="seattle"; this is the
// trivial canonical path.
func TestIsCanonicalMetro_PrimarySlug(t *testing.T) {
	if !isCanonicalMetro("seattle") {
		t.Error("isCanonicalMetro(\"seattle\") = false; want true (primary slug match)")
	}
}

// TestIsCanonicalMetro_AliasToSingleCanonical pins the U21 "alias to
// single canonical" branch: "sf" is an alias on san-francisco, and the
// input is NOT itself ambiguous as a city name (LookupByName("sf")
// returns no matches because no Place has Name="sf"). Should be
// canonical.
func TestIsCanonicalMetro_AliasToSingleCanonical(t *testing.T) {
	if !isCanonicalMetro("sf") {
		t.Error("isCanonicalMetro(\"sf\") = false; want true (alias to single canonical, no name ambiguity)")
	}
}

// TestIsCanonicalMetro_AliasToAmbiguousName_AfterHydration is THE
// round-4 P1 fix. After Tock hydration absorbs a dynamic "bellevue"
// entry into curated bellevue-wa (U18 name+coords match), the dynamic
// slug "bellevue" is added as an alias on bellevue-wa. Pre-U21,
// isCanonicalMetro("bellevue") returned true via the Lookup hit and
// silently picked Bellevue WA — masking the genuine WA/NE/KY ambiguity.
// Post-U21, isCanonicalMetro must return false here because
// LookupByName("bellevue") returns 3 matches, so the input is
// ambiguous as a city name even though Lookup resolves it via an alias.
func TestIsCanonicalMetro_AliasToAmbiguousName_AfterHydration(t *testing.T) {
	t.Cleanup(func() { setDynamicMetros(nil, 0) })
	// Mimic Tock hydration: a dynamic "bellevue" entry whose
	// (Name, Lat, Lng) match bellevue-wa within 5 km triggers U18's
	// name+coords merge path which appends "bellevue" to
	// bellevue-wa's Aliases.
	setDynamicMetros([]Place{
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6101, Lng: -122.2015},
	}, 1)
	// Sanity: Lookup("bellevue") must now succeed (alias resolution),
	// otherwise the test is exercising a different code path than the
	// regression.
	if _, ok := getRegistry().Lookup("bellevue"); !ok {
		t.Fatal("setup: Lookup(\"bellevue\") should succeed after hydration alias-append")
	}
	if isCanonicalMetro("bellevue") {
		t.Error("isCanonicalMetro(\"bellevue\") = true; want false (alias to ambiguous name; envelope path must fire)")
	}
}

// TestIsCanonicalMetro_PrimarySlugUnambiguous_EvenAfterHydration
// verifies the primary-slug path keeps winning even under the same
// hydration as the round-4 fix. "bellevue-wa" equals the curated
// Place's primary Slug; it remains canonical regardless of what aliases
// the merge appends.
func TestIsCanonicalMetro_PrimarySlugUnambiguous_EvenAfterHydration(t *testing.T) {
	t.Cleanup(func() { setDynamicMetros(nil, 0) })
	setDynamicMetros([]Place{
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6101, Lng: -122.2015},
	}, 1)
	if !isCanonicalMetro("bellevue-wa") {
		t.Error("isCanonicalMetro(\"bellevue-wa\") = false; want true (primary slug; unambiguous even after hydration)")
	}
}

// TestIsCanonicalMetro_UnknownSlug pins the false branch: a value with
// no Lookup hit and no LookupByName hit is not canonical.
func TestIsCanonicalMetro_UnknownSlug(t *testing.T) {
	if isCanonicalMetro("totally-fake-place") {
		t.Error("isCanonicalMetro(\"totally-fake-place\") = true; want false")
	}
}

// TestIsCanonicalMetro_NameMatchSingle pins the "no Lookup hit, single
// LookupByName hit" branch. The case is documented even though
// "seattle" hits the primary-slug path first; the assertion here is
// idempotent with TestIsCanonicalMetro_PrimarySlug but documents the
// fallthrough case explicitly.
func TestIsCanonicalMetro_NameMatchSingle(t *testing.T) {
	if !isCanonicalMetro("seattle") {
		t.Error("isCanonicalMetro(\"seattle\") = false; want true (canonical via slug or single name match)")
	}
}

// TestApplyGeoFilter_NilContextIntegration is the R13 contract test
// at the integration layer: ResolveLocation("") returns nil GeoContext
// and applyGeoFilter(results, nil, ...) is a true pass-through. The
// two together prove the no-filter path for callers that omit
// --location.
func TestApplyGeoFilter_NilContextIntegration(t *testing.T) {
	gc, env, err := ResolveLocation("", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc != nil || env != nil {
		t.Fatalf("empty input should produce (nil, nil, nil); got gc=%v env=%v", gc, env)
	}
	results := []goatResult{
		{Name: "Anywhere", Latitude: 1, Longitude: 1, MatchScore: 0.95},
		{Name: "Far Far Away", Latitude: 50, Longitude: 50, MatchScore: 0.4},
	}
	got := applyGeoFilter(results, gc, metroFilterHardReject)
	if len(got) != 2 {
		t.Errorf("nil ctx should preserve all rows; got %d", len(got))
	}
}

// TestResolveLocation_NYCAndDCAliases — U22. Natural-language short
// forms ("nyc", "dc") and truncated names ("new york", "washington")
// must resolve via the alias-aware LookupByName extension. Pre-U22
// these returned location_unknown because lookupByNameIn did strict
// exact-equal on Place.Name only. Pipeline integration covers the
// LocKindCity and LocKindCityState paths.
func TestResolveLocation_NYCAndDCAliases(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		wantResolved string
	}{
		{"nyc short form", "nyc", "New York City, NY"},
		{"new york natural-language", "new york", "New York City, NY"},
		{"new york with state qualifier", "new york, ny", "New York City, NY"},
		{"washington natural-language", "washington", "Washington, DC"},
		{"dc short form", "dc", "Washington, DC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc, env, err := ResolveLocation(tc.input, ResolveOptions{Source: SourceExplicitFlag})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env != nil {
				t.Fatalf("expected GeoContext; got envelope %+v", env)
			}
			if gc == nil {
				t.Fatal("expected non-nil GeoContext")
			}
			if gc.Tier != ResolutionTierHigh {
				t.Errorf("Tier = %q; want %q", gc.Tier, ResolutionTierHigh)
			}
			if gc.ResolvedTo != tc.wantResolved {
				t.Errorf("ResolvedTo = %q; want %q", gc.ResolvedTo, tc.wantResolved)
			}
		})
	}
}

// TestResolveLocation_NYCWithWrongState — U22. State qualifier still
// filters out wrong-state candidates after the alias-aware city-name
// lookup. "new york, ca" finds NYC via the new alias on the city-name
// step, then the state filter eliminates it (NYC.State = "NY"),
// leaving zero candidates → location_unknown envelope.
func TestResolveLocation_NYCWithWrongState(t *testing.T) {
	gc, env, err := ResolveLocation("new york, ca", ResolveOptions{Source: SourceExplicitFlag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc != nil {
		t.Errorf("expected envelope for state-mismatch; got GeoContext %+v", gc)
	}
	if env == nil {
		t.Fatal("expected envelope")
	}
	if env.ErrorKind != ErrorKindLocationUnknown {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationUnknown)
	}
}
