// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: supplier price summary.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

type supplierLine struct {
	Supplier    string  `json:"supplier"`
	Car         string  `json:"car"`
	CarClass    string  `json:"car_class"`
	PerDay      float64 `json:"per_day"`
	Total       float64 `json:"total"`
	Currency    string  `json:"currency"`
	Excess      float64 `json:"excess"`
	ExcessKnown bool    `json:"excess_known"`
	Offers      int     `json:"offers"`
	Rating      float64 `json:"rating,omitempty"`
	Reviews     int     `json:"reviews,omitempty"`
}

func newNovelSuppliersCmd(flags *rootFlags) *cobra.Command {
	var driverAge int
	var source string
	cmd := &cobra.Command{
		Use:         "suppliers [location-code] <pickup> <dropoff>",
		Short:       "One line per supplier — each with its cheapest full-insurance offer, ranked (Málaga by default)",
		Long:        "Collapse a DoYouSpain result set to the cheapest full-insurance offer per supplier — Delpaso, Record Go, Wiber, Sixt and the rest — ranked cheapest first. Location defaults to Málaga Airport (MAL02); pass a code as the first argument to look elsewhere.",
		Example:     "  rentalcarspain-pp-cli suppliers 20/08/2026 27/08/2026 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location, pickup, dropoff, ok := resolveSearchArgs(args)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("suppliers needs <pickup> <dropoff> (Málaga default) or <location-code> <pickup> <dropoff>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			offers, srcErrs := fetchOffers(ctx, flags, source, location, pickup, dropoff, "", "", driverAge)
			if len(offers) == 0 {
				if err := sourceErrorsError(srcErrs); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("no offers returned"))
			}
			for s, e := range srcErrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: source %s unavailable: %s\n", s, truncate(e.Error(), 80))
			}
			recordSnapshotFor(ctx, flags, searchKey(location, "", pickup, dropoff, driverAge), offers)

			// Key by canonical supplier so the same company doesn't double-list
			// across sources (e.g. "Niza Cars" vs "Nizacars").
			reviewIdx := reviewIndexCached(ctx, flags, location, pickup, dropoff, "", "", driverAge, offers)
			best := map[string]carsource.Offer{}
			counts := map[string]int{}
			for _, o := range offers {
				key := carsource.CanonicalSupplier(o.Supplier)
				counts[key]++
				cur, ok := best[key]
				if !ok || (o.Total > 0 && o.Total < cur.Total) {
					best[key] = o
				}
			}
			lines := make([]supplierLine, 0, len(best))
			for sup, o := range best {
				ri := reviewIdx[sup]
				lines = append(lines, supplierLine{
					Supplier: sup, Car: o.Car, CarClass: o.CarClass,
					PerDay: o.PerDay, Total: o.Total, Currency: o.Currency,
					Excess: o.Excess, ExcessKnown: o.ExcessKnown, Offers: counts[sup],
					Rating: ri.Score, Reviews: ri.Count,
				})
			}
			sort.SliceStable(lines, func(i, j int) bool { return lines[i].Total < lines[j].Total })

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"count": len(lines), "suppliers": lines})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			if len(lines) == 0 {
				fmt.Fprintln(w, "No suppliers found.")
				return nil
			}
			mf, mfErr := newMoneyFmt(ctx, flags)
			if mfErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", mfErr)
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "SUPPLIER\tCHEAPEST CAR\tTOTAL\tEXCESS\tRATING\tOFFERS")
			for _, l := range lines {
				exc := "?"
				if l.ExcessKnown {
					if l.Excess == 0 {
						exc = "none"
					} else {
						exc = fmt.Sprintf("%.0f %s", l.Excess, l.Currency)
					}
				}
				rating := "?"
				if l.Rating > 0 {
					rating = fmt.Sprintf("%.1f", l.Rating)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n", l.Supplier, truncate(l.Car, 24), mf.format(l.Total), exc, rating, l.Offers)
			}
			tw.Flush()
			fmt.Fprintln(w, "\nTOTAL is DoYouSpain's base rate; EXCESS is the deductible/deposit ('none' = fully insured).")
			if mf.active() {
				fmt.Fprintf(w, "%s amounts are indicative (ECB rate %s); you are billed in EUR.\n", mf.currency, mf.rates.Date)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&driverAge, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	cmd.Flags().StringVar(&source, "source", "doyouspain", "Aggregator source: doyouspain|rentalcars|all")
	return cmd
}
