// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"math"
	"strings"
	"testing"
)

func TestParseListing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		ok   bool
		want Listing
	}{
		{
			name: "peerspace search hit shape",
			raw: `{
				"id":"abc123","_id":"abc123","title":"Loft Paris",
				"city":"PARIS","neighborhood":"Ménilmontant",
				"number_guests":90,"attendees_max":90,
				"price.hourly":150.0,
				"is_instant_book_active":true,
				"space_review_stars":5.0,"space_review_count":4,
				"canonical_amenities":["wifi","projector","public_transit","chairs"],
				"description":"Great wifi and projector for meetups",
				"location":{"lat":48.86,"lon":2.39},
				"detailed_pricing":{"booking_rate":161.0,"space_rental":4347.0}
			}`,
			ok: true,
			want: Listing{
				ID: "abc123", Title: "Loft Paris", City: "PARIS",
				Neighborhood: "Ménilmontant", Guests: 90, PriceHourly: 161,
				InstantBook: true, ReviewStars: 5, ReviewCount: 4,
			},
		},
		{
			name: "nested space_rental booking_rate",
			raw: `{
				"id":"x1","title":"Studio",
				"detailed_pricing":{"space_rental":{"booking_rate":80.5}},
				"number_guests":20
			}`,
			ok: true,
			want: Listing{ID: "x1", Title: "Studio", Guests: 20, PriceHourly: 80.5},
		},
		{
			name: "empty object",
			raw:  `{}`,
			ok:   false,
		},
		{
			name: "amenities as objects",
			raw: `{
				"id":"y","title":"Y",
				"canonical_amenities":[{"name":"wifi","display_name":"WiFi"},{"display_name":"Projector"}]
			}`,
			ok:   true,
			want: Listing{ID: "y", Title: "Y"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseListing([]byte(tc.raw))
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if got.ID != tc.want.ID || got.Title != tc.want.Title {
				t.Fatalf("id/title got %#v want %#v", got, tc.want)
			}
			if tc.want.Guests != 0 && got.Guests != tc.want.Guests {
				t.Fatalf("guests=%d want %d", got.Guests, tc.want.Guests)
			}
			if tc.want.PriceHourly != 0 && math.Abs(got.PriceHourly-tc.want.PriceHourly) > 0.01 {
				t.Fatalf("price=%v want %v", got.PriceHourly, tc.want.PriceHourly)
			}
			if tc.want.InstantBook && !got.InstantBook {
				t.Fatalf("expected instant book")
			}
			if tc.name == "amenities as objects" {
				if len(got.Amenities) < 2 {
					t.Fatalf("amenities=%v", got.Amenities)
				}
			}
		})
	}
}

func TestExpandHits(t *testing.T) {
	t.Parallel()
	raw := `{"hits":{"total":2,"hits":[
		{"_id":"a","title":"A","number_guests":10,"price.hourly":50},
		{"_id":"b","_source":{"id":"b","title":"B","number_guests":20,"price.hourly":100}}
	]}}`
	got := ExpandResourceData("search-1", []byte(raw))
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (%+v)", len(got), got)
	}
}

func TestBandPrices(t *testing.T) {
	t.Parallel()
	listings := []Listing{
		{ID: "a", PriceHourly: 10},
		{ID: "b", PriceHourly: 55},
		{ID: "c", PriceHourly: 99},
		{ID: "d", PriceHourly: 100},
		{ID: "e", PriceHourly: 0}, // skipped
	}
	bands := BandPrices(listings, 50)
	if len(bands) != 3 {
		t.Fatalf("bands=%d want 3: %+v", len(bands), bands)
	}
	// 10 → [0,50), 55+99 → [50,100), 100 → [100,150)
	if bands[0].Count != 1 || bands[0].Min != 0 || bands[0].Max != 50 {
		t.Fatalf("band0=%+v", bands[0])
	}
	if bands[1].Count != 2 {
		t.Fatalf("band1 count=%d", bands[1].Count)
	}
	if bands[2].Count != 1 || bands[2].Min != 100 {
		t.Fatalf("band2=%+v", bands[2])
	}
}

