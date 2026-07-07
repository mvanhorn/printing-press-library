// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type rivalRow struct {
	OwnerSlug           string  `json:"owner_slug"`
	Username            string  `json:"username"`
	StartFollowers      int64   `json:"start_followers"`
	EndFollowers        int64   `json:"end_followers"`
	FollowerChange      int64   `json:"follower_change"`
	RecentAvgEngagement float64 `json:"recent_avg_engagement"`
}

func newNovelRivalsCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceFlag, accountFlag string

	cmd := &cobra.Command{
		Use:   "rivals",
		Short: "Track rival public accounts' follower and engagement growth across syncs, benchmarked against your brands.",
		Long: `For each tracked rival, compare its earliest and latest follower-count
snapshots within --since and report the change plus the most recent average
engagement (likes+comments over recent media).

Reads the local store. Track rivals with 'brands track-rival <slug>
<username>', then run 'instagram-pp-cli pull' to collect snapshots.`,
		Example:     "  instagram-pp-cli rivals --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare rivals from local store")
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
			query := `
				SELECT owner_slug, username,
				       MIN(captured_at) AS first_at,
				       MAX(captured_at) AS last_at
				FROM ig_competitor_snapshots
				WHERE captured_at >= ?`
			args2 := []any{cutoff}
			if accountFlag != "" {
				query += ` AND owner_slug = ?`
				args2 = append(args2, slugify(accountFlag))
			}
			query += ` GROUP BY owner_slug, username ORDER BY owner_slug, username`

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]rivalRow, 0)
			for rows.Next() {
				var owner, username, firstAt, lastAt string
				if err := rows.Scan(&owner, &username, &firstAt, &lastAt); err != nil {
					return apiErr(err)
				}
				start := rivalFollowersAt(cmd, db.DB(), owner, username, firstAt)
				end, recentEng := rivalLatest(cmd, db.DB(), owner, username, lastAt)
				out = append(out, rivalRow{
					OwnerSlug:           owner,
					Username:            username,
					StartFollowers:      start,
					EndFollowers:        end,
					FollowerChange:      end - start,
					RecentAvgEngagement: recentEng,
				})
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				env := map[string]any{"rivals": out, "count": len(out)}
				if len(out) == 0 {
					env["note"] = "no rival snapshots in window; run 'brands track-rival' then 'instagram-pp-cli pull'"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No rival snapshots in window. Run 'brands track-rival <slug> <username>' then 'instagram-pp-cli pull'.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "OWNER\tRIVAL\tSTART\tEND\tCHANGE\tRECENT_AVG_ENG")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t@%s\t%d\t%d\t%+d\t%.1f\n", r.OwnerSlug, r.Username, r.StartFollowers, r.EndFollowers, r.FollowerChange, r.RecentAvgEngagement)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&sinceFlag, "since", "30d", "Comparison window (e.g. 30d, 8w, 24h)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit to rivals of a single brand slug")
	return cmd
}

func rivalFollowersAt(cmd *cobra.Command, db *sql.DB, owner, username, at string) int64 {
	var v sql.NullInt64
	_ = db.QueryRowContext(cmd.Context(),
		`SELECT followers_count FROM ig_competitor_snapshots WHERE owner_slug = ? AND username = ? AND captured_at = ? ORDER BY id LIMIT 1`,
		owner, username, at).Scan(&v)
	if v.Valid {
		return v.Int64
	}
	return 0
}

func rivalLatest(cmd *cobra.Command, db *sql.DB, owner, username, at string) (int64, float64) {
	var f sql.NullInt64
	var eng sql.NullFloat64
	_ = db.QueryRowContext(cmd.Context(),
		`SELECT followers_count, recent_avg_engagement FROM ig_competitor_snapshots WHERE owner_slug = ? AND username = ? AND captured_at = ? ORDER BY id DESC LIMIT 1`,
		owner, username, at).Scan(&f, &eng)
	return f.Int64, eng.Float64
}
