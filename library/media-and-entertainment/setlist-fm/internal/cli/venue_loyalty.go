package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newVenueLoyaltyCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:     "venue-loyalty <artist>",
		Short:   "Top venues by number of shows, with home venue detection",
		Example: `  setlist-fm-pp-cli venue-loyalty "Radiohead" --top 15`,
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

			setlists, err := db.GetArtistSetlists(mbid)
			if err != nil {
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for %s", name)
			}

			type venueCount struct {
				VenueID   string `json:"venue_id"`
				VenueName string `json:"venue_name"`
				City      string `json:"city"`
				Country   string `json:"country"`
				Shows     int    `json:"shows"`
				IsHome    bool   `json:"is_home,omitempty"`
			}
			venueCounts := map[string]*venueCount{}

			for _, sl := range setlists {
				if sl.VenueID == "" {
					continue
				}
				if _, ok := venueCounts[sl.VenueID]; !ok {
					venueCounts[sl.VenueID] = &venueCount{
						VenueID:   sl.VenueID,
						VenueName: sl.VenueName,
						City:      sl.CityName,
						Country:   sl.CountryCode,
					}
				}
				venueCounts[sl.VenueID].Shows++
			}

			var venues []venueCount
			for _, v := range venueCounts {
				venues = append(venues, *v)
			}
			sort.Slice(venues, func(i, j int) bool {
				return venues[i].Shows > venues[j].Shows
			})

			// Detect home venue: #1 has significantly more shows than #2
			if len(venues) >= 2 && venues[0].Shows >= venues[1].Shows*2 {
				venues[0].IsHome = true
			}

			if top > 0 && top < len(venues) {
				venues = venues[:top]
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist": name,
					"venues": venues,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Venue Loyalty for %s:\n\n", name)
			headers := []string{"#", "VENUE", "CITY", "COUNTRY", "SHOWS", ""}
			var rows [][]string
			for i, v := range venues {
				badge := ""
				if v.IsHome {
					badge = "HOME"
				}
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					v.VenueName,
					v.City,
					v.Country,
					fmt.Sprintf("%d", v.Shows),
					badge,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&top, "top", 20, "Number of venues to show")
	return cmd
}
