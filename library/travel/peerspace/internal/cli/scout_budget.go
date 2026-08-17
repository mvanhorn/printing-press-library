// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: scout budget — market price bands over local listings.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelScoutBudgetCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagActivity string
	var flagBand int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "budget",
		Short:       "Band market listings by hourly/day rate so you can see what a city+activity actually costs.",
		Example:     "  peerspace-pp-cli scout budget --city Paris --activity meetup --band 50 --json",
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
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"bands":    make([]venuex.Band, 0),
					"city":     flagCity,
					"activity": flagActivity,
					"scanned":  0,
				}, flags)
			}
			defer s.Close()

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			filtered := venuex.FilterListings(listings, flagCity, flagActivity)
			bands := venuex.BandPrices(filtered, float64(flagBand))
			if bands == nil {
				bands = make([]venuex.Band, 0)
			}
			result := map[string]any{
				"bands":    bands,
				"city":     flagCity,
				"activity": flagActivity,
				"scanned":  len(filtered),
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "city=%s activity=%s scanned=%d band=%d\n", flagCity, flagActivity, len(filtered), flagBand)
				for _, b := range bands {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%.0f-%.0f) count=%d samples=%v\n", b.Min, b.Max, b.Count, b.SampleIDs)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City filter (substring match)")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter (substring on title/description/space type)")
	cmd.Flags().IntVar(&flagBand, "band", 50, "Hourly price band width")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path (default: resolved data directory)")
	return cmd
}
