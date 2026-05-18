package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

type decayRow struct {
	VideoID       string  `json:"video_id"`
	Title         string  `json:"title"`
	Views         int64   `json:"views"`
	DaysSincePub  int     `json:"days_since_published"`
	ViewsPerDay   float64 `json:"views_per_day"`
	Baseline      float64 `json:"baseline"`
	BaselineLabel string  `json:"baseline_label"`
	DropRatio     float64 `json:"drop_ratio"`
	PublishedAt   string  `json:"published_at,omitempty"`
}

// newDecayCmd flags under-performing videos for a channel.
//
// --metric velocity (default) compares views-per-day-since-published against a
// per-day baseline, so a 3-month-old video isn't unfairly penalized against a
// 3-week-old one. --metric views falls back to the original raw-views
// comparison and is kept only for backward-compatible scripts; new callers
// should prefer velocity.
//
// --days restricts both the candidate set AND the baseline computation to
// videos published within the trailing N days. Set 0 to consider all cached
// videos (default).
//
// Statistical caveat: with fewer than 30 videos in the comparison set the
// baseline is noisy and individual drop ratios should be treated as
// directional, not as alerts. The command emits a warning to stderr when this
// floor isn't met.
func newDecayCmd(flags *rootFlags) *cobra.Command {
	var channel, dbPath, metric string
	var threshold float64
	var limit, days int
	cmd := &cobra.Command{
		Use:   "decay",
		Short: "Flag underperforming videos by per-day velocity (or raw views) vs the channel baseline",
		Example: `  youtube-creator-analytics-pp-cli decay --threshold 0.3 --json
  youtube-creator-analytics-pp-cli decay --metric velocity --days 90 --json
  youtube-creator-analytics-pp-cli decay --metric views --threshold 0.5 --limit 10`,
		Long: `Flag videos whose performance has decayed below the channel's rolling baseline.

Metric:
  velocity (default) — views_per_day = views / days_since_published.
                       Normalizes for video age. A 3-month-old video isn't
                       penalized against a 3-week-old one.
  views              — raw view count vs raw mean. Time-blind. Legacy
                       behavior, prone to false positives.

--days N restricts both the candidate set AND the baseline to videos
published within the trailing N days (0 = no restriction).

Statistical floor: with fewer than 30 videos the baseline is noisy. Treat
results as directional, not as alerts.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if metric != "velocity" && metric != "views" {
				return fmt.Errorf("--metric must be 'velocity' or 'views', got %q", metric)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			ch, err := resolveChannel(db, channel)
			if err != nil {
				return err
			}
			videos, err := loadVideos(db, ch, 0)
			if err != nil {
				return err
			}
			if len(videos) == 0 {
				return writeNoop(flags, "no_videos", "no cached videos for this channel; run `youtube-creator-analytics-pp-cli sync` first")
			}

			now := time.Now()
			// Apply --days window.
			if days > 0 {
				cutoff := now.AddDate(0, 0, -days)
				filtered := videos[:0]
				for _, v := range videos {
					if !v.PublishedAt.IsZero() && v.PublishedAt.After(cutoff) {
						filtered = append(filtered, v)
					}
				}
				videos = filtered
				if len(videos) == 0 {
					return writeNoop(flags, "no_videos_in_window", fmt.Sprintf("no cached videos published in the last %d days", days))
				}
			}

			// Statistical-noise warning.
			if len(videos) < 30 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: only %d videos in comparison set (<30); results are statistically noisy and should be treated as directional, not as alerts\n",
					len(videos))
			}

			// Compute baseline.
			var baseline float64
			var baselineLabel string
			if metric == "velocity" {
				var totalVPD float64
				var n int
				for _, v := range videos {
					vpd := viewsPerDay(v.ViewCount, v.PublishedAt, now)
					if vpd <= 0 {
						continue
					}
					totalVPD += vpd
					n++
				}
				if n == 0 {
					return writeNoop(flags, "no_baseline", "could not compute baseline velocity (no videos with positive views_per_day)")
				}
				baseline = totalVPD / float64(n)
				baselineLabel = "views_per_day"
			} else {
				var totalViews int64
				for _, v := range videos {
					totalViews += v.ViewCount
				}
				baseline = float64(totalViews) / float64(len(videos))
				baselineLabel = "views"
			}

			out := []decayRow{}
			for _, v := range videos {
				if baseline == 0 {
					continue
				}
				daysSince := daysSincePublished(v.PublishedAt, now)
				vpd := viewsPerDay(v.ViewCount, v.PublishedAt, now)
				var current float64
				if metric == "velocity" {
					current = vpd
				} else {
					current = float64(v.ViewCount)
				}
				drop := 1.0 - current/baseline
				if drop >= threshold {
					out = append(out, decayRow{
						VideoID:       v.ID,
						Title:         v.Title,
						Views:         v.ViewCount,
						DaysSincePub:  daysSince,
						ViewsPerDay:   vpd,
						Baseline:      baseline,
						BaselineLabel: baselineLabel,
						DropRatio:     drop,
						PublishedAt:   v.PublishedAt.Format("2006-01-02"),
					})
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].DropRatio > out[j].DropRatio })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel ID (defaults to most recently synced)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metric, "metric", "velocity", "Comparison metric: 'velocity' (views/day, normalizes for video age) or 'views' (raw, legacy)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.3, "Min drop ratio vs baseline (0.0-1.0)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().IntVar(&days, "days", 0, "Restrict baseline + candidates to videos published in the trailing N days (0 = all)")
	return cmd
}

// daysSincePublished returns the integer number of days between publishedAt
// and now, clamped to a minimum of 1 to avoid divide-by-zero when computing
// per-day rates for videos published in the last 24 hours.
func daysSincePublished(publishedAt, now time.Time) int {
	if publishedAt.IsZero() {
		return 1
	}
	d := int(now.Sub(publishedAt).Hours() / 24)
	if d < 1 {
		return 1
	}
	return d
}

// viewsPerDay normalizes raw views by video age so that videos of different
// ages can be compared on the same scale.
func viewsPerDay(views int64, publishedAt, now time.Time) float64 {
	return float64(views) / float64(daysSincePublished(publishedAt, now))
}
