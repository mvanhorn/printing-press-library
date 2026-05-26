package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func newHeatmapCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "heatmap",
		Short:       "Render a weekday-by-hour activity heatmap of visit counts over --since (ASCII by default, grid via --json)",
		Example:     strings.Trim("safari-history-pp-cli heatmap --since 30d", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, _, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			st, err := store.OpenExisting(snapshot)
			if err != nil {
				if errors.Is(err, store.ErrNoSnapshot) {
					return ErrNoSnapshot
				}
				return err
			}
			defer st.Close()
			events, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: time.Now().UTC(), Limit: 50000, Device: opts.Device})
			if err != nil {
				return err
			}
			var grid [7][24]int64
			for _, e := range events {
				t := e.VisitTime.In(time.Local)
				grid[int(t.Weekday())][t.Hour()]++
			}
			if !opts.Output.JSON && !opts.Output.CSV && !opts.Output.Quiet {
				printHeatmapASCII(grid)
				return nil
			}
			jsonGrid := make([][]int64, 7)
			for d := 0; d < 7; d++ {
				jsonGrid[d] = make([]int64, 24)
				for h := 0; h < 24; h++ {
					jsonGrid[d][h] = grid[d][h]
				}
			}
			out := map[string]any{"since": sinceOrDefault(since, "30d"), "time_zone": time.Local.String(), "weekday_hour_counts": jsonGrid}
			maybePrintEmptyWindowHint(st.DB(), since, len(events) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}

func printHeatmapASCII(grid [7][24]int64) {
	max := int64(0)
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			if grid[d][h] > max {
				max = grid[d][h]
			}
		}
	}
	scale := []string{" ", "░", "▒", "▓", "█"}
	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for d := 0; d < 7; d++ {
		line := days[d] + " "
		for h := 0; h < 24; h++ {
			idx := 0
			if max > 0 {
				idx = int((grid[d][h] * 4) / max)
			}
			line += scale[idx]
		}
		fmt.Println(line)
	}
}
