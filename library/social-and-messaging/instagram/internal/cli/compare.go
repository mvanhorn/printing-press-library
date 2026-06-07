// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type compareRow struct {
	Slug              string  `json:"slug"`
	FollowersCount    int64   `json:"followers_count"`
	Reach             int64   `json:"reach"`
	TotalInteractions int64   `json:"total_interactions"`
	EngagementRate    float64 `json:"engagement_rate"`
	Views             int64   `json:"views"`
	CapturedAt        string  `json:"captured_at"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceFlag, metricFlag string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Rank all your brand accounts side by side by reach, interactions, and engagement rate over a window.",
		Long: `Rank every registered brand by its latest account snapshot within --since.

engagement_rate = total_interactions / reach (reach is the denominator; a
snapshot with zero reach yields engagement_rate 0). Rank by --metric (default er):
  reach          total accounts reached
  interactions   total_interactions
  er             engagement_rate (default; also accepts "engagement")

Reads the local store. Run 'instagram-pp-cli pull' first to populate it.`,
		Example:     "  instagram-pp-cli compare --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare brands from local store")
				return nil
			}
			window, err := parseLooseDuration(sinceFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", sinceFlag, err))
			}
			metric := strings.ToLower(strings.TrimSpace(metricFlag))
			switch metric {
			case "", "engagement", "er", "reach", "interactions":
			default:
				return usageErr(fmt.Errorf("invalid --metric %q: use reach, interactions, or er", metricFlag))
			}

			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().Add(-window).Format(time.RFC3339)
			// Latest snapshot per brand within the window.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT s.slug,
				       COALESCE(s.followers_count,0),
				       COALESCE(s.reach,0),
				       COALESCE(s.total_interactions,0),
				       COALESCE(s.views,0),
				       COALESCE(s.captured_at,'')
				FROM ig_account_snapshots s
				JOIN (
					SELECT slug, MAX(id) AS mx
					FROM ig_account_snapshots
					WHERE captured_at >= ?
					GROUP BY slug
				) latest ON latest.slug = s.slug AND latest.mx = s.id`, cutoff)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]compareRow, 0)
			for rows.Next() {
				var r compareRow
				if err := rows.Scan(&r.Slug, &r.FollowersCount, &r.Reach, &r.TotalInteractions, &r.Views, &r.CapturedAt); err != nil {
					return apiErr(err)
				}
				if r.Reach > 0 {
					r.EngagementRate = float64(r.TotalInteractions) / float64(r.Reach)
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			switch metric {
			case "interactions":
				sort.SliceStable(out, func(i, j int) bool { return out[i].TotalInteractions > out[j].TotalInteractions })
			case "er", "engagement":
				sort.SliceStable(out, func(i, j int) bool { return out[i].EngagementRate > out[j].EngagementRate })
			default:
				sort.SliceStable(out, func(i, j int) bool { return out[i].Reach > out[j].Reach })
			}

			if flags.asJSON {
				env := map[string]any{"brands": out, "count": len(out)}
				if len(out) == 0 {
					env["note"] = "no account snapshots in window; run 'instagram-pp-cli pull' first"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No account snapshots in window. Run 'instagram-pp-cli pull' first.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SLUG\tFOLLOWERS\tREACH\tINTERACTIONS\tER\tVIEWS\tCAPTURED_AT")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.4f\t%d\t%s\n", r.Slug, r.FollowersCount, r.Reach, r.TotalInteractions, r.EngagementRate, r.Views, r.CapturedAt)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&sinceFlag, "since", "30d", "Only consider snapshots newer than this (e.g. 7d, 8w, 24h)")
	cmd.Flags().StringVar(&metricFlag, "metric", "er", "Ranking metric: reach, interactions, or er (default)")
	return cmd
}
