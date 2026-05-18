// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"os"
	"path/filepath"
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
