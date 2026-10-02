package smartex

import "testing"

func TestProductComparisonKnownRouteRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, class, train, product string
		excluded                              bool
	}{
		{"sanyo_ordinary", "Shin-Osaka", "Hiroshima", "reserved", "nozomi", "hayatoku3", true},
		{"tokaido_sanyo_ordinary", "Tokyo", "Hiroshima", "reserved", "nozomi", "hayatoku3", true},
		{"tokaido_ordinary", "Tokyo", "Shin-Osaka", "reserved", "nozomi", "hayatoku3", true},
		{"sanyo_green_unknown_inventory", "Shin-Osaka", "Hiroshima", "green", "nozomi", "hayatoku3", false},
		{"kyushu_ordinary_unknown_inventory", "Hakata", "Kumamoto", "reserved", "sakura", "hayatoku3", false},
		{"sanyo_kyushu_transfer_rules_unknown", "Shin-Osaka", "Kumamoto", "reserved", "sakura", "hayatoku3", false},
		{"unspecified_route_unknown", "", "", "reserved", "", "hayatoku3", false},
		{"tsubame_outside_two_corridor_route", "Tokyo", "Hakata", "reserved", "tsubame", "smart-ex", true},
		{"tsubame_outside_single_corridor_route", "Shin-Osaka", "Hiroshima", "reserved", "tsubame", "smart-ex", true},
		{"tsubame_in_kyushu_coverage_unknown_through_service", "Tokyo", "Kumamoto", "reserved", "tsubame", "smart-ex", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			views, err := CompareProducts("", tc.from, tc.to, tc.class, tc.train, mustNow("2026-10-02T12:00:00+09:00"), 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range views {
				if v.ID != tc.product {
					continue
				}
				if got := v.Assessment == "excluded_by_checked_conditions"; got != tc.excluded {
					t.Fatalf("%s assessment=%s, reasons=%v; excluded=%v", v.ID, v.Assessment, v.ExclusionReasons, tc.excluded)
				}
				if !tc.excluded && (v.Assessment != "requires_confirmation" || len(v.RequiresConfirmation) == 0) {
					t.Fatalf("compatible coverage must preserve unknown eligibility/inventory: %+v", v)
				}
				if v.PriceJPY != nil {
					t.Fatal("a route/class check must not invent a discount price")
				}
				return
			}
			t.Fatalf("product %s absent from comparison", tc.product)
		})
	}
}
