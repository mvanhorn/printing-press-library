// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"os"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func loadFixture(t *testing.T) *xhtml.Node {
	t.Helper()
	f, err := os.Open("testdata/dys-offers.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	doc, err := xhtml.Parse(f)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
}

func TestParseOffers(t *testing.T) {
	offers := parseOffers(loadFixture(t))
	if len(offers) == 0 {
		t.Fatal("expected at least one offer, got 0")
	}
	first := offers[0]
	if first.SupplierCode != "PAS" {
		t.Errorf("first supplier code = %q, want PAS", first.SupplierCode)
	}
	if first.Supplier != "Delpaso" {
		t.Errorf("first supplier = %q, want Delpaso", first.Supplier)
	}
	if !strings.Contains(first.Car, "Citroen DS3") {
		t.Errorf("first car = %q, want it to contain Citroen DS3", first.Car)
	}
	if first.Total <= 0 {
		t.Errorf("first total = %v, want > 0", first.Total)
	}
	if first.PerDay <= 0 {
		t.Errorf("first per_day = %v, want > 0", first.PerDay)
	}
	if first.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", first.Currency)
	}
	if first.Seats == 0 {
		t.Errorf("expected seats parsed, got 0")
	}
}

func TestParsePrice(t *testing.T) {
	cases := map[string]float64{
		"39.98 €":     39.98,
		"£32.45":      32.45,
		"71.40 €":     71.40,
		"1.234,56 €":  1234.56,
		"5,71":        5.71,
		"":            0,
	}
	for in, want := range cases {
		if got := parsePrice(in); got != want {
			t.Errorf("parsePrice(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseLocations(t *testing.T) {
	frag := `<li data-destino='MAL02' data-pais='ES' data-iata='AGP' data-destino-description='Malaga Airport (AGP)'>x</li>` +
		`<li data-destino='MAL01' data-pais='ES' data-destino-description='Malaga City'>y</li>`
	locs := parseLocations(frag)
	if len(locs) != 2 {
		t.Fatalf("got %d locations, want 2", len(locs))
	}
	if locs[0].Code != "MAL02" || locs[0].IATA != "AGP" {
		t.Errorf("loc0 = %+v", locs[0])
	}
	if locs[0].Description != "Malaga Airport (AGP)" {
		t.Errorf("loc0 desc = %q", locs[0].Description)
	}
}

func TestMatchesSupplier(t *testing.T) {
	if !MatchesSupplier("Record Go", "recordgo") {
		t.Error("recordgo alias should match Record Go")
	}
	if !MatchesSupplier("Drivalia", "drivalia") {
		t.Error("substring should match Drivalia")
	}
	if MatchesSupplier("Sixt", "delpaso") {
		t.Error("delpaso should not match Sixt")
	}
	if !MatchesSupplier("Anything", "") {
		t.Error("empty keyword should match anything")
	}
}

func TestParseExcess(t *testing.T) {
	// A CDW tooltip with a stated excess should populate Excess + ExcessKnown
	// and mark the offer as not full insurance.
	title := `Your rental includes Collision Damage Waiver and Theft Protection with an excess of <span class='text-nowrap'>1200.00 €</span>.`
	m := excessRe.FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("excessRe did not match stated excess")
	}
	if got := parsePrice(m[1]); got != 1200.00 {
		t.Errorf("parsed excess = %v, want 1200.00", got)
	}
	// A tooltip with no digit should not match.
	if excessRe.MatchString("Your rental includes Collision Damage Waiver and Theft Protection.") {
		t.Error("excessRe matched a tooltip with no excess amount")
	}
}

func TestRentalDaysAndDateTime(t *testing.T) {
	if d := rentalDays("14/08/2026", "21/08/2026"); d != 7 {
		t.Errorf("rentalDays = %d, want 7", d)
	}
	if d := rentalDays("14/08/2026", "14/08/2026"); d != 1 {
		t.Errorf("same-day rentalDays = %d, want 1 (min)", d)
	}
	dt, err := rcDateTime("14/08/2026", "16:30")
	if err != nil || dt != "2026-08-14T16:30:00" {
		t.Errorf("rcDateTime = %q, %v; want 2026-08-14T16:30:00", dt, err)
	}
}
