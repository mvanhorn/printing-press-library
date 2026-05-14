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

			// Anchor the early window to the video's publish date. The naive
			// "first 3 rows in a 180-day look-back" approach silently breaks
			// for videos older than 180 days: rows[0..2] become days
			// 180-178 ago, not the first 72 hours after publish. Pulling the
			// publish_at from yt_videos fixes the anchor.
			var publishedAt string
			_ = db.QueryRowContext(ctx,
				`SELECT COALESCE(published_at,'') FROM yt_videos WHERE video_id = ?`,
				videoID).Scan(&publishedAt)
			if publishedAt == "" {
				return notFoundErr(fmt.Errorf("video %s is not in the local store; run `yt-studio-pp-cli sync` first to capture its publish_at", videoID))
			}
			publishDate, err := parseVideoPublishedAt(publishedAt)
			if err != nil {
				return notFoundErr(fmt.Errorf("video %s has unparseable published_at %q: %w", videoID, publishedAt, err))
			}

			// Wide window starts at publish date; ends today. Avoids the
			// 180-day look-back's broken anchoring.
			from := publishDate.UTC().Format("2006-01-02")
			to := time.Now().UTC().Format("2006-01-02")
			rows, err := ytstore.MetricsRange(ctx, db, videoID, from, to)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return notFoundErr(fmt.Errorf("no daily metrics for video %s; run `yt-studio-pp-cli sync` first", videoID))
			}

			// Early window: days 0-3 from publish_at. Late window: day 27+.
			// Because rows are now anchored to publish_at, this is honest.
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

// parseVideoPublishedAt parses the published_at field stored in yt_videos.
// The YouTube Data API returns RFC3339 timestamps (e.g.
// "2026-05-14T12:34:56Z"); the local store preserves them verbatim. We
// accept a date-only "YYYY-MM-DD" form as a fallback so manual seeds
// from the design spec keep working.
func parseVideoPublishedAt(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format")
}
