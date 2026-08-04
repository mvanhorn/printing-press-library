// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

func drRate(timing string, cents int64) drivaliaRate {
	r := drivaliaRate{PaymentTiming: timing}
	r.Price.Value = cents
	return r
}

// The live API keys the payment option on "paymentTiming", and PAY_NOW is not
// always listed first (PAY_ON_ARRIVAL can precede it and is pricier). payNowCents
// must select PAY_NOW explicitly, not just take rates[0].
func TestDrivaliaPayNowCents(t *testing.T) {
	// PAY_ON_ARRIVAL listed first, PAY_NOW second — must pick PAY_NOW (7254).
	v := drivaliaVehicle{Rates: []drivaliaRate{
		drRate("PAY_ON_ARRIVAL", 7800),
		drRate("PAY_NOW", 7254),
	}}
	if got := v.payNowCents(); got != 7254 {
		t.Errorf("payNowCents = %d, want 7254 (the PAY_NOW rate, not the first)", got)
	}

	// Legacy "type" field still recognized as a fallback.
	legacy := drivaliaVehicle{Rates: []drivaliaRate{{Type: "PAY_ON_ARRIVAL"}, {Type: "PAY_NOW"}}}
	legacy.Rates[0].Price.Value = 900
	legacy.Rates[1].Price.Value = 800
	if got := legacy.payNowCents(); got != 800 {
		t.Errorf("legacy type field: payNowCents = %d, want 800", got)
	}

	// No PAY_NOW present → fall back to the first rate.
	noNow := drivaliaVehicle{Rates: []drivaliaRate{drRate("PAY_ON_ARRIVAL", 5000)}}
	if got := noNow.payNowCents(); got != 5000 {
		t.Errorf("no PAY_NOW: payNowCents = %d, want 5000 (first rate)", got)
	}

	// No rates at all → 0.
	if got := (drivaliaVehicle{}).payNowCents(); got != 0 {
		t.Errorf("empty rates: payNowCents = %d, want 0", got)
	}
}
