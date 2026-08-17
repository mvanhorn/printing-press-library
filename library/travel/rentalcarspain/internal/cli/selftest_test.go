// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
)

// expectedDirectSuppliers must include Delpaso only at Málaga and the
// Canaries-only pair only at island airports, so a company that legitimately
// does not serve an airport is never flagged unreachable.
func TestExpectedDirectSuppliers(t *testing.T) {
	has := func(set []string, name string) bool {
		for _, s := range set {
			if s == name {
				return true
			}
		}
		return false
	}
	agp := expectedDirectSuppliers("AGP")
	if !has(agp, "Delpaso") {
		t.Errorf("AGP should expect Delpaso, got %v", agp)
	}
	if has(agp, "CICAR") || has(agp, "Autoreisen") {
		t.Errorf("AGP should not expect the Canaries-only clients, got %v", agp)
	}
	bcn := expectedDirectSuppliers("bcn") // case-insensitive
	if has(bcn, "Delpaso") {
		t.Errorf("BCN (not Málaga) should not expect Delpaso, got %v", bcn)
	}
	tfs := expectedDirectSuppliers("TFS")
	if !has(tfs, "CICAR") || !has(tfs, "Autoreisen") {
		t.Errorf("TFS should expect the Canaries-only clients, got %v", tfs)
	}
	if has(tfs, "Delpaso") {
		t.Errorf("TFS should not expect Delpaso, got %v", tfs)
	}
}

func offer(sup, car string, total float64) carsource.Offer {
	return carsource.Offer{Supplier: sup, Car: car, Total: total, Currency: "EUR", URL: "https://booking.example"}
}

// A healthy probe: every mainland client present, real names, sane prices.
func healthyProbe(iata string, cheapest float64) airportProbe {
	return airportProbe{
		IATA: iata,
		Offers: map[string][]carsource.Offer{
			"Centauro":  {offer("Centauro", "Fiat 500", cheapest+40)},
			"Drivalia":  {offer("Drivalia", "FIAT PANDA", cheapest+20)},
			"Clickrent": {offer("Clickrent", "Toyota Aygo", cheapest)},
			"Goldcar":   {offer("Goldcar", "Citroen C3", cheapest+60)},
		},
		Errs: map[string]error{},
	}
}

func statusOf(results []selftestResult, name string) string {
	for _, r := range results {
		if r.Name == name {
			return r.Status
		}
	}
	return "missing"
}

// A fully healthy two-airport run must pass every check.
func TestRunSelftestChecks_AllPass(t *testing.T) {
	probes := []airportProbe{healthyProbe("BCN", 200), healthyProbe("MAD", 250)}
	results := runSelftestChecks(probes, 35, 2800)
	for _, r := range results {
		if r.Status == selftestFail {
			t.Errorf("expected no failures, but %s failed: %s", r.Name, r.Detail)
		}
	}
	if got := statusOf(results, "prices-vary-by-airport"); got != selftestPass {
		t.Errorf("prices-vary-by-airport = %s, want pass", got)
	}
}

// Identical cheapest totals across airports is the same-price-everywhere
// regression and must fail.
func TestPricesVaryByAirport_Regression(t *testing.T) {
	probes := []airportProbe{healthyProbe("BCN", 112), healthyProbe("MAD", 112)}
	r := checkPricesVaryByAirport(probes)
	if r.Status != selftestFail {
		t.Errorf("identical cheapest totals should fail, got %s: %s", r.Status, r.Detail)
	}
}

// A missing expected company (network error) fails the reachability check.
func TestCheckReachable_Missing(t *testing.T) {
	p := healthyProbe("AGP", 200) // AGP also expects Delpaso, which is absent
	p.Errs["Delpaso"] = fmt.Errorf("boom")
	r := checkReachable(p)
	if r.Status != selftestFail {
		t.Fatalf("missing Delpaso at AGP should fail, got %s: %s", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "Delpaso") {
		t.Errorf("detail should name Delpaso, got %q", r.Detail)
	}
}

// A near-zero Centauro total is a leaked shadow group and must fail.
func TestCheckNoShadowFictions(t *testing.T) {
	p := healthyProbe("BCN", 200)
	p.Offers["Centauro"] = append(p.Offers["Centauro"], offer("Centauro", "Ghost Group", 0))
	r := checkNoShadowFictions(p, 35)
	if r.Status != selftestFail {
		t.Errorf("near-zero Centauro group should fail, got %s: %s", r.Status, r.Detail)
	}
}

// The literal "acriss" placeholder is the missing-meta.language regression.
func TestCheckDrivaliaRealNames(t *testing.T) {
	p := healthyProbe("BCN", 200)
	p.Offers["Drivalia"] = []carsource.Offer{offer("Drivalia", "acriss", 210)}
	if r := checkDrivaliaRealNames(p); r.Status != selftestFail {
		t.Errorf("placeholder \"acriss\" name should fail, got %s: %s", r.Status, r.Detail)
	}
	// Real names pass.
	p.Offers["Drivalia"] = []carsource.Offer{offer("Drivalia", "FIAT 500", 210)}
	if r := checkDrivaliaRealNames(p); r.Status != selftestPass {
		t.Errorf("real names should pass, got %s: %s", r.Status, r.Detail)
	}
}

// An out-of-band total (e.g. cents parsed as euros) must fail the price-band check.
func TestCheckPlausiblePriceBand(t *testing.T) {
	p := healthyProbe("BCN", 200)
	p.Offers["Goldcar"] = []carsource.Offer{offer("Goldcar", "Cents Bug", 2.29)}
	if r := checkPlausiblePriceBand(p, 35, 2800); r.Status != selftestFail {
		t.Errorf("out-of-band total should fail, got %s: %s", r.Status, r.Detail)
	}
}

// When an input source is absent, its dependent checks skip (never fail) — the
// reachability check owns that failure.
func TestChecksSkipWhenSourceAbsent(t *testing.T) {
	p := airportProbe{IATA: "BCN", Offers: map[string][]carsource.Offer{}, Errs: map[string]error{}}
	if r := checkNoShadowFictions(p, 35); r.Status != selftestSkip {
		t.Errorf("no Centauro offers should skip, got %s", r.Status)
	}
	if r := checkDrivaliaRealNames(p); r.Status != selftestSkip {
		t.Errorf("no Drivalia offers should skip, got %s", r.Status)
	}
	if r := checkPlausiblePriceBand(p, 35, 2800); r.Status != selftestSkip {
		t.Errorf("no offers should skip, got %s", r.Status)
	}
}
