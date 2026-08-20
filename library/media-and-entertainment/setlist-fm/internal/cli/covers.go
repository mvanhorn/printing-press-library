package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newCoversCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:     "covers <artist>",
		Short:   "All cover songs played by an artist, ranked by frequency",
		Example: `  setlist-fm-pp-cli covers "Radiohead" --top 20`,
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

			type coverInfo struct {
				Song           string `json:"song"`
				OriginalArtist string `json:"original_artist"`
				Count          int    `json:"count"`
			}
			coverMap := map[string]*coverInfo{}

			for _, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if song.IsCover && song.CoverArtistName != "" {
						key := song.Name + "|" + song.CoverArtistName
						if _, ok := coverMap[key]; !ok {
							coverMap[key] = &coverInfo{
								Song:           song.Name,
								OriginalArtist: song.CoverArtistName,
							}
						}
						coverMap[key].Count++
					}
				}
			}

			var covers []coverInfo
			for _, c := range coverMap {
				covers = append(covers, *c)
			}
			sort.Slice(covers, func(i, j int) bool {
				return covers[i].Count > covers[j].Count
			})

			if top > 0 && top < len(covers) {
				covers = covers[:top]
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":       name,
					"total_covers": len(covers),
					"covers":       covers,
				}, flags)
			}

			if len(covers) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No covers found for %s\n", name)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Covers played by %s:\n\n", name)
			headers := []string{"#", "SONG", "ORIGINAL ARTIST", "TIMES PLAYED"}
			var rows [][]string
			for i, c := range covers {
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					c.Song,
					c.OriginalArtist,
					fmt.Sprintf("%d", c.Count),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&top, "top", 0, "Limit results (0 = all)")
	return cmd
}
