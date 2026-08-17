// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: run the direct clients across a matrix of awkward query shapes
// (1-day, 30-day, far-future, out-of-hours) so edge-case parser breakage that a
// plain week-long search never exercises is caught before it reaches a user.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

// queryShape is one row of the matrix: a rental length, how far out it starts,
// and the pickup/dropoff times to probe with.
type queryShape struct {
	Name        string
	Days        int
	OffsetDays  int
	PickupTime  string
	DropoffTime string
	// Skip marks a shape the tool does not yet support (e.g. one-way): it is
	// recorded as a coverage gap rather than silently omitted.
	Skip       bool
	SkipReason string
}

// buildQueryShapes returns the matrix relative to a baseline offset (days from
// today). far-future sits well beyond the baseline; out-of-hours brackets the
// day with an early pickup and a late return that some offices reject.
func buildQueryShapes(baseOffset int) []queryShape {
	return []queryShape{
		{Name: "1-day", Days: 1, OffsetDays: baseOffset, PickupTime: "10:00", DropoffTime: "10:00"},
		{Name: "7-day", Days: 7, OffsetDays: baseOffset, PickupTime: "10:00", DropoffTime: "10:00"},
		{Name: "30-day", Days: 30, OffsetDays: baseOffset, PickupTime: "10:00", DropoffTime: "10:00"},
		{Name: "far-future", Days: 7, OffsetDays: baseOffset + 255, PickupTime: "10:00", DropoffTime: "10:00"},
		{Name: "out-of-hours", Days: 7, OffsetDays: baseOffset, PickupTime: "06:30", DropoffTime: "23:45"},
		{Name: "one-way", Skip: true,
			SkipReason: "one-way (differing dropoff airport) is not supported by the direct clients — round-trip only"},
	}
}

// shapeResult is one query shape's outcome, plus its cheapest total for the
// cross-shape monotonicity invariant.
type shapeResult struct {
	Shape       string  `json:"shape"`
	Status      string  `json:"status"` // pass | fail | skip
	Detail      string  `json:"detail"`
	Cheapest    float64 `json:"cheapest,omitempty"`
	HasCheapest bool    `json:"-"`
}

func newQueryMatrixCmd(flags *rootFlags) *cobra.Command {
	var airport string
	var age, startOffset int
	var includeAgg bool
	cmd := &cobra.Command{
		Use:   "querymatrix [airport]",
		Short: "Probe the direct clients across awkward query shapes to catch edge-case parser breakage",
		Long: "Run every direct client at one airport across a matrix of query shapes a plain week-long\n" +
			"search never exercises, and assert each shape degrades gracefully:\n" +
			"  • 1-day / 7-day / 30-day — short and long rentals price sanely\n" +
			"  • far-future — dates ~300 days out still quote (rates published that far ahead)\n" +
			"  • out-of-hours — an early pickup / late return still yields offers from 24h suppliers,\n" +
			"    with closed offices reporting a clean office-hours error rather than an empty result\n" +
			"  • one-way — recorded as a coverage gap (the direct clients are round-trip only)\n\n" +
			"Cross-shape invariant: the cheapest total is monotonic in duration (1-day < 7-day < 30-day),\n" +
			"which catches a per-day/total confusion. Airport defaults to Málaga; exits non-zero on any\n" +
			"failure so it can gate CI.",
		Example:     "  rentalcarspain-pp-cli querymatrix\n  rentalcarspain-pp-cli querymatrix MAD --include-aggregators --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 1 {
				airport = args[0]
				if _, ok := carsource.ResolveAirport(airport); !ok {
					return fmt.Errorf("unknown airport %q: pass a Spanish airport IATA code such as AGP, BCN, PMI or MAD (see the `airports` command)", airport)
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			shapes := buildQueryShapes(startOffset)
			results := make([]shapeResult, 0, len(shapes))
			cheapestByShape := map[string]float64{}
			for _, sh := range shapes {
				if sh.Skip {
					results = append(results, shapeResult{Shape: sh.Name, Status: selftestSkip, Detail: sh.SkipReason})
					continue
				}
				r := runQueryShape(ctx, flags, airport, sh, age, includeAgg)
				if r.HasCheapest {
					cheapestByShape[sh.Name] = r.Cheapest
				}
				results = append(results, r)
			}
			results = append(results, checkDurationMonotonic(cheapestByShape))

			failed := 0
			for _, r := range results {
				if r.Status == selftestFail {
					failed++
				}
			}

			loc := resolveLocationInput(airport)
			label := loc.IATA
			if label == "" {
				label = loc.Name
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{
					"airport": label, "driver_age": age, "include_aggregators": includeAgg,
					"shapes": results, "failed": failed, "ok": failed == 0,
				})
				if err := printOutputWithFlags(cmd.OutOrStdout(), b, flags); err != nil {
					return err
				}
				if failed > 0 {
					return selftestErr(fmt.Errorf("%d query shape(s) failed", failed))
				}
				return nil
			}

			w := cmd.OutOrStdout()
			src := "direct clients"
			if includeAgg {
				src = "direct clients + aggregators"
			}
			fmt.Fprintf(w, "querymatrix %s (%s), age %d\n\n", label, src, age)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "RESULT\tSHAPE\tDETAIL")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", selftestBadge(r.Status), r.Shape, r.Detail)
			}
			tw.Flush()
			if failed > 0 {
				fmt.Fprintf(w, "\nFAIL: %d query shape(s) broke — a parser likely mishandles that shape.\n", failed)
				return selftestErr(fmt.Errorf("%d query shape(s) failed", failed))
			}
			fmt.Fprintln(w, "\nOK: every query shape degrades gracefully.")
			return nil
		},
	}
	cmd.Flags().StringVar(&airport, "airport", "AGP", "Airport to probe (IATA, name, or DoYouSpain code)")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age for the probe searches")
	cmd.Flags().IntVar(&startOffset, "start-offset", 45, "Days from today for the baseline pickup date")
	cmd.Flags().BoolVar(&includeAgg, "include-aggregators", false, "Also probe DoYouSpain + Rentalcars offers (broader parser coverage, slower)")
	return cmd
}

