// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package zimmo

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func loadPage(t *testing.T) SearchResult {
	t.Helper()
	b, err := os.ReadFile("testdata/search_page.json")
	if err != nil {
		t.Fatal(err)
	}
	var r SearchResult
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestParseListingsFromRealPage(t *testing.T) {
	r := loadPage(t)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	var withHistory, rented int
	for _, raw := range r.Listings {
		l, err := Parse(raw, "fr", now)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(l.Code) != 5 {
			t.Errorf("zimmo code %q should be 5 chars", l.Code)
		}
		if l.PostalCode == "" || l.Status == "" || l.Type == "" {
			t.Errorf("%s: missing postcode/status/type: %+v", l.Code, l)
		}
		if l.URL == "" {
			t.Errorf("%s: no detail URL", l.Code)
		}
		if len(l.PriceHistory) > 0 {
			withHistory++
			if l.TotalCutPct == nil {
				t.Errorf("%s: price history without total cut", l.Code)
			}
		}
		if l.Rented {
			rented++
			if l.RentPerYear == nil || *l.RentPerYear <= 0 {
				t.Errorf("%s: rented without rent", l.Code)
			}
		}
		if l.Price != nil && l.Surface != nil && l.PricePerM2 == nil {
			t.Errorf("%s: price and surface but no price_per_m2", l.Code)
		}
	}
	if withHistory == 0 || rented == 0 {
		t.Fatalf("fixture should contain a price-history listing (%d) and a rented one (%d)", withHistory, rented)
	}
}

func TestFilterShapes(t *testing.T) {
	min, max := 200000, 400000
	c := Criteria{Categories: []string{"APARTMENT"}, PlaceIDs: []int{72}, MinPrice: &min, MaxPrice: &max, EPC: []string{"F", "G"}}
	f := c.Filter()
	st := f["status"].(map[string]any)["in"].([]string)
	if len(st) != 2 || st[0] != "FOR_SALE" || st[1] != "TAKE_OVER" {
		t.Errorf("default status should be FOR_SALE+TAKE_OVER, got %v", st)
	}
	pr := f["price"].(map[string]any)
	if pr["unknown"] != false {
		t.Errorf("range filters need unknown:false, got %v", pr)
	}
	if _, ok := f["polygon"]; ok {
		t.Error("no polygon expected")
	}
	c2 := Criteria{Statuses: []string{"SOLD"}, Polygon: SquareAround(50.83, 4.36, 500)}
	f2 := c2.Filter()
	poly := f2["polygon"].(map[string]any)["in"].([]any)[0].(map[string]any)["points"].([][]float64)
	if len(poly) != 5 || poly[0][0] != poly[4][0] {
		t.Errorf("polygon must be a closed ring of [lat,lon], got %v", poly)
	}
	if poly[0][0] > 51 || poly[0][0] < 50 {
		t.Errorf("first coordinate must be latitude, got %v", poly[0])
	}
	if st2 := f2["status"].(map[string]any)["in"].([]string); len(st2) != 1 {
		t.Errorf("SOLD must not add TAKE_OVER: %v", st2)
	}
}

func TestParseWords(t *testing.T) {
	cases := map[string]string{"maison": "HOUSE", "Appartement": "APARTMENT", "huis": "HOUSE", "terrain": "PLOT", "kot": "ROOM"}
	for in, want := range cases {
		got, err := ParseCategory(in)
		if err != nil || got != want {
			t.Errorf("ParseCategory(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if _, err := ParseCategory("castle-in-the-sky"); err == nil {
		t.Error("unknown category must error")
	}
	for in, want := range map[string]string{"rent": "TO_RENT", "à-louer": "TO_RENT", "vendu": "SOLD", "sale": "FOR_SALE"} {
		got, err := ParseStatus(in)
		if err != nil || got != want {
			t.Errorf("ParseStatus(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if got, err := ParseEPC("f, G,A++"); err != nil || len(got) != 3 || got[2] != "A" {
		t.Errorf("ParseEPC: %v %v", got, err)
	}
	if _, err := ParseEPC("H"); err == nil {
		t.Error("H is not an EPC letter")
	}
}

func TestParseSearchURL(t *testing.T) {
	c, err := ParseSearchURL("https://www.zimmo.be/fr/bruxelles-1000/a-vendre/appartement")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Postcodes) != 1 || c.Postcodes[0] != "1000" || c.Statuses[0] != "FOR_SALE" || c.Categories[0] != "APARTMENT" {
		t.Errorf("SEO path parse: %+v", c)
	}
	c, err = ParseSearchURL("https://www.zimmo.be/fr/recherche-avancee?search=eyJmaWx0ZXIiOnsic3RhdHVzIjp7ImluIjpbIkZPUl9TQUxFIl19fX0%3D")
	if err != nil {
		t.Fatal(err)
	}
	if c.Raw["status"] == nil {
		t.Errorf("search= param should decode into Raw filter: %+v", c)
	}
	c, err = ParseSearchURL("https://www.zimmo.be/fr/liege/a-vendre/maison")
	if err != nil || len(c.Communes) != 1 || c.Communes[0] != "liege" {
		t.Errorf("commune path: %+v %v", c, err)
	}
	if _, err := ParseSearchURL("https://www.immoweb.be/fr/recherche"); err == nil {
		t.Error("non-zimmo URL must be rejected")
	}
}

func TestStats(t *testing.T) {
	m, _ := Median([]float64{3, 1, 2, 10})
	if m != 2.5 {
		t.Errorf("median=%v", m)
	}
	q, _ := Quantile([]float64{1, 2, 3, 4, 5}, 0.25)
	if q != 2 {
		t.Errorf("p25=%v", q)
	}
	if d := DistanceM(50.85, 4.35, 50.85, 4.36); d < 690 || d > 720 {
		t.Errorf("distance=%v", d)
	}
	if jwtExpiry("a.b") != (time.Time{}) {
		t.Error("bad jwt must give zero expiry")
	}
}

func TestTitleCaseAccent(t *testing.T) {
	if got := titleCase("ÉGHEZÉE"); got != "Éghezée" {
		t.Errorf("titleCase must keep a leading accented capital: %q", got)
	}
	c, err := ParseSearchURL("https://www.zimmo.be/fr/recherche-avancee?search=eyJmaWx0ZXIiOnsic3RhdHVzIjp7ImluIjpbIkZPUl9TQUxFIl19fX0=")
	if err != nil || c.Raw == nil {
		t.Errorf("unescaped base64 padding/plus must decode: %v", err)
	}
}
