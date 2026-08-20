package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newSongGapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "song-gap <artist> <song-name>",
		Short: "Find the biggest gaps between plays of one song",
		Long: `Shows each gap between performances of a specific song, measured in
both shows and calendar days. Identifies when the song went away and
when it came back.`,
		Example: `  setlist-fm-pp-cli song-gap "Radiohead" "Creep"`,
		Args:    cobra.ExactArgs(2),
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
			songName := args[1]

			setlists, err := db.GetArtistSetlists(mbid)
			if err != nil {
				return err
			}

			// Find all dates where the song was played (chronological order)
			type playDate struct {
				Date      string
				SetlistID string
				ShowIndex int
			}
			var playDates []playDate

			for i, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if strings.EqualFold(song.Name, songName) {
						playDates = append(playDates, playDate{
							Date:      sl.EventDate,
							SetlistID: sl.ID,
							ShowIndex: i,
						})
						break
					}
				}
			}

			if len(playDates) < 2 {
				if len(playDates) == 1 {
					return fmt.Errorf("song %q was only played once by %s (on %s); need at least 2 plays to compute gaps", songName, name, playDates[0].Date)
				}
				return fmt.Errorf("song %q not found in any setlists for %s", songName, name)
			}

			// Reverse so chronological (setlists come newest-first)
			for i, j := 0, len(playDates)-1; i < j; i, j = i+1, j-1 {
				playDates[i], playDates[j] = playDates[j], playDates[i]
			}

			type gap struct {
				From  string `json:"from"`
				To    string `json:"to"`
				Days  int    `json:"days"`
				Shows int    `json:"shows_between"`
			}
			var gaps []gap

			for i := 0; i < len(playDates)-1; i++ {
				d1 := parseSetlistDate(playDates[i].Date)
				d2 := parseSetlistDate(playDates[i+1].Date)
				days := 0
				if !d1.IsZero() && !d2.IsZero() {
					days = int(math.Abs(d2.Sub(d1).Hours() / 24))
				}
				shows := int(math.Abs(float64(playDates[i+1].ShowIndex - playDates[i].ShowIndex)))
				gaps = append(gaps, gap{
					From:  playDates[i].Date,
					To:    playDates[i+1].Date,
					Days:  days,
					Shows: shows,
				})
			}

			// Sort by days descending
			sort.Slice(gaps, func(i, j int) bool {
				return gaps[i].Days > gaps[j].Days
			})

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":      name,
					"song":        songName,
					"total_plays": len(playDates),
					"gaps":        gaps,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Song Gaps: %q by %s (%d total plays)\n\n", songName, name, len(playDates))
			headers := []string{"#", "FROM", "TO", "DAYS", "SHOWS BETWEEN"}
			var rows [][]string
			for i, g := range gaps {
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					g.From,
					g.To,
					fmt.Sprintf("%d", g.Days),
					fmt.Sprintf("%d", g.Shows),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}
