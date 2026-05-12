// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package gflights

import "testing"

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
