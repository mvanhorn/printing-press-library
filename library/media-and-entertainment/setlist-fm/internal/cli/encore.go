package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newEncoreCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:     "encore <artist>",
		Short:   "Top encore songs, encore frequency, and average encore length",
		Example: `  setlist-fm-pp-cli encore "Radiohead" --top 10`,
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

			showsWithEncore := 0
			totalEncoreSongs := 0
			encoreOpeners := map[string]int{}
			encoreClosers := map[string]int{}
			encoreSongs := map[string]int{}

			for _, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil || len(songs) == 0 {
					continue
				}

				// Find encore songs
				var encoreItems []struct {
					name string
					pos  int
				}
				for _, song := range songs {
					if song.IsEncore {
						encoreItems = append(encoreItems, struct {
							name string
							pos  int
						}{song.Name, song.Position})
						encoreSongs[song.Name]++
					}
				}

				if len(encoreItems) > 0 {
					showsWithEncore++
					totalEncoreSongs += len(encoreItems)
					encoreOpeners[encoreItems[0].name]++
					encoreClosers[encoreItems[len(encoreItems)-1].name]++
				}
			}

			encorePct := 0.0
			avgEncoreLen := 0.0
			if len(setlists) > 0 {
				encorePct = float64(showsWithEncore) / float64(len(setlists)) * 100
			}
			if showsWithEncore > 0 {
				avgEncoreLen = float64(totalEncoreSongs) / float64(showsWithEncore)
			}

			topEncoreOpeners := topN(encoreOpeners, top)
			topEncoreClosers := topN(encoreClosers, top)

			// Top encore songs overall
			type songFreq struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}
			var topSongs []songFreq
			for name, count := range encoreSongs {
				topSongs = append(topSongs, songFreq{name, count})
			}
			sort.Slice(topSongs, func(i, j int) bool {
				return topSongs[i].Count > topSongs[j].Count
			})
			if top > 0 && top < len(topSongs) {
				topSongs = topSongs[:top]
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":             name,
					"total_shows":        len(setlists),
					"shows_with_encore":  showsWithEncore,
					"encore_pct":         fmt.Sprintf("%.1f%%", encorePct),
					"avg_encore_length":  fmt.Sprintf("%.1f", avgEncoreLen),
					"top_encore_openers": topEncoreOpeners,
					"top_encore_closers": topEncoreClosers,
					"top_encore_songs":   topSongs,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Encore Analysis for %s\n\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "  Shows with encore:  %d / %d (%.1f%%)\n", showsWithEncore, len(setlists), encorePct)
			fmt.Fprintf(cmd.OutOrStdout(), "  Avg encore length:  %.1f songs\n\n", avgEncoreLen)

			fmt.Fprintf(cmd.OutOrStdout(), "Top Encore Songs:\n")
			headers := []string{"#", "SONG", "ENCORE PLAYS"}
			var rows [][]string
			for i, s := range topSongs {
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					s.Name,
					fmt.Sprintf("%d", s.Count),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&top, "top", 10, "Number of songs to show")
	return cmd
}
