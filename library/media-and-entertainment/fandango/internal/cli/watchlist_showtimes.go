// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelWatchlistShowtimesCmd(flags *rootFlags) *cobra.Command {
	var flagZipCode string
	var movies string

	cmd := &cobra.Command{
		Use:         "watchlist-showtimes",
		Short:       "Match a local movie watchlist against currently bookable nearby showtimes.",
		Example:     "  fandango-pp-cli watchlist-showtimes --zip-code 10001 --agent --select matches.title,matches.theater,matches.start,matches.purchase_url",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would match a movie watchlist against Fandango showtimes")
				return nil
			}
			watchlist := splitNonEmpty(movies)
			if flagZipCode == "" || len(watchlist) == 0 {
				return usageErr(fmt.Errorf("--zip-code and --movies are required"))
			}
			rows, err := fetchFandangoShowtimes(cmd, flags, map[string]string{
				"ZipCode": flagZipCode, "Radius": "25", "Limit": "100",
			})
			if err != nil {
				return err
			}
			matches := make([]fandangoPlanRow, 0)
			for _, row := range rows {
				title := strings.ToLower(row.Title)
				for _, wanted := range watchlist {
					if strings.Contains(title, strings.ToLower(wanted)) {
						matches = append(matches, row)
						break
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"zip_code": flagZipCode, "watchlist": watchlist, "count": len(matches), "matches": matches,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagZipCode, "zip-code", "", "Postal code for nearby showtimes")
	cmd.Flags().StringVar(&movies, "movies", "", "Comma-separated movie titles to match")
	return cmd
}
