package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newDebutCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:     "debut <artist>",
		Short:   "Songs played exactly once live — rare finds for collectors",
		Example: `  setlist-fm-pp-cli debut "Radiohead" --top 50`,
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

			// Count plays per song
			type songInfo struct {
				Name  string
				Date  string
				Venue string
			}
			songPlays := map[string]int{}
			songFirst := map[string]songInfo{}

			for _, sl := range setlists {
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
					if _, ok := songFirst[key]; !ok {
						songFirst[key] = songInfo{
							Name:  song.Name,
							Date:  sl.EventDate,
							Venue: sl.VenueName,
						}
					}
					// Keep updating to get the latest occurrence, since setlists come newest-first
					songFirst[key] = songInfo{
						Name:  song.Name,
						Date:  sl.EventDate,
						Venue: sl.VenueName,
					}
				}
			}

			type debutEntry struct {
				Song  string `json:"song"`
				Date  string `json:"date"`
				Venue string `json:"venue"`
			}
			var debuts []debutEntry
			for key, count := range songPlays {
				if count == 1 {
					info := songFirst[key]
					debuts = append(debuts, debutEntry{
						Song:  info.Name,
						Date:  info.Date,
						Venue: info.Venue,
					})
				}
			}

			sort.Slice(debuts, func(i, j int) bool {
				return debuts[i].Date > debuts[j].Date
			})

			if top > 0 && top < len(debuts) {
				debuts = debuts[:top]
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":       name,
					"total_debuts": len(debuts),
					"debuts":       debuts,
				}, flags)
			}

			if len(debuts) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No one-time-only songs found for %s\n", name)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Songs played exactly once by %s (%d total):\n\n", name, len(debuts))
			headers := []string{"SONG", "DATE", "VENUE"}
			var rows [][]string
			for _, d := range debuts {
				rows = append(rows, []string{d.Song, d.Date, d.Venue})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&top, "top", 0, "Limit results (0 = all)")
	return cmd
}
