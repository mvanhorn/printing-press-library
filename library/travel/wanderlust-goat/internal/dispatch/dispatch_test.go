package dispatch

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
)

func TestBuildJapanFood(t *testing.T) {
	anchor := AnchorRef{Query: "Park Hyatt Tokyo", Lat: 35.6856, Lng: 139.6913, Country: "JP", Display: "Park Hyatt Tokyo"}
	plan := Build(anchor, sources.IntentFood, []string{"kissaten", "no tourists"}, 1200)
	if plan.Country != "JP" {
		t.Errorf("country mismatch")
	}
	hasTabelog := false
	hasLeFooding := false
	hasReddit := false
	for _, c := range plan.Calls {
		if c.Client == "tabelog" {
			hasTabelog = true
			// Country boost should bring trust above the registry baseline.
			if c.ExpectedTrust < 0.94 {
				t.Errorf("tabelog should get +0.05 boost in JP, got trust=%v", c.ExpectedTrust)
			}
		}
		if c.Client == "lefooding" {
			hasLeFooding = true
		}
		if c.Client == "reddit" {
			hasReddit = true
			if c.Params["min_score"] != "10" {
				t.Errorf("reddit min_score should be 10, got %q", c.Params["min_score"])
			}
		}
	}
	if !hasTabelog {
		t.Error("JP food plan should include Tabelog")
	}
	if hasLeFooding {
		t.Error("JP plan should NOT include Le Fooding (FR canonical)")
	}
	if !hasReddit {
		t.Error("plan should include Reddit")
	}
	if len(plan.Sources) == 0 {
		t.Error("plan must list source slugs")
	}
}

func TestBuildOSRMExcluded(t *testing.T) {
	anchor := AnchorRef{Query: "Anywhere", Lat: 0, Lng: 0, Country: "*"}
	plan := Build(anchor, sources.IntentFood, nil, 500)
	for _, c := range plan.Calls {
		if c.Client == "osrm" {
			t.Error("OSRM is per-pair, not a search source — must not appear in plan")
		}
	}
}
