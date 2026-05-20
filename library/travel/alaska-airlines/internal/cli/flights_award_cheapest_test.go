// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-19: award-cheapest planner) — unit tests for the
// pure helpers introduced by the award-cheapest planner.

package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestResolveDestinations_Region(t *testing.T) {
	got, err := resolveDestinations("", "japan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"HND", "NRT", "KIX", "ITM", "NGO", "FUK", "CTS", "OKA"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("japan region = %v, want %v", got, want)
	}
}

func TestResolveDestinations_RegionUppercase(t *testing.T) {
	got, err := resolveDestinations("", "JAPAN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 8 {
		t.Errorf("expected 8 codes from JAPAN region, got %d", len(got))
	}
}

func TestResolveDestinations_UnknownRegion(t *testing.T) {
	_, err := resolveDestinations("", "atlantis")
	if err == nil {
		t.Fatal("expected error for unknown region")
	}
	if !strings.Contains(err.Error(), "unknown --destination-region") {
		t.Errorf("error should mention unknown region, got: %v", err)
	}
}

func TestResolveDestinations_CommaList(t *testing.T) {
	got, err := resolveDestinations("hnd, NRT,kix", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"HND", "NRT", "KIX"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("comma list = %v, want %v", got, want)
	}
}

func TestResolveDestinations_DedupeAndTrim(t *testing.T) {
	got, _ := resolveDestinations("HND,HND, hnd , NRT", "")
	want := []string{"HND", "NRT"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupe failed: got %v, want %v", got, want)
	}
}

func TestResolveDestinations_Empty(t *testing.T) {
	got, err := resolveDestinations("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestResolveDateWindows_Month(t *testing.T) {
	dep, ret, err := resolveDateWindows("2026-08", "", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.from != "2026-08-01" || dep.to != "2026-08-31" {
		t.Errorf("depart window = [%s, %s], want [2026-08-01, 2026-08-31]", dep.from, dep.to)
	}
	if ret.from == "" || ret.to == "" {
		t.Errorf("return window empty for round-trip month: dep=%v ret=%v", dep, ret)
	}
}

func TestResolveDateWindows_OneWay(t *testing.T) {
	dep, ret, err := resolveDateWindows("2026-08", "", "", "", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.from != "2026-08-01" {
		t.Errorf("depart from = %s, want 2026-08-01", dep.from)
	}
	if ret.from != "" || ret.to != "" {
		t.Errorf("one-way should have empty return window, got: %v", ret)
	}
}

func TestResolveDateWindows_ExplicitOverridesMonth(t *testing.T) {
	dep, _, err := resolveDateWindows("2026-08", "2026-08-15", "2026-08-20", "2026-08-22", "2026-09-05", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.from != "2026-08-15" || dep.to != "2026-08-20" {
		t.Errorf("explicit override failed: got [%s, %s]", dep.from, dep.to)
	}
}

func TestResolveDateWindows_InvalidMonth(t *testing.T) {
	_, _, err := resolveDateWindows("not-a-month", "", "", "", "", false)
	if err == nil {
		t.Fatal("expected error for invalid month")
	}
}

func TestResolveDateWindows_MissingWindow(t *testing.T) {
	_, _, err := resolveDateWindows("", "", "", "", "", false)
	if err == nil {
		t.Fatal("expected error when no window inputs given")
	}
}

func TestEnumerateDatePairs_RoundTrip(t *testing.T) {
	dep := dateWindow{from: "2026-08-15", to: "2026-08-17"}
	ret := dateWindow{from: "2026-08-20", to: "2026-09-10"}
	pairs := enumerateDatePairs(dep, ret, 5, 7, false)
	// 3 depart dates x 3 night options (5,6,7) = 9 candidates, all
	// fall within return window.
	if len(pairs) != 9 {
		t.Errorf("expected 9 pairs, got %d: %+v", len(pairs), pairs)
	}
}

func TestEnumerateDatePairs_OneWay(t *testing.T) {
	dep := dateWindow{from: "2026-08-15", to: "2026-08-18"}
	pairs := enumerateDatePairs(dep, dateWindow{}, 5, 21, true)
	if len(pairs) != 4 {
		t.Errorf("expected 4 one-way pairs, got %d", len(pairs))
	}
	for _, p := range pairs {
		if p.ret != "" {
			t.Errorf("one-way pair should have empty return, got %s", p.ret)
		}
	}
}

func TestEnumerateDatePairs_ReturnOutOfWindow(t *testing.T) {
	dep := dateWindow{from: "2026-08-15", to: "2026-08-15"}
	ret := dateWindow{from: "2026-09-01", to: "2026-09-30"}
	// min=5 max=21 → return would be 2026-08-20..2026-09-05; only 2026-09-01..2026-09-05 fall in window.
	// That's nights 17,18,19,20,21 = 5 valid.
	pairs := enumerateDatePairs(dep, ret, 5, 21, false)
	if len(pairs) != 5 {
		t.Errorf("expected 5 pairs in clipped return window, got %d: %+v", len(pairs), pairs)
	}
}

func TestExtractLowestAwardPrice_NoData(t *testing.T) {
	got := extractLowestAwardPrice(json.RawMessage(`{}`), "", -1)
	if got.Miles != nil {
		t.Errorf("empty doc should yield nil miles, got %+v", got)
	}
}

func TestExtractLowestAwardPrice_FindsLowest(t *testing.T) {
	// SvelteKit-shaped fixture: a nonstop AS row and a 1-stop JX row,
	// each with multiple cabins. extractLowestAwardPrice should pick
	// the lowest atmosPoints across all rows/cabins.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":     map[string]any{"atmosPoints": float64(145000), "grandTotal": float64(55.0)},
				"REFUNDABLE_BUSINESS": map[string]any{"atmosPoints": float64(450000), "grandTotal": float64(55.0)},
			}),
			makeRow(segments("JX", "JX"), map[string]any{
				"REFUNDABLE_MAIN": map[string]any{"atmosPoints": float64(120000), "grandTotal": float64(55.0)},
			}),
		},
	}
	got := extractLowestAwardPrice(json.RawMessage(buildSvelteKitFixture(root)), "", -1)
	if got.Miles == nil {
		t.Fatalf("expected miles extracted, got %+v", got)
	}
	if *got.Miles != 120000 {
		t.Errorf("expected lowest = 120000, got %d", *got.Miles)
	}
}

