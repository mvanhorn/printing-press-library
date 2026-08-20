package cli

import (
	"testing"
	"time"
)

func TestPctOff(t *testing.T) {
	cases := []struct {
		name          string
		strike, price float64
		want          int
	}{
		{"normal discount", 587, 188, 68},
		{"no discount when price >= strike", 100, 120, 0},
		{"zero strike", 0, 100, 0},
		{"zero price", 100, 0, 0},
		{"half off", 200, 100, 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pctOff(c.strike, c.price); got != c.want {
				t.Errorf("pctOff(%v, %v) = %d, want %d", c.strike, c.price, got, c.want)
			}
		})
	}
}

func TestParseCategories(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "luxe", []string{"luxe"}, false},
		{"multi mixed case", "Luxe,HIP", []string{"luxe", "hip"}, false},
		{"crashpad with space", "crash pad", []string{"crashpad"}, false},
		{"unknown tier", "platinum", nil, true},
		{"one bad in list", "luxe,bogus", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseCategories(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseCategories(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if len(got) != len(c.want) {
				t.Fatalf("parseCategories(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("parseCategories(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestMatchesCategories(t *testing.T) {
	cases := []struct {
		hotelCat string
		want     []string
		match    bool
	}{
		{"Luxe", nil, true},              // empty want matches all
		{"Luxe", []string{"luxe"}, true}, // case-insensitive
		{"Basic", []string{"luxe", "hip"}, false},
		{"CrashPad", []string{"crashpad"}, true},
	}
	for _, c := range cases {
		if got := matchesCategories(c.hotelCat, c.want); got != c.match {
			t.Errorf("matchesCategories(%q, %v) = %v, want %v", c.hotelCat, c.want, got, c.match)
		}
	}
}

func TestComputeDates(t *testing.T) {
	// Friday 2026-05-22 as the reference "now".
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC) // Wednesday
	cases := []struct {
		when            string
		wantIn, wantOut string
	}{
		{"tonight", "", ""},
		{"", "", ""},
		{"tomorrow", "2026-05-21", "2026-05-22"},
		{"weekend", "2026-05-22", "2026-05-24"}, // upcoming Friday -> Sunday
		{"garbage", "", ""},
	}
	for _, c := range cases {
		t.Run(c.when, func(t *testing.T) {
			gotIn, gotOut := computeDates(c.when, now)
			if gotIn != c.wantIn || gotOut != c.wantOut {
				t.Errorf("computeDates(%q) = (%q, %q), want (%q, %q)", c.when, gotIn, gotOut, c.wantIn, c.wantOut)
			}
		})
	}
}

func TestSortDeals(t *testing.T) {
	deals := []htDeal{
		{HotelName: "A", Price: 200, PctOff: 10},
		{HotelName: "B", Price: 100, PctOff: 50},
		{HotelName: "C", Price: 150, PctOff: 30},
	}
	byPrice := append([]htDeal(nil), deals...)
	sortDeals(byPrice, "price")
	if byPrice[0].HotelName != "B" || byPrice[2].HotelName != "A" {
		t.Errorf("sort by price = %s,%s,%s; want B,C,A", byPrice[0].HotelName, byPrice[1].HotelName, byPrice[2].HotelName)
	}
	byDiscount := append([]htDeal(nil), deals...)
	sortDeals(byDiscount, "discount")
	if byDiscount[0].HotelName != "B" || byDiscount[2].HotelName != "A" {
		t.Errorf("sort by discount = %s,%s,%s; want B,C,A", byDiscount[0].HotelName, byDiscount[1].HotelName, byDiscount[2].HotelName)
	}
}

func TestGroupByNeighborhood(t *testing.T) {
	deals := []htDeal{
		{HotelName: "A", Neighborhood: "Downtown", Price: 200, PctOff: 10},
		{HotelName: "B", Neighborhood: "Downtown", Price: 100, PctOff: 50},
		{HotelName: "C", Neighborhood: "Beach", Price: 150, PctOff: 30},
		{HotelName: "D", Neighborhood: "", Price: 90, PctOff: 5},
	}
	groups := groupByNeighborhood(deals)
	byName := map[string]htNeighborhood{}
	for _, g := range groups {
		byName[g.Neighborhood] = g
	}
	if dt := byName["Downtown"]; dt.DealCount != 2 || dt.MinPrice != 100 || dt.BestPctOff != 50 || dt.CheapestHotel != "B" {
		t.Errorf("Downtown rollup wrong: %+v", dt)
	}
	if _, ok := byName["(unknown)"]; !ok {
		t.Error("expected unknown-neighborhood bucket for blank neighborhood")
	}
}

func TestValidateCoord(t *testing.T) {
	if err := validateCoord("lat", "37.7749", -90, 90); err != nil {
		t.Errorf("valid lat rejected: %v", err)
	}
	if err := validateCoord("lng", "-122.4194", -180, 180); err != nil {
		t.Errorf("valid lng rejected: %v", err)
	}
	if err := validateCoord("lat", "example-value", -90, 90); err == nil {
		t.Error("non-numeric coord accepted")
	}
	if err := validateCoord("lat", "200", -90, 90); err == nil {
		t.Error("out-of-range coord accepted")
	}
}

func TestResolveGeoCoordValidation(t *testing.T) {
	// nil client is safe here: the lat/lng path validates and returns before
	// touching the client.
	if _, _, _, err := resolveGeo(nil, 0, "37.7749", "-122.4194"); err != nil {
		t.Errorf("valid coords rejected: %v", err)
	}
	if _, _, _, err := resolveGeo(nil, 0, "example-value", "example-value"); err == nil {
		t.Error("non-numeric coords accepted")
	}
	if _, _, _, err := resolveGeo(nil, 0, "37.7749", ""); err == nil {
		t.Error("missing --lng accepted")
	}
	if _, _, _, err := resolveGeo(nil, 0, "", ""); err == nil {
		t.Error("no geo source accepted")
	}
}
