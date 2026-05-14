package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newCtrDecayCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "ctr-decay [video_id]",
		Short: "Compare first-72h CTR to day-30 CTR for a video",
		Long: strings.TrimSpace(`
Reads daily metrics from the local store and computes the CTR delta between
the first 72 hours after publish and the day-30 window. Flags fast-decaying
thumbnails (>20% drop) and steady performers.

Run 'yt-studio-pp-cli sync' first to populate daily metrics.`),
		Example:     "  yt-studio-pp-cli ctr-decay dQw4w9WgXcQ --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			videoID := args[0]
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			// Get all metrics for the video (wide window)
			from := time.Now().UTC().AddDate(0, 0, -180).Format("2006-01-02")
			to := time.Now().UTC().Format("2006-01-02")
			rows, err := ytstore.MetricsRange(ctx, db, videoID, from, to)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return notFoundErr(fmt.Errorf("no daily metrics for video %s; run `yt-studio-pp-cli sync` first", videoID))
			}

			// First 3 days (sorted ascending) vs day 28-30 window
			early := rows[:min3(len(rows), 3)]
			lateStart := 27
			if len(rows) <= lateStart {
				lateStart = len(rows) - 1
			}
			late := rows[lateStart:]

			earlyCTR := avgCTR(early)
			lateCTR := avgCTR(late)
			delta := earlyCTR - lateCTR
			rel := 0.0
			if earlyCTR > 0 {
				rel = delta / earlyCTR
			}
			verdict := "steady"
			switch {
			case rel >= 0.20:
				verdict = "fast-decay"
			case rel >= 0.10:
				verdict = "moderate-decay"
			case rel <= -0.05:
				verdict = "rising"
			}

			res := map[string]any{
				"video_id":       videoID,
				"early_ctr":      earlyCTR,
				"late_ctr":       lateCTR,
				"absolute_delta": delta,
				"relative_delta": rel,
				"verdict":        verdict,
				"early_window":   fmt.Sprintf("%d days", len(early)),
				"late_window":    fmt.Sprintf("%d days starting day %d", len(late), lateStart),
				"days_in_store":  len(rows),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ctr-decay for %s\n", videoID)
			fmt.Fprintf(cmd.OutOrStdout(), "  early CTR (days 1-%d): %.3f%%\n", len(early), earlyCTR*100)
			fmt.Fprintf(cmd.OutOrStdout(), "  late  CTR (day %d+):   %.3f%%\n", lateStart, lateCTR*100)
			fmt.Fprintf(cmd.OutOrStdout(), "  delta:                 %.3f%% (relative %.1f%%)\n", delta*100, rel*100)
			fmt.Fprintf(cmd.OutOrStdout(), "  verdict:               %s\n", verdict)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func avgCTR(rows []ytstore.VideoMetricsDay) float64 {
	if len(rows) == 0 {
		return 0
	}
	sum := 0.0
	n := 0
	for _, r := range rows {
		if r.CTR > 0 {
			sum += r.CTR
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}
