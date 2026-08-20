package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newOverdueCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:     "overdue <artist>",
		Short:   "Rank songs by shows since last played — what's most overdue to return",
		Example: `  setlist-fm-pp-cli overdue "Radiohead" --top 30`,
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

			// Track when each song was last played (by setlist index, 0=most recent)
			type overdueInfo struct {
				Name           string `json:"name"`
				ShowsSinceLast int    `json:"shows_since_last"`
				LastPlayed     string `json:"last_played"`
				TotalPlays     int    `json:"total_plays"`
			}
			songLastIdx := map[string]int{}
			songPlays := map[string]int{}
			songLastDate := map[string]string{}
			songDisplayName := map[string]string{}

			for i, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if song.Name == "" {
						continue
					}
					key := strings.ToLower(song.Name)
					songPlays[key]++
					if _, ok := songLastIdx[key]; !ok {
						songLastIdx[key] = i
						songLastDate[key] = sl.EventDate
						songDisplayName[key] = song.Name
					}
				}
			}

			var results []overdueInfo
			for key, lastIdx := range songLastIdx {
				results = append(results, overdueInfo{
					Name:           songDisplayName[key],
					ShowsSinceLast: lastIdx,
					LastPlayed:     songLastDate[key],
					TotalPlays:     songPlays[key],
				})
			}

			// Sort by shows since last played, descending
			sort.Slice(results, func(i, j int) bool {
				return results[i].ShowsSinceLast > results[j].ShowsSinceLast
			})

			if top > 0 && top < len(results) {
				results = results[:top]
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":  name,
					"overdue": results,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Most overdue songs for %s:\n\n", name)
			headers := []string{"#", "SONG", "SHOWS SINCE", "LAST PLAYED", "TOTAL PLAYS"}
			var rows [][]string
			for i, r := range results {
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					r.Name,
					fmt.Sprintf("%d", r.ShowsSinceLast),
					r.LastPlayed,
					fmt.Sprintf("%d", r.TotalPlays),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&top, "top", 20, "Number of songs to show")
	return cmd
}
