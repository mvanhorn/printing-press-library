package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newVsWatchlistCmd(flags *rootFlags) *cobra.Command {
	var (
		metric string
		period string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "vs-watchlist",
		Short: "Compare own channel against the watchlist on selected metrics",
		Long: strings.TrimSpace(`
Reads aggregated metrics from the local store and produces a normalized
comparison between your own channel and each watchlist competitor.

Supported metrics (comma-separated):
  ctr             — mean CTR across all videos in the period
  retention       — mean avg_view_pct across all videos
  upload-cadence  — uploads per week
`),
		Example:     "  yt-studio-pp-cli vs-watchlist --metric ctr,retention,upload-cadence --period 28d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if metric == "" {
				return usageErr(errors.New("--metric is required (comma-separated: ctr,retention,upload-cadence)"))
			}
			metrics := splitCSV(metric)
			for _, m := range metrics {
				switch m {
				case "ctr", "retention", "upload-cadence":
				default:
					return usageErr(fmt.Errorf("unknown metric %q", m))
				}
			}
			daysWindow := parsePeriodDays(period)
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			ownIDs, err := ytstore.ListOwnChannelIDs(ctx, db)
			if err != nil {
				return err
			}
			if len(ownIDs) == 0 {
				return notFoundErr(errors.New("no 'own' channel synced yet; run `yt-studio-pp-cli sync --full`"))
			}
			watchlist, err := ytstore.ListWatchlist(ctx, db)
			if err != nil {
				return err
			}

			result := map[string]any{
				"period_days": daysWindow,
				"metrics":     metrics,
				"own":         metricsForChannels(ctx, db, ownIDs, metrics, daysWindow),
				"competitors": []map[string]any{},
			}
			competitors := []map[string]any{}
			for _, w := range watchlist {
				row := map[string]any{
					"channel_id": w.ChannelID,
					"title":      w.Title,
					"metrics":    metricsForChannels(ctx, db, []string{w.ChannelID}, metrics, daysWindow),
				}
				competitors = append(competitors, row)
			}
			result["competitors"] = competitors

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "vs-watchlist (period: %d days, metrics: %s)\n", daysWindow, strings.Join(metrics, ","))
			fmt.Fprintf(w, "  OWN: %+v\n", result["own"])
			for _, c := range competitors {
				fmt.Fprintf(w, "  %s (%s): %+v\n", c["channel_id"], c["title"], c["metrics"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "", "Comma-separated metrics: ctr,retention,upload-cadence")
	cmd.Flags().StringVar(&period, "period", "28d", "Window (e.g. 7d, 28d, 90d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func metricsForChannels(ctx context.Context, db *sql.DB, channelIDs []string, metrics []string, days int) map[string]any {
	out := map[string]any{}
	if len(channelIDs) == 0 {
		return out
	}
	placeholders := strings.Repeat("?,", len(channelIDs))
	placeholders = strings.TrimSuffix(placeholders, ",")
	channelArgs := make([]any, len(channelIDs))
	for i, id := range channelIDs {
		channelArgs[i] = id
	}

	for _, m := range metrics {
		switch m {
		case "ctr":
			args := append([]any{}, channelArgs...)
			args = append(args, fmt.Sprintf("-%d days", days))
			row := db.QueryRowContext(ctx, `
				SELECT AVG(m.ctr) FROM yt_video_metrics_daily m
				JOIN yt_videos v ON v.video_id = m.video_id
				WHERE v.channel_id IN (`+placeholders+`)
				AND m.day >= date('now', ?)
				AND m.ctr > 0
			`, args...)
			var avg sql.NullFloat64
			_ = row.Scan(&avg)
			out["ctr"] = avg.Float64
		case "retention":
			args := append([]any{}, channelArgs...)
			args = append(args, fmt.Sprintf("-%d days", days))
			row := db.QueryRowContext(ctx, `
				SELECT AVG(m.avg_view_pct) FROM yt_video_metrics_daily m
				JOIN yt_videos v ON v.video_id = m.video_id
				WHERE v.channel_id IN (`+placeholders+`)
				AND m.day >= date('now', ?)
				AND m.avg_view_pct > 0
			`, args...)
			var avg sql.NullFloat64
			_ = row.Scan(&avg)
			out["retention"] = avg.Float64
		case "upload-cadence":
			args := append([]any{}, channelArgs...)
			args = append(args, fmt.Sprintf("-%d days", days))
			row := db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM yt_videos
				WHERE channel_id IN (`+placeholders+`)
				AND published_at >= datetime('now', ?)
			`, args...)
			var count int
			_ = row.Scan(&count)
			perWeek := 0.0
			if days > 0 {
				perWeek = float64(count) / (float64(days) / 7.0)
			}
			out["upload_cadence_per_week"] = perWeek
			out["uploads_in_window"] = count
		}
	}
	return out
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parsePeriodDays(p string) int {
	p = strings.TrimSpace(strings.ToLower(p))
	if p == "" {
		return 28
	}
	if strings.HasSuffix(p, "d") {
		var d int
		_, err := fmt.Sscanf(p, "%dd", &d)
		if err == nil {
			return d
		}
	}
	if strings.HasSuffix(p, "w") {
		var w int
		_, err := fmt.Sscanf(p, "%dw", &w)
		if err == nil {
			return w * 7
		}
	}
	return 28
}
