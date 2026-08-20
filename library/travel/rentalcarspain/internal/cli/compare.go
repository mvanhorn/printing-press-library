// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aggregator-vs-direct cross-check.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

type compareView struct {
	Pickup     string           `json:"pickup"`
	Dropoff    string           `json:"dropoff"`
	Supplier   string           `json:"supplier"`
	Aggregator *carsource.Offer `json:"aggregator_offer"`
	Direct     *carsource.Offer `json:"direct_offer"`
	DeltaTotal float64          `json:"delta_total"` // direct - aggregator (positive = direct is pricier)
	Cheaper    string           `json:"cheaper"`     // "aggregator" | "direct" | "tie" | "unknown"
	Note       string           `json:"note,omitempty"`
}

// compareDelta returns the direct-minus-aggregator total and which side is
// cheaper. A positive delta means the direct quote costs more, so the
// aggregator is cheaper. Differences within a one-cent deadband are a "tie";
// when either side is missing the verdict is "unknown".
func compareDelta(agg, dir *carsource.Offer) (delta float64, cheaper string) {
	if agg == nil || dir == nil {
		return 0, "unknown"
	}
	delta = dir.Total - agg.Total
	switch {
	case delta > 0.01:
		return delta, "aggregator"
	case delta < -0.01:
		return delta, "direct"
	default:
		return delta, "tie"
	}
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var driverAge int
	cmd := &cobra.Command{
		Use:         "compare <pickup> <dropoff>",
		Short:       "Compare DoYouSpain's cheapest Delpaso offer against Delpaso's own site",
		Long:        "Your booking ritual in one call: fetch DoYouSpain's cheapest Delpaso offer for Malaga Airport and Delpaso's own cheapest quote, then show the delta. Use it to confirm the aggregator price is real before booking.\n\nDo NOT use it to browse multiple suppliers; use 'search' for that. Delpaso is a Malaga-only supplier, so location is fixed to MAL02.",
		Example:     "  rentalcarspain-pp-cli compare 20/08/2026 27/08/2026 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs <pickup> <dropoff>"))
			}
			if err := requireDateArgs("compare", args[0], args[1]); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			pickup, dropoff := args[0], args[1]

			// Aggregator side: DoYouSpain at Malaga Airport, filtered to Delpaso.
			dys := carsource.NewDoYouSpain(carHTTPClient(flags))
			aggOffers, aggErr := dys.Search(ctx, carsource.SearchQuery{ // pp:client-call
				LocationCode: "MAL02", Pickup: pickup, Dropoff: dropoff, DriverAge: driverAge,
			})
			var aggBest *carsource.Offer
			if aggErr == nil {
				var delpasoOnly []carsource.Offer
				for _, o := range aggOffers {
					if carsource.MatchesSupplier(o.Supplier, "delpaso") {
						delpasoOnly = append(delpasoOnly, o)
					}
				}
				if b, ok := cheapest(delpasoOnly); ok {
					aggBest = &b
				}
			}

			// Direct side: Delpaso's own site.
			del := carsource.NewDelpaso(carHTTPClient(flags))
			directOffers, dirErr := del.Quote(ctx, pickup, dropoff, "", "", driverAge) // pp:client-call
			var dirBest *carsource.Offer
			if dirErr == nil {
				if b, ok := cheapest(directOffers); ok {
					dirBest = &b
				}
			}

			view := compareView{Pickup: pickup, Dropoff: dropoff, Supplier: "Delpaso", Aggregator: aggBest, Direct: dirBest, Cheaper: "unknown"}
			switch {
			case aggErr != nil && dirErr != nil:
				view.Note = fmt.Sprintf("both sources failed: aggregator=%v; direct=%v", aggErr, dirErr)
			case aggErr != nil:
				view.Note = fmt.Sprintf("aggregator failed: %v", aggErr)
			case dirErr != nil:
				view.Note = fmt.Sprintf("direct failed: %v", dirErr)
			}
			view.DeltaTotal, view.Cheaper = compareDelta(aggBest, dirBest)

			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(view)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "SOURCE\tCAR\tPER DAY\tTOTAL")
			if aggBest != nil {
				fmt.Fprintf(tw, "DoYouSpain\t%s\t%.2f\t%.2f %s\n", truncate(aggBest.Car, 26), aggBest.PerDay, aggBest.Total, aggBest.Currency)
			} else {
				fmt.Fprintln(tw, "DoYouSpain\t(no Delpaso offer)\t-\t-")
			}
			if dirBest != nil {
				fmt.Fprintf(tw, "Delpaso direct\t%s\t%.2f\t%.2f %s\n", truncate(dirBest.Car, 26), dirBest.PerDay, dirBest.Total, dirBest.Currency)
			} else {
				fmt.Fprintln(tw, "Delpaso direct\t(no quote)\t-\t-")
			}
			tw.Flush()
			if aggBest != nil && dirBest != nil {
				fmt.Fprintf(w, "\nCheaper: %s (delta %.2f %s)\n", view.Cheaper, view.DeltaTotal, aggBest.Currency)
			}
			if view.Note != "" {
				fmt.Fprintf(w, "Note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&driverAge, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	return cmd
}
