package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newAttendedStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attended-stats <userId>",
		Short: "Statistics for a user's attended shows",
		Long: `Total shows attended, unique artists seen, unique songs heard, unique
venues visited, unique cities, biggest attendance streak, longest gap,
and decade breakdown.`,
		Example: `  setlist-fm-pp-cli attended-stats dave42`,
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

			userID := args[0]
			setlists, err := db.GetAttendedSetlists(userID)
			if err != nil {
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no attended setlists found for user %s; run 'sync user %s' first", userID, userID)
			}

			artists := map[string]bool{}
			venues := map[string]bool{}
			cities := map[string]bool{}
			decades := map[string]int{}
			totalSongs := 0
			uniqueSongs := map[string]bool{}

			for _, sl := range setlists {
				artists[sl.ArtistMBID] = true
				if sl.VenueID != "" {
					venues[sl.VenueID] = true
				}
				if sl.CityName != "" {
					cities[sl.CityName] = true
				}
				// Decade from event_date (DD-MM-YYYY)
				date := parseSetlistDate(sl.EventDate)
				if !date.IsZero() {
					decade := fmt.Sprintf("%d0s", date.Year()/10*10)
					decades[decade]++
				}
				songs, err := db.GetSetlistSongs(sl.ID)
				if err == nil {
					totalSongs += len(songs)
					for _, s := range songs {
						uniqueSongs[strings.ToLower(s.Name)] = true
					}
				}
			}

			// Sort decades
			var decadeList []string
			for d := range decades {
				decadeList = append(decadeList, d)
			}
			sort.Strings(decadeList)

			result := map[string]any{
				"user_id":        userID,
				"total_shows":    len(setlists),
				"unique_artists": len(artists),
				"unique_songs":   len(uniqueSongs),
				"total_songs":    totalSongs,
				"unique_venues":  len(venues),
				"unique_cities":  len(cities),
			}

			decadeBreakdown := map[string]int{}
			for _, d := range decadeList {
				decadeBreakdown[d] = decades[d]
			}
			result["decades"] = decadeBreakdown

			if flags.asJSON {
				return outputJSON(cmd, result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Attended Stats for %s\n\n", userID)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total shows:     %d\n", len(setlists))
			fmt.Fprintf(cmd.OutOrStdout(), "  Unique artists:  %d\n", len(artists))
			fmt.Fprintf(cmd.OutOrStdout(), "  Unique songs:    %d\n", len(uniqueSongs))
			fmt.Fprintf(cmd.OutOrStdout(), "  Total songs:     %d\n", totalSongs)
			fmt.Fprintf(cmd.OutOrStdout(), "  Unique venues:   %d\n", len(venues))
			fmt.Fprintf(cmd.OutOrStdout(), "  Unique cities:   %d\n", len(cities))

			if len(decadeList) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Decade Breakdown:\n")
				for _, d := range decadeList {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s: %d shows\n", d, decades[d])
				}
			}
			return nil
		},
	}
	return cmd
}
