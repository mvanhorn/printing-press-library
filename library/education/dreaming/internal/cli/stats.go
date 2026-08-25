// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// stats: offline analytics over the synced store — totals, rolling averages,
// streaks, best day, and per-guide / per-tag breakdowns. Absorbs the analytics
// of ds-insights-web and HarryPeach's stats modules. Works offline.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show input-hours analytics: totals, rolling averages, streak, and best day",
		Long:        "Offline analytics over your synced data: total cumulative hours, daily goal,\ncurrent/longest streak, best day, and 7/30-day rolling averages.",
		Example:     "  dreaming-pp-cli stats\n  dreaming-pp-cli stats --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			u, ok, err := ensureUser(cmd.Context(), flags, db)
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no user stats cached — run 'dreaming-pp-cli sync' first"))
			}
			series, _ := loadDailyInput(cmd.Context(), db)
			cur, longest := streaks(series)
			var bestDay daySeconds
			for _, d := range series {
				if d.Seconds > bestDay.Seconds {
					bestDay = d
				}
			}
			avg7 := hoursFromSeconds(int64(recentAverageSeconds(series, 7))) * 60
			avg30 := hoursFromSeconds(int64(recentAverageSeconds(series, 30))) * 60

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"total_hours":         round1(u.TotalHours()),
					"on_platform_hours":   round1(hoursFromSeconds(u.WatchTime)),
					"external_hours":      round1(hoursFromSeconds(u.ExternalSeconds)),
					"level":               levelForHours(u.TotalHours()),
					"daily_goal_minutes":  u.DailyGoalSeconds / 60,
					"current_streak_days": cur,
					"longest_streak_days": longest,
					"best_day":            bestDay.Date,
					"best_day_minutes":    bestDay.Seconds / 60,
					"avg_min_per_day_7d":  round1(avg7),
					"avg_min_per_day_30d": round1(avg30),
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total input:   %.1f h  (L%d)\n", u.TotalHours(), levelForHours(u.TotalHours()))
			fmt.Fprintf(cmd.OutOrStdout(), "  on-platform: %.1f h   external: %.1f h\n", hoursFromSeconds(u.WatchTime), hoursFromSeconds(u.ExternalSeconds))
			fmt.Fprintf(cmd.OutOrStdout(), "Daily goal:    %d min\n", u.DailyGoalSeconds/60)
			fmt.Fprintf(cmd.OutOrStdout(), "Streak:        %d days (longest %d)\n", cur, longest)
			if bestDay.Date != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Best day:      %s (%d min)\n", bestDay.Date, bestDay.Seconds/60)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Avg/day:       %.0f min (7d)   %.0f min (30d)\n", avg7, avg30)
			return nil
		},
	}
	cmd.AddCommand(newStatsBreakdownCmd(flags, "by-guide", "guide", "Hours and videos watched per guide/teacher"))
	cmd.AddCommand(newStatsBreakdownCmd(flags, "by-tag", "topic", "Hours and videos watched per topic/tag"))
	return cmd
}

// newStatsBreakdownCmd builds a watched-videos breakdown grouped by a videos
// column (guide or topic), joined through the watched playlist.
func newStatsBreakdownCmd(flags *rootFlags, use, column, short string) *cobra.Command {
	example := "  dreaming-pp-cli stats " + use + "\n  dreaming-pp-cli stats " + use + " --agent"
	return &cobra.Command{
		Use:         use,
		Short:       short,
		Example:     example,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			// column is a fixed literal (guide|topic), never user input.
			q := fmt.Sprintf(`SELECT COALESCE(NULLIF(v.%s,''),'(unknown)') AS k, COUNT(*), COALESCE(SUM(v.duration),0)
				FROM playlist p JOIN videos v ON v.id = p.video_id GROUP BY k ORDER BY 2 DESC`, column)
			rows, err := db.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return err
			}
			defer rows.Close()
			type grp struct {
				Key     string `json:"key"`
				Videos  int    `json:"videos"`
				Minutes int    `json:"minutes"`
			}
			var out []grp
			for rows.Next() {
				var g grp
				var secs int
				if err := rows.Scan(&g.Key, &g.Videos, &secs); err != nil {
					return err
				}
				g.Minutes = secs / 60
				out = append(out, g)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Videos > out[j].Videos })
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No watched videos in the local store yet. Run 'sync' after watching some videos.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "%s\tVIDEOS\tMINUTES\n", strings.ToUpper(column))
			for _, g := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\n", truncate(g.Key, 30), g.Videos, g.Minutes)
			}
			return tw.Flush()
		},
	}
}
