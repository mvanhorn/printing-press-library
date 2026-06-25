// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNearCmd(flags *rootFlags) *cobra.Command {
	var cuisine, sortKey string
	var pickup, openNow bool
	var limit int

	cmd := &cobra.Command{
		Use:   "near <address>",
		Short: "Search restaurants near a street address",
		Long: "Search Grubhub restaurants near a street address. Auto-geocodes the address and mints an anonymous token, so no setup is required.\n\n" +
			"Use this for live restaurant search by address. For a fee/ETA comparison board use 'compare'; for finding a specific dish use 'dish'; for offline full-text search over already-synced data use the framework 'search' command.",
		Example:     "  grubhub-pp-cli near \"350 5th Ave, New York, NY\" --sort fee --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search restaurants near the given address")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address is required, e.g. near \"350 5th Ave, New York, NY\""))
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
			rows := cardsToRows(cards, cuisine, openNow)
			sortRows(rows, sortKey)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"address":       args[0],
					"resolved":      fmt.Sprintf("%s, %s %s", coord.Locality, coord.Region, coord.Postal),
					"order_method":  method,
					"total_results": total,
					"count":         len(rows),
					"restaurants":   rows,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d restaurants near %s, %s (%d total available)\n\n", len(rows), coord.Locality, coord.Region, total)
			return renderRestaurantTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Filter to restaurants matching this cuisine (e.g. sushi, pizza)")
	cmd.Flags().BoolVar(&pickup, "pickup", false, "Search pickup instead of delivery")
	cmd.Flags().StringVar(&sortKey, "sort", "", "Sort by: fee, min, eta, rating, distance, deals, name")
	cmd.Flags().BoolVar(&openNow, "open-now", false, "Only show restaurants currently open")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum restaurants to return")
	return cmd
}
