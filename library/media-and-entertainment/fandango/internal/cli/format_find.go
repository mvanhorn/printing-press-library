// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelFormatFindCmd(flags *rootFlags) *cobra.Command {
	var flagMovieId string
	var flagIdProvider string
	var flagZipCode string

	cmd := &cobra.Command{
		Use:         "format-find",
		Short:       "Compare a movie's available presentation formats across nearby theaters.",
		Example:     "  fandango-pp-cli format-find --movie-id 12345 --id-provider fandangoApi --zip-code 10001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare Fandango formats")
				return nil
			}
			if flagMovieId == "" || flagIdProvider == "" || flagZipCode == "" {
				return usageErr(fmt.Errorf("--movie-id, --id-provider, and --zip-code are required"))
			}
			rows, err := fetchFandangoShowtimes(cmd, flags, map[string]string{
				"MovieId": flagMovieId, "MovieIdProvider": flagIdProvider, "ZipCode": flagZipCode,
				"Radius": "25", "Limit": "100",
			})
			if err != nil {
				return err
			}
			groups := map[string][]fandangoPlanRow{}
			for _, row := range rows {
				key := strings.TrimSpace(row.Format)
				if key == "" {
					key = "Standard or unspecified"
				}
				groups[key] = append(groups[key], row)
			}
			formats := make([]string, 0, len(groups))
			for key := range groups {
				formats = append(formats, key)
			}
			sort.Strings(formats)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"movie_id": flagMovieId, "formats": formats, "showtimes_by_format": groups,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagMovieId, "movie-id", "", "Fandango or Fabric Origin movie ID")
	cmd.Flags().StringVar(&flagIdProvider, "id-provider", "fandangoApi", "Movie ID provider: fandangoApi, fandango, or IVA")
	cmd.Flags().StringVar(&flagZipCode, "zip-code", "", "Postal code for nearby showtimes")
	return cmd
}
