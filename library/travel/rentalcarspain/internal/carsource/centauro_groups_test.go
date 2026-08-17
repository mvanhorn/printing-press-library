// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

// Centauro returns "no-rates" shadow groups next to the real ones. They carry
// amount=0 / noRates=true but still expose a Premium package price, which is
// NOT a bookable total — reading it produced a fictional price that was
// identical at every branch (e.g. €112 everywhere) and ~2.5x too cheap.
func TestBookableCentauroGroup(t *testing.T) {
	realGroup := centauroVehicleGroup{
		Code: "A", Name: "KIA PICANTO", Amount: 145, Available: true, NoRates: false,
		Packages: []centauroPackage{{Code: "Premium", Amount: 285}},
	}
	shadowGroup := centauroVehicleGroup{
		Code: "AS", Name: "FIAT 500", Amount: 0, Available: true, NoRates: true,
		Packages: []centauroPackage{{Code: "Premium", Amount: 112}},
	}

	if !bookableCentauroGroup(realGroup) {
		t.Error("a group with a real base rate must be bookable")
	}
	if bookableCentauroGroup(shadowGroup) {
		t.Error("a noRates/zero-amount shadow group must be rejected")
	}
	// Defence in depth: either signal alone disqualifies the group.
	if bookableCentauroGroup(centauroVehicleGroup{Code: "X", Amount: 0, Available: true}) {
		t.Error("zero base amount must be rejected even when noRates is false")
	}
	if bookableCentauroGroup(centauroVehicleGroup{Code: "X", Amount: 145, Available: true, NoRates: true}) {
		t.Error("noRates must be rejected even when an amount is present")
	}
	if bookableCentauroGroup(centauroVehicleGroup{Code: "X", Amount: 145, Available: false}) {
		t.Error("unavailable groups must be rejected")
	}
}

// Centauro carries an auto-applied promo (e.g. the "Summer" -25 offer) live in
// the availability response. The payable Premium total is the package price
// minus Discount.Amount — reading the list price alone over-quotes by the promo
// (real booking: Premium 240 - 25 = 215, per www.centauro.net HAR, Group A).
func TestCentauroNetPremium(t *testing.T) {
	withPromo := centauroVehicleGroup{
		Code: "A", Name: "FIAT 500", Amount: 100, Available: true, NoRates: false,
		Packages: []centauroPackage{{Code: "Premium", Amount: 240}},
		Discount: &centauroDiscount{Code: "W292625", Amount: 25},
	}
	if got := centauroNetPremium(withPromo); got != 215 {
		t.Errorf("net premium with promo = %v, want 215 (240 - 25)", got)
	}

	// No discount object → full list price, unchanged.
	noPromo := centauroVehicleGroup{
		Code: "A", Name: "FIAT 500", Amount: 100, Available: true,
		Packages: []centauroPackage{{Code: "Premium", Amount: 240}},
	}
	if got := centauroNetPremium(noPromo); got != 240 {
		t.Errorf("net premium without promo = %v, want 240", got)
	}

	// A degenerate discount >= the premium must never drive the price negative
	// or to zero; ignore it and quote the list price.
	badPromo := centauroVehicleGroup{
		Code: "A", Name: "FIAT 500", Amount: 100, Available: true,
		Packages: []centauroPackage{{Code: "Premium", Amount: 240}},
		Discount: &centauroDiscount{Code: "BOGUS", Amount: 300},
	}
	if got := centauroNetPremium(badPromo); got != 240 {
		t.Errorf("net premium with over-large discount = %v, want 240 (discount ignored)", got)
	}
}

// Centauro charges obligatory driver-age surcharges online, in the payable total:
// "Conductor joven" (YD, under 25) and "Conductor senior" (SD, 74+). Both codes
// are always listed; only the one made mandatory by the driver's birth date
// carries minimumQuantity>=1. Verified on Centauro Málaga (AGP, 7 days, FIAT 500):
// young +€91, senior +€49; a standard-age driver adds nothing.
func TestCentauroMandatoryAgeSurcharge(t *testing.T) {
	young := centauroVehicleGroup{Services: []centauroService{
		{Code: "YD", Name: "Conductor joven", Amount: 91, AmountPerDay: 13, MinimumQuantity: 1, Choosable: false},
		{Code: "SD", Name: "Conductor senior", Amount: 49, AmountPerDay: 7, MinimumQuantity: 0, Choosable: false},
	}}
	if got := centauroMandatoryAgeSurcharge(young); got != 91 {
		t.Errorf("young-driver surcharge = %v, want 91", got)
	}

	senior := centauroVehicleGroup{Services: []centauroService{
		{Code: "YD", Name: "Conductor joven", Amount: 91, MinimumQuantity: 0},
		{Code: "SD", Name: "Conductor senior", Amount: 49, MinimumQuantity: 1},
	}}
	if got := centauroMandatoryAgeSurcharge(senior); got != 49 {
		t.Errorf("senior-driver surcharge = %v, want 49", got)
	}

	// Standard age: both listed but neither mandatory → no surcharge.
	standard := centauroVehicleGroup{Services: []centauroService{
		{Code: "YD", Name: "Conductor joven", Amount: 91, MinimumQuantity: 0},
		{Code: "SD", Name: "Conductor senior", Amount: 49, MinimumQuantity: 0},
	}}
	if got := centauroMandatoryAgeSurcharge(standard); got != 0 {
		t.Errorf("standard-age surcharge = %v, want 0", got)
	}

	// A mandatory non-age service (e.g. an equipment extra) must be ignored — we
	// only fold in the YD/SD age supplements.
	other := centauroVehicleGroup{Services: []centauroService{
		{Code: "GPS", Name: "GPS", Amount: 40, MinimumQuantity: 1},
	}}
	if got := centauroMandatoryAgeSurcharge(other); got != 0 {
		t.Errorf("non-age mandatory service = %v, want 0 (ignored)", got)
	}
}
