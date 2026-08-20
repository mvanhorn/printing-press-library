// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: a live canary that asserts the invariants we verified by hand —
// so a supplier changing their API breaks the check, not a silent wrong quote.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

// selftest status literals. A canary is pass/fail/skip: skip means the check
// could not run (its input was unreachable — the reachability check owns that
// failure), so a skip never fails the run on its own.
const (
	selftestPass = "pass"
	selftestFail = "fail"
	selftestSkip = "skip"
)

// priceFloorPerDay / priceCeilPerDay bound a plausible all-in zero-excess
// daily rate for these mid-market Spanish suppliers. Below the floor a quote is
// implausible (a cents-parsed-as-euros bug); above the ceiling it is implausible
// even for their premium tier (an Audi Q8 tops out near €400–700/day, so a
// digit-shift bug to €4000+ is what this catches). The band is deliberately
// wide — it flags garbage, not merely-expensive cars.
const (
	priceFloorPerDay = 5.0
	priceCeilPerDay  = 1200.0
)

// plausibleBand returns the total-price band for a rental of the given length.
func plausibleBand(days int) (floor, ceil float64) {
	return float64(days) * priceFloorPerDay, float64(days) * priceCeilPerDay
}

// selftestResult is one invariant check's outcome.
type selftestResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass | fail | skip
	Detail string `json:"detail"`
}

// airportProbe holds the direct-client offers gathered for one airport, keyed
// by company name, plus the per-company errors.
type airportProbe struct {
	IATA   string
	Offers map[string][]carsource.Offer
	Errs   map[string]error
}

// canaryIATAs are the Canary-Islands airports where the islands-only direct
// clients (CICAR, Autoreisen) are expected to serve and the mainland-priced
// clients still work.
var canaryIATAs = map[string]bool{
	"TFS": true, "TFN": true, "LPA": true, "ACE": true, "FUE": true, "SPC": true,
}

// expectedDirectSuppliers lists the direct companies that should return offers
// at an airport, so a network error or empty result for one of them is a real
// regression rather than an expected out-of-region miss (Delpaso is
// Málaga-only; CICAR/Autoreisen are Canaries-only).
func expectedDirectSuppliers(iata string) []string {
	iata = strings.ToUpper(strings.TrimSpace(iata))
	// Mainland-priced clients serve every Spanish airport.
	set := []string{"Centauro", "Drivalia", "Clickrent", "Goldcar"}
	if iata == "AGP" {
		set = append(set, "Delpaso")
	}
	if canaryIATAs[iata] {
		set = append(set, "CICAR", "Autoreisen")
	}
	sort.Strings(set)
	return set
}

func newSelftestCmd(flags *rootFlags) *cobra.Command {
	var pickupTime, dropoffTime, airportsCSV string
	var age, days, startOffset int
	cmd := &cobra.Command{
		Use:   "selftest",
		Short: "Live canary: assert the zero-excess pricing invariants against real supplier APIs",
		Long: "Run a live search across two airports and assert the invariants verified by hand, so a\n" +
			"supplier changing their API breaks this check instead of silently producing a wrong quote:\n" +
			"  • direct clients reachable — every company that should serve the airport returns offers\n" +
			"  • prices vary by airport — the cheapest direct total differs between the two airports\n" +
			"    (catches a regression to the old same-price-everywhere bug)\n" +
			"  • no shadow-group fictions — Centauro offers are all real priced groups, none near zero\n" +
			"  • Drivalia real names — car names are real models, not the literal \"acriss\" placeholder\n" +
			"    (catches the missing request-language regression)\n" +
			"  • plausible price band — every direct total sits in a sane per-day range\n\n" +
			"Dates default to a mid-future week so the canary is always valid without hardcoding. Exits\n" +
			"non-zero if any invariant fails, so it can gate CI or a cron.",
		Example:     "  rentalcarspain-pp-cli selftest\n  rentalcarspain-pp-cli selftest --airports AGP,MAD --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			codes := splitCSV(airportsCSV)
			if len(codes) != 2 {
				return usageErr(fmt.Errorf("--airports needs exactly two comma-separated airports (e.g. AGP,BCN)"))
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			pickup := time.Now().AddDate(0, 0, startOffset)
			dropoff := pickup.AddDate(0, 0, days)
			pickupStr := pickup.Format("02/01/2006")
			dropoffStr := dropoff.Format("02/01/2006")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Probe both airports concurrently.
			probes := make([]airportProbe, len(codes))
			var wg sync.WaitGroup
			for i, code := range codes {
				wg.Add(1)
				go func(i int, code string) {
					defer wg.Done()
					probes[i] = selftestProbe(ctx, flags, code, pickupStr, dropoffStr, pickupTime, dropoffTime, age)
				}(i, code)
			}
			wg.Wait()

			floor, ceil := plausibleBand(days)
			results := runSelftestChecks(probes, floor, ceil)

			failed := 0
			for _, r := range results {
				if r.Status == selftestFail {
					failed++
				}
			}

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{
					"airports": codes, "pickup": pickupStr, "dropoff": dropoffStr, "days": days,
					"driver_age": age, "checks": results, "failed": failed, "ok": failed == 0,
				})
				if err := printOutputWithFlags(cmd.OutOrStdout(), b, flags); err != nil {
					return err
				}
				if failed > 0 {
					return selftestErr(fmt.Errorf("%d invariant(s) failed", failed))
				}
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "selftest %s → %s (%d days), age %d, airports %s\n\n",
				pickupStr, dropoffStr, days, age, strings.Join(codes, " + "))
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "RESULT\tCHECK\tDETAIL")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", selftestBadge(r.Status), r.Name, r.Detail)
			}
			tw.Flush()
			if failed > 0 {
				fmt.Fprintf(w, "\nFAIL: %d invariant(s) violated — a supplier parser or API likely changed.\n", failed)
				return selftestErr(fmt.Errorf("%d invariant(s) failed", failed))
			}
			fmt.Fprintln(w, "\nOK: all invariants hold.")
			return nil
		},
	}
	cmd.Flags().StringVar(&airportsCSV, "airports", "AGP,BCN", "Two comma-separated airports to cross-check (prices must differ between them)")
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "10:00", "Pickup time HH:MM")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "10:00", "Dropoff time HH:MM")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age for the probe searches")
	cmd.Flags().IntVar(&days, "days", 7, "Rental length in days")
	cmd.Flags().IntVar(&startOffset, "start-offset", 45, "Days from today for the probe pickup date")
	return cmd
}

