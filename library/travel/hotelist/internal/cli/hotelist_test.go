package cli

import (
	"math"
	"testing"
)

func TestBuildAPIParamsNestedSerialization(t *testing.T) {
	filters := []apiFilter{
		filterGeohash("ezdn"),
		filterCountry("Spain"),
		filterChain("EM"),
		filterMinRating(8),
		filterMaxPrice(150),
		filterSort("best-value", "desc"),
		filterBbox([4]float64{34, 72, -25, 45}),
	}
	p := buildAPIParams(filters, "marriott")
	checks := map[string]string{
		"filters[0][target]":         "geohash",
		"filters[0][value]":          "ezdn",
		"filters[0][type]":           "starts_with",
		"filters[1][target]":         "country",
		"filters[1][value]":          "Spain",
		"filters[2][value]":          "EM",
		"filters[3][target]":         "hotellist_rating",
		"filters[3][value]":          "8",
		"filters[4][value]":          "150",
		"filters[5][value][key]":     "best-value",
		"filters[5][value][order]":   "desc",
		"filters[6][value][lat_min]": "34",
		"filters[6][value][lng_max]": "45",
		"search":                     "marriott",
	}
	for k, want := range checks {
		if got := p[k]; got != want {
			t.Errorf("param %q = %q, want %q", k, got, want)
		}
	}
	// sort-by must not carry a top-level [type]
	if _, ok := p["filters[5][type]"]; ok {
		t.Error("sort-by should not emit a [type] key")
	}
}

func TestNormalizeChain(t *testing.T) {
	cases := []struct {
		in       string
		wantOK   bool
		wantCode string
	}{
		{"marriott", true, "EM"},
		{"Marriott", true, "EM"},
		{"EM", true, "EM"},
		{"four seasons", true, "FS"},
		{"ritz-carlton", true, "RZ"},
		{"nonsense", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		code, _, ok := normalizeChain(c.in)
		if ok != c.wantOK || code != c.wantCode {
			t.Errorf("normalizeChain(%q) = (%q,%v), want (%q,%v)", c.in, code, ok, c.wantCode, c.wantOK)
		}
	}
}

func TestAmenityLabel(t *testing.T) {
	cases := map[string]string{
		"weights":       "Weightlifting gym",
		"gym":           "Gym",
		"pool":          "Pool",
		"tennis":        "Tennis court",
		"coworking":     "Coworking",
		"bathtub":       "Bath",
		"something-new": "Something-New",
		"":              "",
	}
	for in, want := range cases {
		if got := amenityLabel(in); got != want {
			t.Errorf("amenityLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMakeSlug(t *testing.T) {
	cases := map[string]string{
		"Bangkok":       "bangkok",
		"New York City": "new-york-city",
		"  São Paulo  ": "s-o-paulo",
		"A Coruna":      "a-coruna",
	}
	for in, want := range cases {
		if got := makeSlug(in); got != want {
			t.Errorf("makeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValueScoreAndSort(t *testing.T) {
	hs := []hlHotel{
		{HotelID: "a", Rating: 9, Price: 0},   // no price -> sinks
		{HotelID: "b", Rating: 8, Price: 100}, // value 0.08
		{HotelID: "c", Rating: 9, Price: 90},  // value 0.10 (best)
	}
	sortHotelsByValue(hs)
	if hs[0].HotelID != "c" || hs[1].HotelID != "b" || hs[2].HotelID != "a" {
		t.Errorf("sortHotelsByValue order = %v, want c,b,a", []string{hs[0].HotelID, hs[1].HotelID, hs[2].HotelID})
	}
}

func TestIsExceptional(t *testing.T) {
	y2020 := 2020
	y2000 := 2000
	cases := []struct {
		h    hlHotel
		want bool
	}{
		{hlHotel{Rating: 9, YearBuilt: &y2020}, true},
		{hlHotel{Rating: 9, YearBuilt: &y2000}, false},   // too old
		{hlHotel{Rating: 7.5, YearBuilt: &y2020}, false}, // below 8
		{hlHotel{Rating: 8.5, YearBuilt: nil}, true},     // unknown year, high score
	}
	for i, c := range cases {
		if got := isExceptional(c.h); got != c.want {
			t.Errorf("case %d isExceptional = %v, want %v", i, got, c.want)
		}
	}
}

func TestParseHotelModal(t *testing.T) {
	html := `<h1>Avenida</h1>
	<div class="tab tab-ratings active"><table>
	<tr><td class="key">🏩 Hotelist Score</td><td class="value"><div class="rating r9"><div class="filling" style="width:91%">9.1</div></div><div class="last-updated">1mo ago (2026-04-12)</div></td></tr>
	<tr><td class="key">📸 AI rating of photos</td><td class="value"><div class="rating r8"><div class="filling" style="width:80%">8.0</div></div></td></tr>
	</table></div>
	<div class="tab tab-amenities">
	<div class="amenity"><div class="amenity-icon"></div><div class="amenity-name">Restaurant</div></div>
	<div class="amenity"><div class="amenity-icon"></div><div class="amenity-name">Parking</div></div>
	</div>`
	d := parseHotelModal("KYLCGAVE", html)
	if d.Name != "Avenida" {
		t.Errorf("name = %q, want Avenida", d.Name)
	}
	if len(d.Ratings) != 2 {
		t.Fatalf("ratings = %d, want 2", len(d.Ratings))
	}
	if d.Ratings[0].Value != 9.1 {
		t.Errorf("first rating value = %v, want 9.1", d.Ratings[0].Value)
	}
	if d.Ratings[0].Updated == "" {
		t.Error("expected last-updated on first rating")
	}
	if len(d.Amenities) != 2 || d.Amenities[0] != "Restaurant" {
		t.Errorf("amenities = %v, want [Restaurant Parking]", d.Amenities)
	}
}

func TestStatsHelpers(t *testing.T) {
	xs := []float64{8, 9, 7, 10}
	if got := meanF(xs); math.Abs(got-8.5) > 1e-9 {
		t.Errorf("meanF = %v, want 8.5", got)
	}
	if got := medianF(xs); math.Abs(got-8.5) > 1e-9 {
		t.Errorf("medianF = %v, want 8.5", got)
	}
	if got := stddevF([]float64{5, 5, 5}); got != 0 {
		t.Errorf("stddevF of constant = %v, want 0", got)
	}
	mn, mx := minMaxF(xs)
	if mn != 7 || mx != 10 {
		t.Errorf("minMaxF = (%v,%v), want (7,10)", mn, mx)
	}
}

func TestResolveSort(t *testing.T) {
	if f, ok := resolveSort("value"); !ok || f.SortKey != "best-value" {
		t.Errorf("resolveSort(value) = (%+v,%v)", f, ok)
	}
	if f, ok := resolveSort("price"); !ok || f.SortKey != "price" || f.SortOrder != "asc" {
		t.Errorf("resolveSort(price) = (%+v,%v)", f, ok)
	}
	if _, ok := resolveSort("bogus"); ok {
		t.Error("resolveSort(bogus) should be !ok")
	}
}
