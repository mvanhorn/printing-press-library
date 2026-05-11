// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// TestGeoContext_JSONRoundtrip pins the JSON contract — the response
// shape consumers depend on this being stable. New fields are
// additive (omitempty); existing fields keep their tags.
func TestGeoContext_JSONRoundtrip(t *testing.T) {
	gc := &GeoContext{
		Origin:     "bellevue, wa",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   25,
		Score:      0.91,
		Tier:       ResolutionTierHigh,
		Source:     SourceExplicitFlag,
		Alternates: []Candidate{
			{Name: "Bellevue, NE", State: "NE", ContextHints: []string{"Omaha metro"}, ScoreIfPicked: 0.18, Centroid: [2]float64{41.14, -95.91}},
		},
	}

	data, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Pin the new wire keys so a future rename trips the test.
	s := string(data)
	if !contains(s, `"score":0.91`) {
		t.Errorf("JSON missing \"score\":0.91; got %s", s)
	}
	if !contains(s, `"tier":"high"`) {
		t.Errorf("JSON missing \"tier\":\"high\"; got %s", s)
	}

	var got GeoContext
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Origin != gc.Origin || got.ResolvedTo != gc.ResolvedTo {
		t.Errorf("origin/resolved_to drift: %+v", got)
	}
	if got.Centroid != gc.Centroid {
		t.Errorf("centroid drift: got %v, want %v", got.Centroid, gc.Centroid)
	}
	if got.Source != gc.Source {
		t.Errorf("source drift: got %q, want %q", got.Source, gc.Source)
	}
	if got.Score != gc.Score {
		t.Errorf("score drift: got %v, want %v", got.Score, gc.Score)
	}
	if got.Tier != gc.Tier {
		t.Errorf("tier drift: got %q, want %q", got.Tier, gc.Tier)
	}
	if len(got.Alternates) != 1 || got.Alternates[0].Name != "Bellevue, NE" {
		t.Errorf("alternates drift: %+v", got.Alternates)
	}
}

// TestGeoContext_ForOpenTable verifies the OT projection extracts
// centroid lat/lng. The OT client surface accepts only lat/lng
// (MetroID is unused in v1).
func TestGeoContext_ForOpenTable(t *testing.T) {
	gc := &GeoContext{
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
	}
	got := gc.ForOpenTable()
	want := opentable.LocationInput{Lat: 47.6101, Lng: -122.2015}
	if got != want {
		t.Errorf("ForOpenTable: got %+v, want %+v", got, want)
	}
}

// TestGeoContext_ForTock verifies the Tock projection extracts city
// display name + slug from ResolvedTo plus lat/lng. Tock's SearchCity
// requires the City (display name) as both a query param and a path
// slug.
func TestGeoContext_ForTock(t *testing.T) {
	cases := []struct {
		name       string
		resolvedTo string
		wantCity   string
		wantSlug   string
	}{
		{"city + state", "Bellevue, WA", "Bellevue", "bellevue"},
		{"multi-word city", "New York City, NY", "New York City", "new-york-city"},
		{"bare city no comma", "Seattle", "Seattle", "seattle"},
		{"city + state with extra whitespace", "  Portland , OR  ", "Portland", "portland"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc := &GeoContext{
				ResolvedTo: tc.resolvedTo,
				Centroid:   [2]float64{47.6101, -122.2015},
			}
			got := gc.ForTock()
			if got.City != tc.wantCity {
				t.Errorf("city: got %q, want %q", got.City, tc.wantCity)
			}
			if got.Slug != tc.wantSlug {
				t.Errorf("slug: got %q, want %q", got.Slug, tc.wantSlug)
			}
			if got.Lat != 47.6101 || got.Lng != -122.2015 {
				t.Errorf("lat/lng drift: got (%v, %v)", got.Lat, got.Lng)
			}
		})
	}
}

// TestGeoContext_NilForProvider — nil GeoContext represents "no
// constraint." Calling ForOpenTable/ForTock/ForResy on nil returns a
// zero-value input. Caller is expected to check for nil before
// calling, but the methods are nil-safe defensively.
func TestGeoContext_NilForProvider(t *testing.T) {
	var gc *GeoContext
	if got := gc.ForOpenTable(); got != (opentable.LocationInput{}) {
		t.Errorf("nil.ForOpenTable: got %+v, want zero", got)
	}
	if got := gc.ForTock(); got != (tock.LocationInput{}) {
		t.Errorf("nil.ForTock: got %+v, want zero", got)
	}
	if got := gc.ForResy(); got != (resy.LocationInput{}) {
		t.Errorf("nil.ForResy: got %+v, want zero", got)
	}
}

// TestGeoContext_ForResy projects the typed GeoContext into Resy's
// LocationInput. Resy's /3/venuesearch/search uses a short city code
// (two/three letters) as the body field, so the projection is keyed
// off the resolved city display name via resyCityFromResolvedTo. Lat/
// Lng anchor client-side post-filtering since Resy dropped server-
// side `location` support in 2026 (see internal/source/resy.LocationInput).
func TestGeoContext_ForResy(t *testing.T) {
	cases := []struct {
		name       string
		resolvedTo string
		wantCity   string // empty means "no city filter"
	}{
		{"new york city", "New York City, NY", "ny"},
		{"manhattan folds to NY", "Manhattan, NY", "ny"},
		{"seattle", "Seattle, WA", "sea"},
		{"san francisco", "San Francisco, CA", "sf"},
		{"chicago", "Chicago, IL", "chi"},
		{"los angeles", "Los Angeles, CA", "la"},
		{"miami", "Miami, FL", "mia"},
		{"unknown city falls through to empty", "Bellevue, WA", ""},
		{"case-insensitive", "SEATTLE, WA", "sea"},
		{"whitespace tolerance", "  Portland , OR  ", "pdx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc := &GeoContext{
				ResolvedTo: tc.resolvedTo,
				Centroid:   [2]float64{40.7128, -74.0060},
			}
			got := gc.ForResy()
			if got.City != tc.wantCity {
				t.Errorf("city: got %q, want %q", got.City, tc.wantCity)
			}
			if got.Lat != 40.7128 || got.Lng != -74.0060 {
				t.Errorf("lat/lng drift: got (%v, %v)", got.Lat, got.Lng)
			}
		})
	}
}

