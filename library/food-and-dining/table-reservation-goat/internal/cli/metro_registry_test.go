// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
)

// TestStaticRegistry_Lookup covers the legacy 20-entry baseline plus
// alias resolution. The aliases were previously OR'd into single switch
// cases; the registry surfaces them as a property so we test each shape
// once and trust them everywhere.
func TestStaticRegistry_Lookup(t *testing.T) {
	r := staticMetroRegistry{}
	cases := []struct {
		input    string
		wantSlug string
	}{
		{"seattle", "seattle"},
		{"Seattle", "seattle"},        // case insensitive
		{"  seattle  ", "seattle"},    // whitespace tolerated
		{"sf", "san-francisco"},       // alias
		{"SF", "san-francisco"},       // alias case insensitive
		{"nyc", "new-york-city"},      // alias
		{"new-york", "new-york-city"}, // alias
		{"manhattan", "new-york-city"},
		{"nola", "new-orleans"},
		{"vegas", "las-vegas"},
		{"dc", "washington-dc"},
		{"philly", "philadelphia"},
		{"la", "los-angeles"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			m, ok := r.Lookup(tc.input)
			if !ok {
				t.Fatalf("Lookup(%q) returned !ok; expected slug %q", tc.input, tc.wantSlug)
			}
			if m.Slug != tc.wantSlug {
				t.Errorf("Lookup(%q).Slug = %q; want %q", tc.input, m.Slug, tc.wantSlug)
			}
			if m.Lat == 0 && m.Lng == 0 {
				t.Errorf("Lookup(%q).Lat/Lng = 0/0; centroid must be populated", tc.input)
			}
			if m.Name == "" {
				t.Errorf("Lookup(%q).Name is empty; display name required for Tock ?city= param", tc.input)
			}
		})
	}
}

// TestStaticRegistry_UnknownSlug verifies the unknown case returns
// ok=false rather than a zero-valued Metro pretending to be a real
// entry. Issue #406's "unknown metro %q (known: %s)" error UX depends
// on this signal.
func TestStaticRegistry_UnknownSlug(t *testing.T) {
	r := staticMetroRegistry{}
	for _, in := range []string{"", "  ", "bellevue", "shanghai", "made-up-metro"} {
		if _, ok := r.Lookup(in); ok {
			t.Errorf("Lookup(%q) returned ok=true unexpectedly", in)
		}
	}
}

// TestChainedRegistry_DynamicOverridesStatic verifies the core promise
// of the dynamic-over-static chain: when Tock metroArea hydrates a
// metro that the static fallback ALSO covers, the dynamic centroid
// wins. Concrete case: Tock's seattle entry has slightly different
// coords than the static placeholder, and the chain must surface the
// dynamic ones because they're closer to whatever Tock's search uses
// internally.
func TestChainedRegistry_DynamicOverridesStatic(t *testing.T) {
	dynamicSeattle := Metro{Slug: "seattle", Name: "Seattle (dyn)", Lat: 47.7, Lng: -122.4}
	chain := chainedMetroRegistry{dynamic: []Metro{dynamicSeattle}}
	m, ok := chain.Lookup("seattle")
	if !ok {
		t.Fatal("expected seattle lookup to succeed")
	}
	if m.Name != "Seattle (dyn)" {
		t.Errorf("Name = %q; want dynamic %q (chain must prefer dynamic over static)", m.Name, "Seattle (dyn)")
	}
	if m.Lat != 47.7 || m.Lng != -122.4 {
		t.Errorf("centroid = %v,%v; want dynamic 47.7,-122.4", m.Lat, m.Lng)
	}
}

// TestChainedRegistry_StaticFallback verifies entries the dynamic
// source DOESN'T cover still resolve via the static fallback.
func TestChainedRegistry_StaticFallback(t *testing.T) {
	chain := chainedMetroRegistry{dynamic: []Metro{
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6101, Lng: -122.2015},
	}}
	if m, ok := chain.Lookup("chicago"); !ok || m.Slug != "chicago" {
		t.Errorf("chicago should fall through to static; got (%+v, %v)", m, ok)
	}
	if m, ok := chain.Lookup("bellevue"); !ok || m.Slug != "bellevue" {
		t.Errorf("bellevue should resolve from dynamic; got (%+v, %v)", m, ok)
	}
}

// TestChainedRegistry_All verifies the union semantics: dynamic
// entries come first, then static entries the dynamic source didn't
// already cover. Order matters for the "known metros" error UX, which
// surfaces the most relevant entries first.
func TestChainedRegistry_All(t *testing.T) {
	chain := chainedMetroRegistry{dynamic: []Metro{
		{Slug: "bellevue", Name: "Bellevue", Lat: 47.6, Lng: -122.2},
		{Slug: "seattle", Name: "Seattle (dyn)", Lat: 47.6, Lng: -122.3},
	}}
	all := chain.All()
	if all[0].Slug != "bellevue" || all[1].Slug != "seattle" {
		t.Errorf("dynamic entries should appear first; got %v + %v", all[0].Slug, all[1].Slug)
	}
	for i, m := range all {
		if m.Slug == "seattle" && m.Name == "Seattle" && i > 1 {
			t.Errorf("static seattle leaked into All() at idx %d; dedupe failed", i)
		}
	}
	hasBellevue := slices.ContainsFunc(all, func(m Metro) bool { return m.Slug == "bellevue" })
	if !hasBellevue {
		t.Error("bellevue (dynamic-only) missing from All()")
	}
	hasAustin := slices.ContainsFunc(all, func(m Metro) bool { return m.Slug == "austin" })
	if !hasAustin {
		t.Error("austin (static-only) missing from All() — fallback merging is broken")
	}
}