func TestBandCapacity(t *testing.T) {
	t.Parallel()
	listings := []Listing{
		{ID: "a", Guests: 5},
		{ID: "b", Guests: 15},
		{ID: "c", Guests: 25},
		{ID: "d", Guests: 0},
	}
	bands := BandCapacity(listings, 10)
	if len(bands) != 3 {
		t.Fatalf("bands=%+v", bands)
	}
	if bands[0].Min != 0 || bands[0].Max != 10 || bands[0].Count != 1 {
		t.Fatalf("band0=%+v", bands[0])
	}
}

func TestScoreTechFit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		l         Listing
		guests    int
		budget    float64
		vibe      []string
		minScore  int
		wantGap   string
		noGap     string
	}{
		{
			name: "strong tech fit",
			l: Listing{
				ID: "1", Guests: 50, PriceHourly: 120,
				Description: "wifi projector chairs public transit late evening",
				Amenities:   []string{"wifi", "projector", "chairs", "public_transit"},
				ReviewStars: 5, ReviewCount: 10, InstantBook: true,
			},
			guests:   40,
			budget:   180,
			minScore: 70,
			noGap:    "wifi",
		},
		{
			name: "over budget under capacity",
			l: Listing{
				ID: "2", Guests: 10, PriceHourly: 300,
				Description: "empty room",
			},
			guests:   40,
			budget:   100,
			minScore: 0,
			wantGap:  "over_budget",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc, gaps := ScoreTechFit(tc.l, tc.guests, tc.budget, tc.vibe)
			if sc < tc.minScore {
				t.Fatalf("score=%d want >= %d gaps=%v", sc, tc.minScore, gaps)
			}
			if tc.wantGap != "" {
				found := false
				for _, g := range gaps {
					if g == tc.wantGap {
						found = true
					}
				}
				if !found {
					t.Fatalf("gaps=%v missing %q", gaps, tc.wantGap)
				}
			}
			if tc.noGap != "" {
				for _, g := range gaps {
					if g == tc.noGap {
						t.Fatalf("unexpected gap %q in %v", tc.noGap, gaps)
					}
				}
			}
		})
	}
}

func TestGapChecklist(t *testing.T) {
	t.Parallel()
	l := Listing{
		Description: "Has WiFi and chairs only",
		Amenities:   []string{"wifi", "chairs"},
	}
	gaps := GapChecklist(l, "tech-meetup")
	has := map[string]bool{}
	for _, g := range gaps {
		has[g] = true
	}
	if has[GapWiFi] {
		t.Fatalf("wifi should be present, gaps=%v", gaps)
	}
	if !has[GapProjectorAV] {
		t.Fatalf("projector_av should be missing, gaps=%v", gaps)
	}
	if !has[GapTransit] {
		t.Fatalf("transit should be missing, gaps=%v", gaps)
	}
}

func TestMedian(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"one", []float64{3}, 3},
		{"odd", []float64{1, 3, 2}, 2},
		{"even", []float64{1, 2, 3, 4}, 2.5},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Median(tc.in)
			if got != tc.want {
				t.Fatalf("Median(%v)=%v want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestDeltaIDs(t *testing.T) {
	t.Parallel()
	d := DeltaIDs([]string{"a", "b"}, []string{"b", "c"})
	if len(d.Added) != 1 || d.Added[0] != "c" {
		t.Fatalf("added=%v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0] != "a" {
		t.Fatalf("removed=%v", d.Removed)
	}
	if len(d.Kept) != 1 || d.Kept[0] != "b" {
		t.Fatalf("kept=%v", d.Kept)
	}
}

func TestExtractFavoriteIDs(t *testing.T) {
	t.Parallel()
	raw := `{"attachments":[{"value":"id1"},{"value":"id2"},{"value":"id1"}]}`
	ids := ExtractFavoriteIDs([]byte(raw))
	if len(ids) != 2 {
		t.Fatalf("ids=%v", ids)
	}
}

func TestExportMarkdown(t *testing.T) {
	t.Parallel()
	md := ExportMarkdown([]Listing{{ID: "1", Title: "Loft", City: "Paris", PriceHourly: 100, Guests: 40, Amenities: []string{"wifi"}}})
	if md == "" || !strings.Contains(md, "Loft") || !strings.Contains(md, "Paris") {
		t.Fatalf("md=%q", md)
	}
}