// runQueryShape probes one shape at one airport and evaluates it: it passes when
// at least one supplier returns offers and every returned total sits in a
// duration-scaled plausibility band; it fails on a total outage or an
// out-of-band price.
func runQueryShape(ctx context.Context, flags *rootFlags, airport string, sh queryShape, age int, includeAgg bool) shapeResult {
	pickup := time.Now().AddDate(0, 0, sh.OffsetDays)
	dropoff := pickup.AddDate(0, 0, sh.Days)
	pickupStr := pickup.Format("02/01/2006")
	dropoffStr := dropoff.Format("02/01/2006")

	probe := selftestProbe(ctx, flags, airport, pickupStr, dropoffStr, sh.PickupTime, sh.DropoffTime, age)
	if includeAgg {
		offers, _ := fetchOffers(ctx, flags, "all", airport, pickupStr, dropoffStr, sh.PickupTime, sh.DropoffTime, age)
		if len(offers) > 0 {
			probe.Offers["aggregators"] = offers
		}
	}

	floor, ceil := plausibleBand(sh.Days)

	var all []carsource.Offer
	suppliers := 0
	for _, offers := range probe.Offers {
		if len(offers) > 0 {
			suppliers++
		}
		all = append(all, offers...)
	}

	if len(all) == 0 {
		return shapeResult{Shape: sh.Name, Status: selftestFail,
			Detail: fmt.Sprintf("%s→%s: no supplier returned offers (%d errored)", pickupStr, dropoffStr, len(probe.Errs))}
	}

	var bad []string
	for _, o := range all {
		if o.Total < floor || o.Total > ceil {
			bad = append(bad, fmt.Sprintf("%s €%.2f", truncate(o.Car, 16), o.Total))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return shapeResult{Shape: sh.Name, Status: selftestFail,
			Detail: fmt.Sprintf("%s→%s: %d out-of-band (€%.0f–€%.0f): %s",
				pickupStr, dropoffStr, len(bad), floor, ceil, strings.Join(bad, ", "))}
	}

	best, _ := cheapest(all)
	return shapeResult{Shape: sh.Name, Status: selftestPass, Cheapest: best.Total, HasCheapest: true,
		Detail: fmt.Sprintf("%s→%s: %d suppliers, %d offers, cheapest €%.2f (%s)",
			pickupStr, dropoffStr, suppliers, len(all), best.Total, best.Supplier)}
}

// checkDurationMonotonic asserts the cheapest total grows with rental length —
// 1-day < 7-day < 30-day — which a per-day/total confusion would violate. It
// skips when any of the three durations produced no offers.
func checkDurationMonotonic(cheapestByShape map[string]float64) shapeResult {
	name := "duration-monotonic"
	d1, ok1 := cheapestByShape["1-day"]
	d7, ok7 := cheapestByShape["7-day"]
	d30, ok30 := cheapestByShape["30-day"]
	if !ok1 || !ok7 || !ok30 {
		return shapeResult{Shape: name, Status: selftestSkip, Detail: "needs 1-day, 7-day and 30-day totals"}
	}
	detail := fmt.Sprintf("1-day €%.2f < 7-day €%.2f < 30-day €%.2f", d1, d7, d30)
	if !(d1 < d7 && d7 < d30) {
		return shapeResult{Shape: name, Status: selftestFail, Detail: "totals not monotonic in duration — " + detail}
	}
	return shapeResult{Shape: name, Status: selftestPass, Detail: detail}
}
