// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type growthRow struct {
	Slug           string  `json:"slug"`
	StartFollowers int64   `json:"start_followers"`
	EndFollowers   int64   `json:"end_followers"`
	AbsChange      int64   `json:"abs_change"`
	PctChange      float64 `json:"pct_change"`
	WeeksCovered   float64 `json:"weeks_covered"`
	Note           string  `json:"note,omitempty"`
}

func newNovelGrowthCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceFlag string

	cmd := &cobra.Command{
		Use:   "growth",
		Short: "Track follower-count growth over time per brand, week over week.",
		Long: `For each brand, compare the earliest and latest follower_count snapshots
within --since and report absolute and percentage change plus the weeks
covered. A brand with a single snapshot reports abs_change 0 with a note.

Reads the local store. Run 'instagram-pp-cli pull' on a schedule to build the
historical series this command needs.`,
		Example: `  # Follower growth over the default 8-week window
  instagram-pp-cli growth

  # Last 30 days, as JSON for piping into jq
  instagram-pp-cli growth --since 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute follower growth from local store")
				return nil
			}
			window, err := parseLooseDuration(sinceFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", sinceFlag, err))
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().Add(-window).Format(time.RFC3339)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT slug,
				       MIN(captured_at) AS first_at,
				       MAX(captured_at) AS last_at,
				       COUNT(*) AS n
				FROM ig_account_snapshots
				WHERE captured_at >= ?
				GROUP BY slug
				ORDER BY slug`, cutoff)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]growthRow, 0)
			for rows.Next() {
				var slug, firstAt, lastAt string
				var n int64
				if err := rows.Scan(&slug, &firstAt, &lastAt, &n); err != nil {
					return apiErr(err)
				}
				start := followersAt(cmd.Context(), db.DB(), slug, firstAt)
				end := followersAt(cmd.Context(), db.DB(), slug, lastAt)
				g := growthRow{
					Slug:           slug,
					StartFollowers: start,
					EndFollowers:   end,
					AbsChange:      end - start,
				}
				if start > 0 {
					g.PctChange = float64(end-start) / float64(start) * 100
				}
				if t0, ok0 := parseIGTime(firstAt); ok0 {
					if t1, ok1 := parseIGTime(lastAt); ok1 {
						g.WeeksCovered = t1.Sub(t0).Hours() / (24 * 7)
					}
				}
				if n < 2 {
					g.AbsChange = 0
					g.PctChange = 0
					g.Note = "only one snapshot in window; need at least two to measure growth"
				}
				out = append(out, g)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
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
			fmt.Fprintln(tw, "SLUG\tSTART\tEND\tABS_CHANGE\tPCT_CHANGE\tWEEKS")
			for _, g := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%+d\t%+.2f%%\t%.1f\n", g.Slug, g.StartFollowers, g.EndFollowers, g.AbsChange, g.PctChange, g.WeeksCovered)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&sinceFlag, "since", "8w", "Growth window (e.g. 8w, 30d, 24h)")
	return cmd
}

func followersAt(ctx context.Context, db *sql.DB, slug, capturedAt string) int64 {
	var v sql.NullInt64
	_ = db.QueryRowContext(ctx, `SELECT followers_count FROM ig_account_snapshots WHERE slug = ? AND captured_at = ? ORDER BY id LIMIT 1`, slug, capturedAt).Scan(&v)
	if v.Valid {
		return v.Int64
	}
	return 0
}
