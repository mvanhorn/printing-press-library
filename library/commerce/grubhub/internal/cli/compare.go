// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var sortKey, cuisine string
	var maxFee, maxMin float64
	var etaUnder, limit int
	var pickup bool

	cmd := &cobra.Command{
		Use:   "compare <address>",
		Short: "Compare delivery fee, minimum, ETA, rating, and distance across every nearby restaurant",
		Long: "Build a sortable comparison board of every nearby restaurant's delivery fee, minimum, ETA, rating, and distance in one view — the figures the Grubhub app buries one tap deep per restaurant.\n\n" +
			"Use this for the full ranked table. For a single recommended pick use 'pick'; for a specific dish use 'dish'.",
		Example:     "  grubhub-pp-cli compare \"350 5th Ave, New York, NY\" --sort fee --eta-under 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a delivery comparison board for the given address")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address is required, e.g. compare \"350 5th Ave, New York, NY\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			coord, err := geocodeAddress(ctx, c, args[0])
			if err != nil {
				return err
			}
			method := "delivery"
			if pickup {
				method = "pickup"
			}
			cards, total, err := searchCards(ctx, c, coord, searchOptions{orderMethod: method})
			if err != nil {
				return err
			}
			rows := cardsToRows(cards, cuisine, false)
			rows = filterComparison(rows, maxFee, maxMin, etaUnder)
			if sortKey == "" {
				sortKey = "fee"
			}
			sortRows(rows, sortKey)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"address":       args[0],
					"order_method":  method,
					"sorted_by":     sortKey,
					"total_results": total,
					"count":         len(rows),
					"restaurants":   rows,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Comparison of %d restaurants near %s, %s (sorted by %s)\n\n", len(rows), coord.Locality, coord.Region, sortKey)
			return renderRestaurantTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&sortKey, "sort", "fee", "Sort by: fee, min, eta, rating, distance, deals, name")
	cmd.Flags().Float64Var(&maxFee, "max-fee", 0, "Only restaurants with delivery fee at or below this many dollars")
	cmd.Flags().Float64Var(&maxMin, "max-min", 0, "Only restaurants with order minimum at or below this many dollars")
	cmd.Flags().IntVar(&etaUnder, "eta-under", 0, "Only restaurants with an ETA under this many minutes")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Filter to a cuisine (e.g. pizza, sushi)")
	cmd.Flags().BoolVar(&pickup, "pickup", false, "Compare pickup instead of delivery")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum restaurants to return")
	return cmd
}

func filterComparison(rows []restaurantRow, maxFee, maxMin float64, etaUnder int) []restaurantRow {
	out := make([]restaurantRow, 0, len(rows))
	for _, r := range rows {
		if maxFee > 0 && float64(r.DeliveryFeeCents) > maxFee*100 {
			continue
		}
		if maxMin > 0 && float64(r.MinimumCents) > maxMin*100 {
			continue
		}
		if etaUnder > 0 && r.ETAMinutes >= etaUnder {
			continue
		}
		out = append(out, r)
	}
	return out
}
