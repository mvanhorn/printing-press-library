// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cheapest-pickup-date sweep across a window.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

type dateResult struct {
	Pickup   string  `json:"pickup"`
	Dropoff  string  `json:"dropoff"`
	Total    float64 `json:"cheapest_total"`
	Supplier string  `json:"cheapest_supplier"`
	Car      string  `json:"cheapest_car"`
	Currency string  `json:"currency"`
	Offers   int     `json:"offer_count"`
	Error    string  `json:"error,omitempty"`
}

func newNovelDatesCmd(flags *rootFlags) *cobra.Command {
	var from, to, nightsStr, sortBy string
	var driverAge int
	cmd := &cobra.Command{
		Use:         "dates [location-code]",
		Short:       "Rank pickup dates in a window by cheapest full-insurance total (Málaga by default)",
		Long:        "Fan out one DoYouSpain search per candidate pickup date across a window (--from..--to), each for --nights nights, then rank the dates by cheapest full-insurance total. Use it when your dates are flexible. Location defaults to Málaga Airport (MAL02); pass a code to look elsewhere.",
		Example:     "  rentalcarspain-pp-cli dates --from 10/08/2026 --to 24/08/2026 --nights 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := defaultLocationCode
			if len(args) >= 1 {
				location = args[0]
			}
			if from == "" || to == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are required (dd/mm/yyyy)"))
			}
			nights, err := parseNightsFlag(nightsStr)
			if err != nil {
				return usageErr(err)
			}
			fromT, err := parseDMY(from)
			if err != nil {
				return usageErr(fmt.Errorf("--from: %w", err))
			}
			toT, err := parseDMY(to)
			if err != nil {
				return usageErr(fmt.Errorf("--to: %w", err))
			}
			if toT.Before(fromT) {
				return usageErr(fmt.Errorf("--to must not be before --from"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Cap fan-out to keep runtime and load bounded.
			const maxDays = 21
			dys := carsource.NewDoYouSpain(carHTTPClient(flags))
			results := make([]dateResult, 0)
			day := 0
			for d := fromT; !d.After(toT) && day < maxDays; d = d.AddDate(0, 0, 1) {
				day++
				pickup := d.Format("02/01/2006")
				dropoff := d.AddDate(0, 0, nights).Format("02/01/2006")
				offers, serr := dys.Search(ctx, carsource.SearchQuery{ // pp:client-call
					LocationCode: location, Pickup: pickup, Dropoff: dropoff, DriverAge: driverAge,
				})
				r := dateResult{Pickup: pickup, Dropoff: dropoff, Currency: "EUR"}
				if serr != nil {
					r.Error = serr.Error()
					results = append(results, r)
					continue
				}
				recordSnapshotFor(ctx, flags, searchKey(location, "", pickup, dropoff, driverAge), offers)
				if best, ok := cheapest(offers); ok {
					r.Total = best.Total
					r.Supplier = best.Supplier
					r.Car = best.Car
					r.Offers = len(offers)
				}
				results = append(results, r)
			}
			// Rank results. "cheapest" (default) puts the lowest total first
			// with errored/empty dates last; "date" keeps chronological order.
			if sortBy != "date" {
				sort.SliceStable(results, func(i, j int) bool {
					a, b := results[i], results[j]
					ai := a.Error == "" && a.Total > 0
					bi := b.Error == "" && b.Total > 0
					if ai != bi {
						return ai
					}
					return a.Total < b.Total
				})
			}

			if wantsMachineOutput(flags) || flags.asJSON {
				bb, _ := json.Marshal(map[string]any{"location": location, "nights": nights, "count": len(results), "dates": results})
				return printOutputWithFlags(cmd.OutOrStdout(), bb, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(w, "No dates evaluated.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "PICKUP\tDROPOFF\tCHEAPEST\tSUPPLIER")
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(tw, "%s\t%s\t(error)\t%s\n", r.Pickup, r.Dropoff, truncate(r.Error, 24))
					continue
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2f %s\t%s\n", r.Pickup, r.Dropoff, r.Total, r.Currency, r.Supplier)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Window start pickup date (dd/mm/yyyy)")
	cmd.Flags().StringVar(&to, "to", "", "Window end pickup date (dd/mm/yyyy)")
	cmd.Flags().StringVar(&nightsStr, "nights", "7", "Rental length in nights")
	cmd.Flags().StringVar(&sortBy, "sort", "cheapest", "Sort order: cheapest|date")
	cmd.Flags().IntVar(&driverAge, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	return cmd
}

// parseDMY parses dd/mm/yyyy or yyyy-mm-dd into a time.Time.
func parseDMY(s string) (time.Time, error) {
	for _, layout := range []string{"02/01/2006", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %q is not dd/mm/yyyy", s)
}
