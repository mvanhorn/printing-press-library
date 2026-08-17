// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: venues recommend — rank listings for tech events.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelVenuesRecommendCmd(flags *rootFlags) *cobra.Command {
	var flagGuests int
	var flagBudgetMax float64
	var flagVibe string
	var flagCity string
	var flagActivity string
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "recommend",
		Short:       "Rank synced venues for tech meetups/workshops by headcount, date window, budget, and vibe keywords.",
		Example:     "  peerspace-pp-cli venues recommend --guests 40 --budget-max 180 --vibe projector,wifi --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagLimit <= 0 {
				flagLimit = 25
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			defer s.Close()

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			filtered := venuex.FilterListings(listings, flagCity, flagActivity)
			vibe := venuex.ParseVibeCSV(flagVibe)
			ranked := venuex.RankListings(filtered, flagGuests, flagBudgetMax, vibe, flagLimit)
			rows := make([]map[string]any, 0, len(ranked))
			for _, r := range ranked {
				row := listingToRow(r.Listing)
				row["score"] = r.Score
				row["gaps"] = r.Gaps
				rows = append(rows, row)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "venues recommend guests=%d budget_max=%.0f n=%d\n", flagGuests, flagBudgetMax, len(rows))
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "  score=%v  price=%v  guests=%v  %v  %v  gaps=%v\n",
						r["score"], r["price_hourly"], r["guests"], r["id"], r["title"], r["gaps"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagGuests, "guests", 0, "Target headcount")
	cmd.Flags().Float64Var(&flagBudgetMax, "budget-max", 0, "Max hourly rate")
	cmd.Flags().StringVar(&flagVibe, "vibe", "", "Comma-separated vibe keywords (e.g. projector,wifi)")
	cmd.Flags().StringVar(&flagCity, "city", "", "City filter")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Max ranked results")
	return cmd
}
