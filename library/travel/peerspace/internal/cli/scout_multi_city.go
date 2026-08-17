// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: scout multi-city — top venues per city under shared constraints.

package cli

// pp:data-source local

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelScoutMultiCityCmd(flags *rootFlags) *cobra.Command {
	var flagCity []string
	var flagActivity string
	var flagGuests int
	var flagBudgetMax float64
	var flagTop int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "multi-city",
		Short:       "Cross-city comparison with top venue options per city under shared tech-event constraints.",
		Example:     "  peerspace-pp-cli scout multi-city --city Paris --city Lyon --guests 30 --budget-max 200 --top 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagTop <= 0 {
				flagTop = 3
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
					"cities": make(map[string]any),
					"top":    flagTop,
				}, flags)
			}
			defer s.Close()

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			// If no cities provided, derive from data.
			cities := flagCity
			if len(cities) == 0 {
				seen := map[string]struct{}{}
				for _, l := range listings {
					if l.City == "" {
						continue
					}
					if _, ok := seen[l.City]; ok {
						continue
					}
					seen[l.City] = struct{}{}
					cities = append(cities, l.City)
					if len(cities) >= 10 {
						break
					}
				}
			}
			ranked := venuex.MultiCityTop(listings, cities, flagActivity, flagGuests, flagBudgetMax, flagTop)
			// Serialize compact rows
			outCities := make(map[string]any, len(ranked))
			for city, rows := range ranked {
				compact := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					row := listingToRow(r.Listing)
					row["score"] = r.Score
					row["gaps"] = r.Gaps
					compact = append(compact, row)
				}
				outCities[city] = compact
			}
			result := map[string]any{
				"cities":     outCities,
				"activity":   flagActivity,
				"guests":     flagGuests,
				"budget_max": flagBudgetMax,
				"top":        flagTop,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, city := range cities {
					rows, _ := ranked[city]
					fmt.Fprintf(cmd.OutOrStdout(), "## %s (%d)\n", city, len(rows))
					for _, r := range rows {
						fmt.Fprintf(cmd.OutOrStdout(), "  score=%d price=%s guests=%d %s %s\n",
							r.Score, formatFloat(r.PriceHourly), r.Guests, r.ID, r.Title)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagCity, "city", nil, "City to include (repeatable)")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity/use filter")
	cmd.Flags().IntVar(&flagGuests, "guests", 0, "Minimum guest capacity")
	cmd.Flags().Float64Var(&flagBudgetMax, "budget-max", 0, "Maximum hourly price")
	cmd.Flags().IntVar(&flagTop, "top", 3, "Top N venues per city")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}

func formatFloat(v float64) string {
	if v == float64(int(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