func TestExtractLowestAwardPrice_MaxStopsFilter(t *testing.T) {
	// 3-stop fare with the lowest miles must be excluded when
	// max-stops=1.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS", "BA", "JL", "AS"), map[string]any{
				"REFUNDABLE_MAIN": map[string]any{"atmosPoints": float64(80000), "grandTotal": float64(5.6)},
			}),
			makeRow(segments("AS", "JX"), map[string]any{
				"REFUNDABLE_MAIN": map[string]any{"atmosPoints": float64(120000), "grandTotal": float64(5.6)},
			}),
		},
	}
	got := extractLowestAwardPrice(json.RawMessage(buildSvelteKitFixture(root)), "", 1)
	if got.Miles == nil {
		t.Fatalf("expected at least one match within max-stops=1")
	}
	if *got.Miles != 120000 {
		t.Errorf("max-stops=1 should exclude the 80k/3-stop offer; got %d", *got.Miles)
	}
}

func TestExtractLowestAwardPrice_CabinFilter(t *testing.T) {
	// A user running award-cheapest with --cabin economy must not get
	// the cheaper BUSINESS row back. The API's SpecFare hint is
	// best-effort; the extractor enforces the cabin at hydration time.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":     map[string]any{"atmosPoints": float64(70000), "grandTotal": float64(5.6)},
				"REFUNDABLE_BUSINESS": map[string]any{"atmosPoints": float64(60000), "grandTotal": float64(5.6)},
			}),
		},
	}
	got := extractLowestAwardPrice(json.RawMessage(buildSvelteKitFixture(root)), "economy", -1)
	if got.Miles == nil {
		t.Fatalf("expected miles, got nil")
	}
	if *got.Miles != 70000 {
		t.Errorf("--cabin economy should pick MAIN 70000, not the cheaper BUSINESS 60000; got %d", *got.Miles)
	}
	if !strings.Contains(strings.ToLower(got.Cabin), "main") {
		t.Errorf("cabin = %q; want a MAIN slot", got.Cabin)
	}
}

func TestBuildAwardSearchParams_AlwaysSetsOnlineAward(t *testing.T) {
	p := buildAwardSearchParams(awardSearchInput{Origin: "SFO", Destination: "HND", Depart: "2026-08-15"})
	if p["ShoppingMethod"] != "onlineaward" {
		t.Errorf("expected ShoppingMethod=onlineaward, got %q", p["ShoppingMethod"])
	}
	if p["UPG"] != "none" {
		t.Errorf("expected UPG=none, got %q", p["UPG"])
	}
	if p["OT"] != "Anytime" || p["DT"] != "Anytime" {
		t.Errorf("expected OT/DT=Anytime; got OT=%q DT=%q", p["OT"], p["DT"])
	}
	if p["O"] != "SFO" || p["D"] != "HND" || p["OD"] != "2026-08-15" {
		t.Errorf("origin/destination/depart not wired: %+v", p)
	}
}

func TestBuildAwardSearchParams_OmitsEmptyOptionals(t *testing.T) {
	p := buildAwardSearchParams(awardSearchInput{Origin: "SFO", Destination: "HND", Depart: "2026-08-15"})
	if _, ok := p["DD"]; ok {
		t.Errorf("should not set DD when return is empty, got %q", p["DD"])
	}
	if _, ok := p["SpecFare"]; ok {
		t.Errorf("should not set SpecFare when cabin is empty")
	}
}
