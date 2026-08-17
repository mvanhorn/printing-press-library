// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: markets neighborhoods — per-neighborhood stats + tech vibe.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelMarketsNeighborhoodsCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagActivity string
	var flagVibe string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "neighborhoods",
		Short:       "Per-neighborhood listing stats plus optional --vibe tech keyword signals from descriptions (WiFi, projector, transit)",
		Example:     "  peerspace-pp-cli markets neighborhoods --city Paris --activity meetup --vibe tech --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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
			stats := venuex.Neighborhoods(listings, flagCity, flagActivity, flagVibe)
			if stats == nil {
				stats = make([]venuex.NeighborhoodStat, 0)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "neighborhoods city=%s activity=%s vibe=%s n=%d\n", flagCity, flagActivity, flagVibe, len(stats))
				for _, st := range stats {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  count=%d  median_price=%.0f  tech=%.2f\n",
						st.Neighborhood, st.Count, st.MedianPrice, st.TechScoreAvg)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City filter")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter")
	cmd.Flags().StringVar(&flagVibe, "vibe", "", "Optional vibe (e.g. tech) to include keyword scores")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
