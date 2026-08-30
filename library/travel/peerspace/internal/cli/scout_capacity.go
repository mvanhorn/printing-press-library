// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: scout capacity — guest capacity histogram.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelScoutCapacityCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagActivity string
	var flagBand int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "capacity",
		Short:       "Histogram of guest capacity for a synced market so you can re-cut when headcount changes.",
		Example:     "  peerspace-pp-cli scout capacity --city Paris --activity meetup --band 10 --json",
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
			bands := venuex.BandCapacity(filtered, flagBand)
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
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter")
	cmd.Flags().IntVar(&flagBand, "band", 10, "Guest capacity band width")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