// TestResyCityFromResolvedTo pins the city-code mapping table. Adding
// a new Resy metro means adding a row here AND in the
// resyCityFromResolvedTo switch — keep them in sync.
func TestResyCityFromResolvedTo(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"New York":          "ny",
		"New York City":     "ny",
		"Manhattan":         "ny",
		"Brooklyn":          "ny",
		"Queens":            "ny",
		"Seattle":           "sea",
		"Los Angeles":       "la",
		"San Francisco":     "sf",
		"Chicago":           "chi",
		"Miami":             "mia",
		"Boston":            "bos",
		"Washington":        "dc",
		"Washington, DC":    "dc",
		"Philadelphia":      "phi",
		"Austin":            "atx",
		"Houston":           "hou",
		"Dallas":            "dfw",
		"Atlanta":           "atl",
		"Denver":            "den",
		"Portland":          "pdx",
		"San Diego":         "sd",
		"Las Vegas":         "las",
		"Nashville":         "bna",
		"New Orleans":       "nola",
		"Minneapolis":       "msp",
		"Bellevue":          "", // not a Resy metro on its own
		"Springfield":       "", // ambiguous; never mapped
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := resyCityFromResolvedTo(in); got != want {
				t.Errorf("resyCityFromResolvedTo(%q) = %q; want %q", in, got, want)
			}
		})
	}
}

// TestGeoContext_SourceZeroValue — zero-value Source ("") is
// distinguishable from SourceDefault. Callers constructing
// GeoContext literals without setting Source land in the zero state;
// validation should not panic but Validate may treat as fine since
// the constraint is on Score, not Source.
func TestGeoContext_SourceZeroValue(t *testing.T) {
	gc := &GeoContext{}
	if gc.Source != Source("") {
		t.Errorf("zero-value Source: got %q, want empty", gc.Source)
	}
	if err := gc.Validate(); err != nil {
		t.Errorf("Validate on zero-value GeoContext: %v", err)
	}
}

// TestGeoContext_ValidateScore — Score must lie in [0, 1].
// Out-of-range values produce a typed error so the constructor can
// fail fast rather than emitting a malformed envelope.
func TestGeoContext_ValidateScore(t *testing.T) {
	cases := []struct {
		name    string
		score   float64
		wantErr bool
	}{
		{"zero", 0, false},
		{"half", 0.5, false},
		{"one", 1.0, false},
		{"slightly negative", -0.001, true},
		{"slightly over one", 1.001, true},
		{"way negative", -1, true},
		{"way over", 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc := &GeoContext{Score: tc.score}
			err := gc.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate(score=%v): err=%v, wantErr=%v", tc.score, err, tc.wantErr)
			}
		})
	}
}

// TestResolutionTier_Constants pins the wire values for the four
// agent-facing tier classifications. Agents and downstream consumers
// branch on these strings; renaming any of them is a breaking change.
func TestResolutionTier_Constants(t *testing.T) {
	cases := []struct {
		got, want ResolutionTier
	}{
		{ResolutionTierUnknown, "unknown"},
		{ResolutionTierLow, "low"},
		{ResolutionTierMedium, "medium"},
		{ResolutionTierHigh, "high"},
	}
	for _, tc := range cases {
		if string(tc.got) != string(tc.want) {
			t.Errorf("tier constant: got %q, want %q", tc.got, tc.want)
		}
	}
}

// TestSource_Constants — pin the wire values so JSON marshaling
// stays stable across refactors. Agents and downstream consumers
// branch on these strings.
func TestSource_Constants(t *testing.T) {
	cases := []struct {
		got, want Source
	}{
		{SourceExplicitFlag, "explicit_flag"},
		{SourceExtractedFromQuery, "extracted_from_query"},
		{SourceDefault, "default"},
	}
	for _, tc := range cases {
		if string(tc.got) != string(tc.want) {
			t.Errorf("source constant: got %q, want %q", tc.got, tc.want)
		}
	}
}

// TestCandidate_JSONShape — pin the candidate JSON contract for
// envelope consumers. tock_business_count is always emitted (no
// omitempty) so consumers can rely on its presence; ContextHints uses
// omitempty so absent hints don't pollute the response.
func TestCandidate_JSONShape(t *testing.T) {
	c := Candidate{
		Name:              "Bellevue, WA",
		State:             "WA",
		ContextHints:      []string{"Seattle metro", "Eastside"},
		TockBusinessCount: 28,
		ScoreIfPicked:     0.78,
		Centroid:          [2]float64{47.6101, -122.2015},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	// Field-name spot-checks; exact ordering may vary so substring is
	// the right granularity.
	for _, needle := range []string{`"name":"Bellevue, WA"`, `"state":"WA"`, `"context_hints":["Seattle metro","Eastside"]`, `"tock_business_count":28`, `"score_if_picked":0.78`, `"centroid":[47.6101,-122.2015]`} {
		if !contains(s, needle) {
			t.Errorf("candidate JSON missing %q in %s", needle, s)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
