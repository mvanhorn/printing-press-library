// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Verifies that the multi-passenger price normalization divides the group
// total Google Flights returns back down to a per-seat fare. Empirical repro
// (SEA->DCA 2026-10-09, DL786 first): 1 pax = $869, 3 pax = $2606 = 3 * $869.
func TestApplyPerPassengerPriceDividesGroupTotal(t *testing.T) {
	flights := []Flight{{Price: 2606}, {Price: 1500}}
	applyPerPassengerPrice(flights, 3)

	if got, want := flights[0].Price, 2606.0/3.0; got != want {
		t.Fatalf("flights[0].Price = %.4f, want %.4f", got, want)
	}
	if got, want := flights[1].Price, 500.0; got != want {
		t.Fatalf("flights[1].Price = %.4f, want %.4f", got, want)
	}
}

func TestApplyPerPassengerPriceNoopForSinglePassenger(t *testing.T) {
	flights := []Flight{{Price: 869}}
	applyPerPassengerPrice(flights, 1)

	if flights[0].Price != 869 {
		t.Fatalf("flights[0].Price = %.2f, want 869 (unchanged)", flights[0].Price)
	}
}

func TestApplyPerPassengerPriceNoopForZeroOrNegative(t *testing.T) {
	for _, n := range []int{0, -1} {
		flights := []Flight{{Price: 869}}
		applyPerPassengerPrice(flights, n)
		if flights[0].Price != 869 {
			t.Fatalf("passengers=%d: flights[0].Price = %.2f, want 869 (unchanged)", n, flights[0].Price)
		}
	}
}

func TestApplyPerPassengerPriceEmptySliceSafe(t *testing.T) {
	applyPerPassengerPrice(nil, 3)
	applyPerPassengerPrice([]Flight{}, 3)
}

// Parser regression tests against captured Google GetShoppingResults
// responses. Fixtures live in testdata/ and are refreshed via the
// `-tags capture` test in capture_test.go.
//
// Audit summary (recorded 2026-05-12, plan 2026-05-12-001):
//
//   sea_kti_2026-12-24_response.json (60 KB):
//     Google returns exactly 2 itineraries (DE+PG via FRA/BKK).
//     A recursive deep walk over the entire decoded response finds the
//     same 2 rows the parser surfaces — there are no hidden buckets.
//     The same holds with --passengers 1 and on off-peak dates: the
//     SEA-KTI route is genuinely thin in Google's shopping API,
//     independent of passenger count or dates, regardless of what the
//     web landing page advertises. KTI opened September 2025 and Asian
//     carriers have not fully published codeshare data under the new
//     IATA code yet.
//
//   sea_bkk_2026-12-24_response.json (284 KB):
//     Google returns 100 itineraries across 10+ carriers. The parser
//     surfaces all 100, confirming the inner[2..3][0] walk is correct
//     for dense responses.
//
// The original bug premise ("only 1 result returned but web shows many")
// turned out to be Google-side sparsity for the new KTI airport code,
// not a parser shortfall. These tests lock in that finding and protect
// against future parser regressions in either direction.

func TestParseOffersResponseSeaKtiSparse(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "sea_kti_2026-12-24_response.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	flights, err := parseOffersResponse(body, "USD")
	if err != nil {
		t.Fatalf("parseOffersResponse: %v", err)
	}
	if len(flights) != 2 {
		t.Errorf("SEA-KTI fixture: got %d flights, want 2 (Google returns this exact count for the route — see audit summary)", len(flights))
	}
	for i, f := range flights {
		if f.Price <= 0 {
			t.Errorf("flight[%d] price = %v, want > 0", i, f.Price)
		}
		if f.Currency != "USD" {
			t.Errorf("flight[%d] currency = %q, want USD", i, f.Currency)
		}
		if len(f.Legs) == 0 {
			t.Errorf("flight[%d] has no legs", i)
		}
	}
}

func TestParseOffersResponseSeaBkkDense(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "sea_bkk_2026-12-24_response.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	flights, err := parseOffersResponse(body, "USD")
	if err != nil {
		t.Fatalf("parseOffersResponse: %v", err)
	}
	if len(flights) < 90 {
		t.Errorf("SEA-BKK fixture: got %d flights, want >= 90 (parser regressed on dense responses)", len(flights))
	}
	multiLeg := 0
	for _, f := range flights {
		if len(f.Legs) > 1 {
			multiLeg++
		}
	}
	if multiLeg == 0 {
		t.Error("expected at least one multi-leg flight in SEA-BKK fixture")
	}
	withAirline := 0
	for _, f := range flights {
		for _, leg := range f.Legs {
			if leg.Airline.Code != "" {
				withAirline++
				break
			}
		}
	}
	if withAirline < 90 {
		t.Errorf("only %d flights have airline codes populated, want >= 90", withAirline)
	}
}

func TestParseOffersResponseEmptyBody(t *testing.T) {
	_, err := parseOffersResponse([]byte(""), "USD")
	if err == nil {
		t.Error("empty body should error, got nil")
	}
}

