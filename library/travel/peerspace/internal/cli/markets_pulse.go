// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: markets pulse — cross-city market aggregates.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelMarketsPulseCmd(flags *rootFlags) *cobra.Command {
	var flagCity []string
	var flagActivity string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "pulse",
		Short:       "Cross-city rollup of median price, listing density, instant-book share, and capacity quantiles.",
		Example:     "  peerspace-pp-cli markets pulse --city Paris --city Lyon --activity meetup --json",
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
			pulses := venuex.PulseByCity(listings, flagCity, flagActivity)
			if pulses == nil {
				pulses = make([]venuex.MarketPulse, 0)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "markets pulse activity=%s cities=%d\n", flagActivity, len(pulses))
				for _, p := range pulses {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  n=%d  median_price=%.0f  ib=%.1f%%  capacity_p50=%.0f\n",
						p.City, p.Count, p.MedianPrice, p.InstantBookPct, p.CapacityP50)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), pulses, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagCity, "city", nil, "City filter (repeatable)")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
