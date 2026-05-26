package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var artistFilter string

	cmd := &cobra.Command{
		Use:   "since <timestamp>",
		Short: "Setlists updated since a given timestamp",
		Long: `Returns setlists from the local store whose last_updated timestamp is
on or after the given ISO timestamp. Useful for delta refresh workflows.`,
		Example: `  setlist-fm-pp-cli since 2024-01-01T00:00:00Z
  setlist-fm-pp-cli since 2024-06-15 --artist "Radiohead"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreOrFail(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			ts, err := parseSinceTimestamp(args[0])
			if err != nil {
				return err
			}

			// Resolve artist filter if provided
			artistMBID := ""
			if artistFilter != "" {
				mbid, _, err := resolveArtistFromStore(db, artistFilter)
				if err != nil {
					return err
				}
				artistMBID = mbid
			}

			setlists, err := db.GetSetlistsSince(ts, artistMBID)
			if err != nil {
				return err
			}

			if flags.asJSON {
				type slOut struct {
					ID          string `json:"id"`
					Artist      string `json:"artist"`
					Venue       string `json:"venue"`
					City        string `json:"city"`
					Date        string `json:"event_date"`
					LastUpdated string `json:"last_updated"`
				}
				var results []slOut
				for _, sl := range setlists {
					results = append(results, slOut{
						ID:          sl.ID,
						Artist:      sl.ArtistName,
						Venue:       sl.VenueName,
						City:        sl.CityName,
						Date:        sl.EventDate,
						LastUpdated: sl.LastUpdated,
					})
				}
				return outputJSON(cmd, map[string]any{
					"since":   ts,
					"count":   len(results),
					"results": results,
				}, flags)
			}

			if len(setlists) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No setlists updated since %s\n", ts)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Setlists updated since %s (%d):\n\n", ts, len(setlists))
			headers := []string{"ID", "ARTIST", "VENUE", "CITY", "DATE", "UPDATED"}
			var rows [][]string
			for _, sl := range setlists {
				rows = append(rows, []string{
					sl.ID,
					truncate(sl.ArtistName, 20),
					truncate(sl.VenueName, 25),
					sl.CityName,
					sl.EventDate,
					sl.LastUpdated,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&artistFilter, "artist", "", "Filter by artist name or MBID")
	return cmd
}

func parseSinceTimestamp(raw string) (string, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("invalid timestamp %q: expected RFC3339, \"2006-01-02T15:04:05\", or \"2006-01-02\"", raw)
}
