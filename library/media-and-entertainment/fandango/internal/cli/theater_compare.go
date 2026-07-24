// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelTheaterCompareCmd(flags *rootFlags) *cobra.Command {
	var flagTheaterIds string
	var flagDate string

	cmd := &cobra.Command{
		Use:         "theater-compare",
		Short:       "Compare theaters by movie coverage, showtime density, operating span, and ticket-link coverage.",
		Example:     "  fandango-pp-cli theater-compare --theater-ids 100,200 --date 2026-07-25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare official Fandango theater inventory")
				return nil
			}
			ids := splitNonEmpty(flagTheaterIds)
			if len(ids) < 2 {
				return usageErr(fmt.Errorf("--theater-ids must contain at least two comma-separated IDs"))
			}
			if flagDate == "" {
				flagDate = time.Now().Format("2006-01-02")
			}
			type comparison struct {
				TheaterID     string `json:"theater_id"`
				Theater       string `json:"theater,omitempty"`
				Movies        int    `json:"movies"`
				Showtimes     int    `json:"showtimes"`
				Earliest      string `json:"earliest,omitempty"`
				Latest        string `json:"latest,omitempty"`
				PurchaseLinks int    `json:"purchase_links"`
				Error         string `json:"error,omitempty"`
			}
			results := make([]comparison, 0, len(ids))
			for _, id := range ids {
				rows, err := fetchFandangoShowtimes(cmd, flags, map[string]string{
					"TheaterId": id, "TheaterIdProvider": "fandangoApi",
					"StartDisplayDate": flagDate, "EndDisplayDate": flagDate, "Limit": "100",
				})
				if err != nil {
					results = append(results, comparison{TheaterID: id, Error: err.Error()})
					continue
				}
				movies := map[string]struct{}{}
				links := 0
				for _, row := range rows {
					movies[row.MovieID+"|"+row.Title] = struct{}{}
					if row.PurchaseURL != "" {
						links++
					}
				}
				item := comparison{TheaterID: id, Movies: len(movies), Showtimes: len(rows), PurchaseLinks: links}
				if len(rows) > 0 {
					item.Theater, item.Earliest, item.Latest = rows[0].Theater, rows[0].Start, rows[len(rows)-1].Start
				}
				results = append(results, item)
			}
			sort.SliceStable(results, func(i, j int) bool { return results[i].Showtimes > results[j].Showtimes })
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"date": flagDate, "theaters": results}, flags)
		},
	}
	cmd.Flags().StringVar(&flagTheaterIds, "theater-ids", "", "Comma-separated Fandango theater IDs")
	cmd.Flags().StringVar(&flagDate, "date", "", "Showtime date (YYYY-MM-DD; default today)")
	return cmd
}