// selftestProbe runs every direct company for one airport and returns the
// offers and errors keyed by company name.
func selftestProbe(ctx context.Context, flags *rootFlags, code, pickup, dropoff, pt, dt string, age int) airportProbe {
	loc := resolveLocationInput(code)
	now := time.Now()
	companies := directCompanies()
	offers := make(map[string][]carsource.Offer, len(companies))
	errs := map[string]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, co := range companies {
		wg.Add(1)
		go func(co directCompany) {
			defer wg.Done()
			out, err := co.quote(ctx, flags, loc, pickup, dropoff, pt, dt, age, now)
			mu.Lock()
			if err != nil {
				errs[co.name] = err
			} else {
				offers[co.name] = out
			}
			mu.Unlock()
		}(co)
	}
	wg.Wait()
	return airportProbe{IATA: strings.ToUpper(strings.TrimSpace(code)), Offers: offers, Errs: errs}
}

// runSelftestChecks evaluates every invariant against the gathered probes.
func runSelftestChecks(probes []airportProbe, floor, ceil float64) []selftestResult {
	var results []selftestResult
	for _, p := range probes {
		results = append(results, checkReachable(p))
	}
	results = append(results, checkPricesVaryByAirport(probes))
	for _, p := range probes {
		results = append(results, checkNoShadowFictions(p, floor))
	}
	for _, p := range probes {
		results = append(results, checkDrivaliaRealNames(p))
	}
	for _, p := range probes {
		results = append(results, checkPlausiblePriceBand(p, floor, ceil))
	}
	for _, p := range probes {
		results = append(results, checkBookingURLs(p))
	}
	return results
}

// checkBookingURLs asserts every direct offer carries a usable https booking
// link, so the `direct` command's "Book at:" footer never silently blanks when
// a parser stops setting Offer.URL.
func checkBookingURLs(p airportProbe) selftestResult {
	name := "booking-urls[" + p.IATA + "]"
	total := 0
	var bad []string
	for company, offers := range p.Offers {
		for _, o := range offers {
			total++
			if u, err := url.Parse(o.URL); err != nil || u.Scheme != "https" || u.Host == "" {
				bad = append(bad, fmt.Sprintf("%s %q", company, truncate(o.URL, 30)))
			}
		}
	}
	if total == 0 {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "no offers (see reachability)"}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return selftestResult{Name: name, Status: selftestFail,
			Detail: fmt.Sprintf("%d offer(s) with no valid https booking URL: %s", len(bad), strings.Join(bad, ", "))}
	}
	return selftestResult{Name: name, Status: selftestPass,
		Detail: fmt.Sprintf("all %d offers carry an https booking URL", total)}
}

// checkReachable asserts every company expected to serve the airport returned
// at least one offer.
func checkReachable(p airportProbe) selftestResult {
	want := expectedDirectSuppliers(p.IATA)
	var missing, ok []string
	for _, name := range want {
		if len(p.Offers[name]) > 0 {
			ok = append(ok, name)
			continue
		}
		if err, had := p.Errs[name]; had {
			missing = append(missing, fmt.Sprintf("%s (%s)", name, truncate(err.Error(), 40))) // pp:client-call
		} else {
			missing = append(missing, name+" (no offers)")
		}
	}
	name := "direct-clients-reachable[" + p.IATA + "]"
	if len(missing) > 0 {
		return selftestResult{Name: name, Status: selftestFail,
			Detail: fmt.Sprintf("unreachable/empty: %s", strings.Join(missing, ", "))}
	}
	return selftestResult{Name: name, Status: selftestPass,
		Detail: fmt.Sprintf("all %d reachable: %s", len(ok), strings.Join(ok, ", "))}
}

