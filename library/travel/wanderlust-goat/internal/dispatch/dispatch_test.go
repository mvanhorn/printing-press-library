package dispatch

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/googleplaces"
)

// fakeSeed implements SeedClient with canned data; used to drive the
// dispatcher end-to-end without hitting Google.
type fakeSeed struct {
	places []googleplaces.Place
}

func (f *fakeSeed) NearbySearch(ctx context.Context, lat, lng, radiusMeters float64, includedTypes []string, maxResults int, languageCode string) ([]googleplaces.Place, error) {
	return f.places, nil
}

func TestRun_DropsClosedPermanently(t *testing.T) {
	seed := &fakeSeed{
		places: []googleplaces.Place{
			{ID: "a", DisplayName: "Open Spot", Lat: 35.6863, Lng: 139.6906, Rating: 4.5, UserRatingCount: 200, BusinessStatus: "OPERATIONAL"},
			// Stage-1 client filter would normally drop this, but we
			// verify defense-in-depth at the dispatcher level too.
			{ID: "b", DisplayName: "Dead Spot", Lat: 35.6864, Lng: 139.6907, BusinessStatus: "CLOSED_PERMANENTLY"},
		},
	}
	d := NewWithSeed(seed, NewRegistry()) // empty registry; no Stage-2 wiring
	plan := Plan{
		Anchor: "Park Hyatt Tokyo",
		Resolved: AnchorResolution{
			Lat: 35.6863, Lng: 139.6906, Country: "JP", Display: "Park Hyatt Tokyo, Shinjuku, Tokyo, Japan",
		},
		Criteria:      "kissaten",
		RadiusMinutes: 15,
		SeedLimit:     5,
	}
	results, _, err := d.Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.BusinessStatus == "CLOSED_PERMANENTLY" {
			t.Errorf("CLOSED_PERMANENTLY survived: %s", r.Name)
		}
	}
}

func TestRun_RanksByScore(t *testing.T) {
	seed := &fakeSeed{
		places: []googleplaces.Place{
			{ID: "low", DisplayName: "Low Rated", Lat: 35.0, Lng: 139.0, Rating: 2.0, UserRatingCount: 10, BusinessStatus: "OPERATIONAL"},
			{ID: "high", DisplayName: "High Rated", Lat: 35.0, Lng: 139.0, Rating: 4.8, UserRatingCount: 1000, BusinessStatus: "OPERATIONAL"},
			{ID: "mid", DisplayName: "Mid Rated", Lat: 35.0, Lng: 139.0, Rating: 4.0, UserRatingCount: 100, BusinessStatus: "OPERATIONAL"},
		},
	}
	d := NewWithSeed(seed, NewRegistry())
	plan := Plan{Resolved: AnchorResolution{Lat: 35.0, Lng: 139.0, Country: "*"}, RadiusMinutes: 15, SeedLimit: 10}
	results, _, err := d.Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "High Rated" {
		t.Errorf("first = %q, want High Rated", results[0].Name)
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Score.Total < results[i].Score.Total {
			t.Errorf("results not sorted desc: %v < %v", results[i-1].Score.Total, results[i].Score.Total)
		}
	}
}

func TestRun_WalkingMinutesPopulated(t *testing.T) {
	seed := &fakeSeed{
		places: []googleplaces.Place{
			{ID: "near", DisplayName: "Right Here", Lat: 35.6863, Lng: 139.6906, Rating: 4.5, BusinessStatus: "OPERATIONAL"},
			{ID: "far", DisplayName: "Around Corner", Lat: 35.6890, Lng: 139.6940, Rating: 4.3, BusinessStatus: "OPERATIONAL"},
		},
	}
	d := NewWithSeed(seed, NewRegistry())
	plan := Plan{Resolved: AnchorResolution{Lat: 35.6863, Lng: 139.6906, Country: "JP"}, RadiusMinutes: 30, SeedLimit: 10}
	results, _, _ := d.Run(context.Background(), plan)
	for _, r := range results {
		if r.WalkingMinutes < 0 {
			t.Errorf("negative walking minutes: %v", r.WalkingMinutes)
		}
	}
}

func TestParseLatLng(t *testing.T) {
	tests := []struct {
		in     string
		wantOK bool
	}{
		{"35.6895,139.6917", true},
		{"35.6895, 139.6917", true},
		{"  35.0 , 139.0  ", true},
		{"abc,def", false},
		{"100,200", false}, // out of range
		{"Paris", false},
	}
	for _, tt := range tests {
		_, _, ok := parseLatLng(tt.in)
		if ok != tt.wantOK {
			t.Errorf("parseLatLng(%q) ok=%v, want %v", tt.in, ok, tt.wantOK)
		}
	}
}

func TestRegistryDefault_HasAllSlugs(t *testing.T) {
	r := DefaultRegistry()
	expected := []string{
		"tabelog", "retty", "hotpepper", "navermap", "naverblog", "lefooding",
		"notecom", "hatena", "kakaomap", "mangoplate", "pudlo", "lafourchette",
		"gamberorosso", "slowfood", "dissapore", "falstaff", "derfeinschmecker",
		"verema", "eltenedor", "squaremeal", "hotdinners", "observerfood",
		"dianping", "mafengwo", "xiaohongshu",
	}
	for _, slug := range expected {
		if r.Get(slug) == nil {
			t.Errorf("DefaultRegistry missing %q", slug)
		}
	}
	if got := len(r.Slugs()); got < len(expected) {
		t.Errorf("DefaultRegistry has %d slugs, want >= %d", got, len(expected))
	}
}
