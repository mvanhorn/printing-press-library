package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTourRouteCmd(flags *rootFlags) *cobra.Command {
	var tourName string

	cmd := &cobra.Command{
		Use:     "tour-route <artist>",
		Short:   "Show the chronological route of a tour: dates, cities, venues",
		Example: `  setlist-fm-pp-cli tour-route "Radiohead" --tour "A Moon Shaped Pool Tour"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreOrFail(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			mbid, name, err := resolveArtistFromStore(db, args[0])
			if err != nil {
				return err
			}

			if tourName == "" {
				tourName, err = db.GetLatestTourName(mbid)
				if err != nil || tourName == "" {
					return fmt.Errorf("no tour found for %s; specify --tour", name)
				}
			}

			setlists, err := db.GetSetlistsByTour(mbid, tourName)
			if err != nil {
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for tour %q by %s", tourName, name)
			}

			if flags.asJSON {
				type stop struct {
					Date    string `json:"date"`
					Venue   string `json:"venue"`
					City    string `json:"city"`
					Country string `json:"country"`
					Songs   int    `json:"songs"`
				}
				var stops []stop
				for _, sl := range setlists {
					stops = append(stops, stop{
						Date:    sl.EventDate,
						Venue:   sl.VenueName,
						City:    sl.CityName,
						Country: sl.CountryCode,
						Songs:   sl.SongCount,
					})
				}
				return outputJSON(cmd, map[string]any{
					"artist": name,
					"tour":   tourName,
					"shows":  len(stops),
					"route":  stops,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tour Route: %s — %s (%d shows)\n\n", name, tourName, len(setlists))
			headers := []string{"#", "DATE", "VENUE", "CITY", "COUNTRY", "SONGS"}
			var rows [][]string
			for i, sl := range setlists {
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					sl.EventDate,
					truncate(sl.VenueName, 30),
					sl.CityName,
					sl.CountryCode,
					fmt.Sprintf("%d", sl.SongCount),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&tourName, "tour", "", "Tour name (default: most recent)")
	return cmd
}