// checkPricesVaryByAirport asserts the cheapest direct total differs between
// the two airports — the guard against a regression to one price everywhere.
func checkPricesVaryByAirport(probes []airportProbe) selftestResult {
	name := "prices-vary-by-airport"
	if len(probes) != 2 {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "needs exactly two airports"}
	}
	a, aOK := probeCheapest(probes[0])
	b, bOK := probeCheapest(probes[1])
	if !aOK || !bOK {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "no offers at one airport (see reachability)"}
	}
	detail := fmt.Sprintf("%s €%.2f (%s) vs %s €%.2f (%s)",
		probes[0].IATA, a.Total, a.Supplier, probes[1].IATA, b.Total, b.Supplier)
	if a.Total == b.Total {
		return selftestResult{Name: name, Status: selftestFail,
			Detail: "identical cheapest total across airports — " + detail}
	}
	return selftestResult{Name: name, Status: selftestPass, Detail: detail}
}

// checkNoShadowFictions asserts Centauro returned real priced groups only — no
// near-zero total leaking from a shadow (noRates / amount=0) group.
func checkNoShadowFictions(p airportProbe, floor float64) selftestResult {
	name := "centauro-no-shadow-fictions[" + p.IATA + "]"
	offers := p.Offers["Centauro"]
	if len(offers) == 0 {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "no Centauro offers (see reachability)"}
	}
	var bad []string
	for _, o := range offers {
		if o.Total < floor {
			bad = append(bad, fmt.Sprintf("%s €%.2f", truncate(o.Car, 18), o.Total))
		}
	}
	if len(bad) > 0 {
		return selftestResult{Name: name, Status: selftestFail,
			Detail: fmt.Sprintf("%d near-zero group(s): %s", len(bad), strings.Join(bad, ", "))}
	}
	return selftestResult{Name: name, Status: selftestPass,
		Detail: fmt.Sprintf("%d priced groups, all ≥ €%.0f", len(offers), floor)}
}

// checkDrivaliaRealNames asserts Drivalia car names are real models, not the
// literal "acriss" placeholder returned when the request omits meta.language.
func checkDrivaliaRealNames(p airportProbe) selftestResult {
	name := "drivalia-real-names[" + p.IATA + "]"
	offers := p.Offers["Drivalia"]
	if len(offers) == 0 {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "no Drivalia offers (see reachability)"}
	}
	var bad []string
	for _, o := range offers {
		car := strings.ToLower(strings.TrimSpace(o.Car))
		if car == "" || car == "acriss" || strings.Contains(car, "acriss") {
			bad = append(bad, fmt.Sprintf("%q", o.Car))
		}
	}
	if len(bad) > 0 {
		return selftestResult{Name: name, Status: selftestFail,
			Detail: fmt.Sprintf("placeholder/empty names: %s", strings.Join(bad, ", "))}
	}
	return selftestResult{Name: name, Status: selftestPass,
		Detail: fmt.Sprintf("%d offers, real model names (e.g. %s)", len(offers), truncate(offers[0].Car, 24))}
}

// checkPlausiblePriceBand asserts every direct total sits in a sane per-day
// band — guards against a parser emitting cents-as-euros or an absurd figure.
func checkPlausiblePriceBand(p airportProbe, floor, ceil float64) selftestResult {
	name := "plausible-price-band[" + p.IATA + "]"
	var bad []string
	total := 0
	for company, offers := range p.Offers {
		for _, o := range offers {
			total++
			if o.Total < floor || o.Total > ceil {
				bad = append(bad, fmt.Sprintf("%s %s €%.2f", company, truncate(o.Car, 16), o.Total))
			}
		}
	}
	if total == 0 {
		return selftestResult{Name: name, Status: selftestSkip, Detail: "no offers (see reachability)"}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return selftestResult{Name: name, Status: selftestFail,
			Detail: fmt.Sprintf("%d out-of-band (€%.0f–€%.0f): %s", len(bad), floor, ceil, strings.Join(bad, ", "))}
	}
	return selftestResult{Name: name, Status: selftestPass,
		Detail: fmt.Sprintf("%d offers all within €%.0f–€%.0f", total, floor, ceil)}
}

// probeCheapest returns the cheapest offer across every company at an airport.
func probeCheapest(p airportProbe) (carsource.Offer, bool) {
	var all []carsource.Offer
	for _, offers := range p.Offers {
		all = append(all, offers...)
	}
	return cheapest(all)
}

func selftestBadge(status string) string {
	switch status {
	case selftestPass:
		return green("PASS")
	case selftestFail:
		return red("FAIL")
	default:
		return yellow("SKIP")
	}
}