// Regression for #1084. Google's batchexecute payload drops trailing
// zero-valued elements (jspb encoding), so a whole-hour time such as 17:00
// arrives as a single-element array [17] rather than [17, 0]. The parser must
// read the hour from t[0] regardless of whether the minute element is present;
// otherwise ~10-14% of legs (every whole-hour departure/arrival) silently
// default to 00:00.
func TestFormatLegDateTimeWholeHourMinuteOmitted(t *testing.T) {
	date := []any{float64(2026), float64(12), float64(26)}
	cases := []struct {
		name    string
		timeArr any
		want    string
	}{
		{"whole-hour evening (17:00) encoded as [17]", []any{float64(17)}, "2026-12-26T17:00:00"},
		{"whole-hour early morning (5:00) encoded as [5]", []any{float64(5)}, "2026-12-26T05:00:00"},
		{"midnight hour with non-zero minute [0,30] kept intact", []any{float64(0), float64(30)}, "2026-12-26T00:30:00"},
		{"normal time with minutes [15,5] untouched", []any{float64(15), float64(5)}, "2026-12-26T15:05:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatLegDateTime(date, tc.timeArr); got != tc.want {
				t.Errorf("formatLegDateTime(%v) = %q, want %q", tc.timeArr, got, tc.want)
			}
		})
	}
}

// A leg whose time-of-day is genuinely absent from the source (empty/missing
// time array) must return an empty string, never a fabricated 00:00 that is
// indistinguishable from a real midnight.
func TestFormatLegDateTimeMissingTimeReturnsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		date    any
		timeArr any
	}{
		{"empty date and time", []any{}, []any{}},
		{"nil date and time", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatLegDateTime(tc.date, tc.timeArr); got != "" {
				t.Errorf("formatLegDateTime(%v,%v) = %q, want \"\" (no fabricated 00:00)", tc.date, tc.timeArr, got)
			}
		})
	}
}

// parseOfferLeg-level proof: a leg array whose departure-time element is the
// truncated whole-hour form [17] yields a real 17:00 departure, while the
// minute-bearing arrival [20,55] is unaffected.
func TestParseOfferLegWholeHourDeparture(t *testing.T) {
	leg := make([]any, 23)
	leg[3] = "ICN"
	leg[6] = "BKK"
	leg[8] = []any{float64(17)}               // departure time-of-day, minute omitted (17:00)
	leg[10] = []any{float64(20), float64(55)} // arrival time-of-day (20:55)
	leg[11] = float64(355)                    // duration minutes
	leg[20] = []any{float64(2026), float64(12), float64(26)}
	leg[21] = []any{float64(2026), float64(12), float64(26)}

	got, ok := parseOfferLeg(leg)
	if !ok {
		t.Fatal("parseOfferLeg returned ok=false")
	}
	if got.DepartureTime != "2026-12-26T17:00:00" {
		t.Errorf("DepartureTime = %q, want 2026-12-26T17:00:00 (must not default to 00:00)", got.DepartureTime)
	}
	if got.ArrivalTime != "2026-12-26T20:55:00" {
		t.Errorf("ArrivalTime = %q, want 2026-12-26T20:55:00", got.ArrivalTime)
	}
}

// End-to-end regression on the captured dense response: before the #1084 fix,
// 26 of 251 leg departures (and 21 arrivals) carried whole-hour times that the
// parser truncated to 00:00. After the fix no leg in this fixture should report
// a midnight departure or arrival, because every leg in the capture has a real
// time-of-day in the source.
func TestParseOffersResponseNoFabricatedMidnight(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "sea_bkk_2026-12-24_response.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	flights, err := parseOffersResponse(body, "USD")
	if err != nil {
		t.Fatalf("parseOffersResponse: %v", err)
	}
	// Count T00:00:00 occurrences rather than failing on each one.
	// A jspb time array of [] with a present date is a *genuine* midnight and
	// correctly produces T00:00:00 — that is not a bug. We only want to catch
	// the pre-#1084 regression where a whole-hour time like [17] was silently
	// truncated to T00:00:00.
	//
	// For this specific SEA-BKK fixture (2026-12-24) every leg has a real
	// non-midnight time-of-day in the source, so the expected count is 0.
	// A fixture that genuinely includes a midnight departure would require a
	// different assertion strategy (e.g. cross-referencing the raw jspb array).
	midnightDeps := 0
	midnightArrs := 0
	wholeHourDeps := 0
	for _, f := range flights {
		for _, leg := range f.Legs {
			if strings.HasSuffix(leg.DepartureTime, "T00:00:00") {
				midnightDeps++
			}
			if strings.HasSuffix(leg.ArrivalTime, "T00:00:00") {
				midnightArrs++
			}
			if strings.HasSuffix(leg.DepartureTime, ":00:00") && leg.DepartureTime != "" {
				wholeHourDeps++
			}
		}
	}
	if midnightDeps != 0 {
		t.Errorf("got %d midnight departures (T00:00:00), want 0 — fabricated midnights still present in fixture", midnightDeps)
	}
	if midnightArrs != 0 {
		t.Errorf("got %d midnight arrivals (T00:00:00), want 0 — fabricated midnights still present in fixture", midnightArrs)
	}
	// Sanity: the capture genuinely contains whole-hour departures (the [HH]
	// truncated form), so this regression is actually exercising the fix.
	if wholeHourDeps == 0 {
		t.Error("expected at least one whole-hour departure in fixture; regression not exercised")
	}
}