// TestSetDynamicMetros_Concurrency verifies the registry singleton can
// be upgraded concurrently without panicking. Two goroutines racing to
// set the dynamic source should both succeed; last writer wins.
func TestSetDynamicMetros_Concurrency(t *testing.T) {
	defer setDynamicMetros(nil, 0)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			setDynamicMetros([]Metro{
				{Slug: "test-metro", Name: "Test", Lat: float64(i), Lng: float64(i)},
			}, int64(i))
		}(i)
	}
	wg.Wait()

	if _, ok := getRegistry().Lookup("test-metro"); !ok {
		t.Error("post-race lookup failed; race may have corrupted the registry")
	}
}

// TestMetroLatLng_LegacyShape verifies the legacy wrapper still
// surfaces the same (lat, lng, ok) signature pre-#406 callers rely on.
func TestMetroLatLng_LegacyShape(t *testing.T) {
	lat, lng, ok := metroLatLng("seattle")
	if !ok || lat == 0 || lng == 0 {
		t.Errorf("legacy wrapper broken: (%v, %v, %v)", lat, lng, ok)
	}
	_, _, ok = metroLatLng("nonexistent-metro")
	if ok {
		t.Error("legacy wrapper should report ok=false on unknown slug")
	}
}

// TestKnownMetros_SnapshotIncludesMajors is a sanity-check that the
// `--metro` UX error message ("unknown metro %q (known: %s)") includes
// at least the major US slugs. Catches accidentally dropping rows from
// staticMetros.
func TestKnownMetros_SnapshotIncludesMajors(t *testing.T) {
	all := knownMetros()
	want := []string{"seattle", "new-york-city", "san-francisco", "chicago", "los-angeles"}
	for _, w := range want {
		if !slices.Contains(all, w) {
			t.Errorf("known metros missing %q: %v", w, strings.Join(all, ","))
		}
	}
}

// TestHydrateMetroRegistry_NoOpOnFailure verifies a failing or empty
// load function doesn't downgrade the registry. The static fallback
// stays in place.
func TestHydrateMetroRegistry_NoOpOnFailure(t *testing.T) {
	defer setDynamicMetros(nil, 0)

	setDynamicMetros([]Metro{{Slug: "preexisting", Name: "Pre", Lat: 1, Lng: 1}}, 100)
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Fatal("setup: dynamic metro not loaded")
	}

	hydrateMetroRegistry(context.Background(), func(context.Context) ([]Metro, int64, error) {
		return nil, 0, errSentinel{}
	})
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Error("error-returning hydrate wiped the dynamic registry")
	}

	hydrateMetroRegistry(context.Background(), func(context.Context) ([]Metro, int64, error) {
		return []Metro{}, 0, nil
	})
	if _, ok := getRegistry().Lookup("preexisting"); !ok {
		t.Error("empty-return hydrate wiped the dynamic registry")
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel test error" }

// TestCityHintFor covers the secondary-city → parent-metro map.
// Issue #406's Bellevue WA case: agents need to know that "bellevue"
// isn't its own metro but rolls into "seattle". Same pattern for
// Oakland, Cambridge, Brooklyn, etc.
func TestCityHintFor(t *testing.T) {
	cases := []struct {
		input    string
		wantHint string
	}{
		{"bellevue", "seattle"},
		{"BELLEVUE", "seattle"}, // case insensitive
		{"  bellevue  ", "seattle"},
		{"oakland", "san-francisco"},
		{"brooklyn", "new-york-city"},
		{"hoboken", "new-york-city"},
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

// TestFormatUnknownMetroError_BellevueHint verifies the three-layer
// helpful-error UX. Bellevue should produce the "lumped under seattle"
// suggestion (highest-quality signal), NOT just "did you mean".
func TestFormatUnknownMetroError_BellevueHint(t *testing.T) {
	got := formatUnknownMetroError("bellevue")
	for _, want := range []string{
		`unknown metro "bellevue"`,
		`lumped under metro "seattle"`,
		`--metro seattle --metro-radius-km 20`,
		`--latitude 47.6062 --longitude -122.3321`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("error missing expected substring %q\nfull error: %s", want, got)
		}
	}
}

// TestFormatUnknownMetroError_GibberishFallsBack verifies a totally
// unknown city (no hint, no name match) gets the count + sample
// fallback message instead of an empty/silent response.
func TestFormatUnknownMetroError_GibberishFallsBack(t *testing.T) {
	got := formatUnknownMetroError("xyz12345-not-a-place")
	if !strings.Contains(got, "unknown metro") {
		t.Errorf("missing 'unknown metro' prefix: %s", got)
	}
	if !strings.Contains(got, "metros known") {
		t.Errorf("missing count signal: %s", got)
	}
}

// TestFormatUnknownMetroError_DidYouMean verifies the middle-layer
// suggester fires when the input shares tokens with real metros but
// doesn't have a hint mapping. For "san-fransisco" (typo), it should
// suggest "san-francisco" via the alias chain or token match.
func TestFormatUnknownMetroError_DidYouMean(t *testing.T) {
	// Construct an input that won't match a hint but shares tokens
	// with a real metro slug. "san" appears in "san-francisco",
	// "san-diego", etc.
	got := formatUnknownMetroError("san-nowhere")
	if !strings.Contains(got, "did you mean") && !strings.Contains(got, "lumped under") {
		t.Errorf("expected 'did you mean' or 'lumped under' branch; got: %s", got)
	}
}
