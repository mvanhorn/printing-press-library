package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSongStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "song-stats <artist> <song-name>",
		Short: "Detailed statistics for one song by one artist",
		Long: `Shows total plays, first/last played dates, longest gap between plays,
average set position, and percentage of shows that included the song.`,
		Example: `  setlist-fm-pp-cli song-stats "Radiohead" "Karma Police"`,
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
			totalSetlists, err := db.CountSetlists(mbid)
			if err != nil {
				return err
			}

			// Get all setlists and find the song
			setlists, err := db.GetArtistSetlists(mbid)
			if err != nil {
				return err
			}

			type playInfo struct {
				date     string
				position int
			}
			var plays []playInfo
			for _, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if strings.EqualFold(song.Name, songName) {
						plays = append(plays, playInfo{date: sl.EventDate, position: song.Position})
						break
					}
				}
			}

			if len(plays) == 0 {
				return fmt.Errorf("song %q not found in any setlists for %s", songName, name)
			}

			// Compute stats
			firstDate := plays[len(plays)-1].date
			lastDate := plays[0].date
			avgPos := 0.0
			for _, p := range plays {
				avgPos += float64(p.position)
			}
			avgPos /= float64(len(plays))

			// Longest gap in days
			longestGap := 0
			longestGapStart := ""
			longestGapEnd := ""
			for i := 0; i < len(plays)-1; i++ {
				d1 := parseSetlistDate(plays[i].date)
				d2 := parseSetlistDate(plays[i+1].date)
				if !d1.IsZero() && !d2.IsZero() {
					gap := int(math.Abs(d1.Sub(d2).Hours() / 24))
					if gap > longestGap {
						longestGap = gap
						longestGapStart = plays[i+1].date
						longestGapEnd = plays[i].date
					}
				}
			}

			pct := float64(len(plays)) / float64(totalSetlists) * 100

			result := map[string]any{
				"artist":           name,
				"song":             songName,
				"total_plays":      len(plays),
				"total_setlists":   totalSetlists,
				"first_played":     firstDate,
				"last_played":      lastDate,
				"percentage":       fmt.Sprintf("%.1f%%", pct),
				"avg_position":     fmt.Sprintf("%.1f", avgPos),
				"longest_gap_days": longestGap,
				"gap_start":        longestGapStart,
				"gap_end":          longestGapEnd,
			}

			if flags.asJSON {
				return outputJSON(cmd, result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Song Stats: %s by %s\n\n", songName, name)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total plays:       %d / %d shows (%.1f%%)\n", len(plays), totalSetlists, pct)
			fmt.Fprintf(cmd.OutOrStdout(), "  First played:      %s\n", firstDate)
			fmt.Fprintf(cmd.OutOrStdout(), "  Last played:       %s\n", lastDate)
			fmt.Fprintf(cmd.OutOrStdout(), "  Avg set position:  %.1f\n", avgPos)
			if longestGap > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Longest gap:       %d days (%s to %s)\n", longestGap, longestGapStart, longestGapEnd)
			}
			return nil
		},
	}
	return cmd
}

// parseSetlistDate parses the DD-MM-YYYY date format from the API.
func parseSetlistDate(s string) time.Time {
	t, err := time.Parse("02-01-2006", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
