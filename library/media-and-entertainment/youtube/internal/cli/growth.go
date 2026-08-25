// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: channel growth between snapshots. Implemented body — regeneration preserves this file.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

type growthRow struct {
	ChannelID   string  `json:"channelId"`
	Title       string  `json:"title"`
	FromAt      string  `json:"windowFrom"`
	ToAt        string  `json:"windowTo"`
	SubsDelta   int64   `json:"subscribersDelta"`
	ViewsDelta  int64   `json:"viewsDelta"`
	VideosDelta int64   `json:"uploadsDelta"`
	SubsPerDay  float64 `json:"subs_per_day"`
	ViewsPerDay float64 `json:"views_per_day"`
	CurrentSubs int64   `json:"currentSubscribers"`
	Snapshots   int     `json:"snapshotsInWindow"`
	Note        string  `json:"note,omitempty"`
}

type growthResult struct {
	Items  []growthRow `json:"items"`
	Window string      `json:"window"`
	Note   string      `json:"note,omitempty"`
}

func newNovelGrowthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	cmd := &cobra.Command{
		Use:     "growth [@handle|channelId]",
		Short:   "Channel-level subscriber, view, and upload-count deltas between dated local snapshots.",
		Long:    "Use this command for channel-level change over time between snapshots.\nDo NOT use it for per-video movement; use 'velocity' instead.",
		Example: "  youtube-pp-cli growth @mkbhd --json\n  youtube-pp-cli growth --days 7 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--days=90",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "growth report")
			}
			if days <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be > 0"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, path, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			chFilter := ""
			if len(args) > 0 {
				chFilter, err = resolveLocalChannelID(ctx, db, args[0])
				if err != nil {
					return notFoundErr(err)
				}
			}
			cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
			items, err := growthRows(ctx, db, cutoff, chFilter)
			if err != nil {
				return apiErr(err)
			}
			res := growthResult{Items: items, Window: fmt.Sprintf("last %d days", days)}
			if len(items) == 0 {
				res.Note = fmt.Sprintf("no channel snapshots in the window (databank %s); run 'monitor' or 'backfill' now and again later — growth needs at least 2 dated snapshots", path)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  subs %+d (%.1f/day)  views %+d  uploads %+d  %s\n",
					it.ChannelID, it.Title, it.SubsDelta, it.SubsPerDay, it.ViewsDelta, it.VideosDelta, it.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&days, "days", 90, "Snapshot window in days")
	return cmd
}

func growthRows(ctx context.Context, db *store.Store, cutoff, chFilter string) ([]growthRow, error) {
	q := `
		WITH windowed AS (
			SELECT channel_id, captured_at, subscriber_count, view_count, video_count,
			       ROW_NUMBER() OVER (PARTITION BY channel_id ORDER BY captured_at ASC)  AS rn_asc,
			       ROW_NUMBER() OVER (PARTITION BY channel_id ORDER BY captured_at DESC) AS rn_desc,
			       COUNT(*)     OVER (PARTITION BY channel_id) AS snaps
			FROM yt_channel_snapshots
			WHERE captured_at >= ?
		),
		firsts AS (SELECT channel_id, captured_at AS from_at, subscriber_count AS s0, view_count AS v0, video_count AS n0, snaps FROM windowed WHERE rn_asc = 1),
		lasts  AS (SELECT channel_id, captured_at AS to_at,   subscriber_count AS s1, view_count AS v1, video_count AS n1 FROM windowed WHERE rn_desc = 1)
		SELECT f.channel_id, COALESCE(w.title,''), f.from_at, l.to_at, f.s0, l.s1, f.v0, l.v1, f.n0, l.n1, f.snaps
		FROM firsts f
		JOIN lasts l ON l.channel_id = f.channel_id
		LEFT JOIN yt_watchlist w ON w.channel_id = f.channel_id`
	argsq := []any{cutoff}
	if chFilter != "" {
		q += ` WHERE f.channel_id = ?`
		argsq = append(argsq, chFilter)
	}
	rows, err := db.DB().QueryContext(ctx, q, argsq...)
	if err != nil {
		return nil, err
	}
	items := make([]growthRow, 0)
	for rows.Next() {
		var r growthRow
		var title sql.NullString
		var s0, s1, v0, v1, n0, n1 int64
		var snaps int
		if err := rows.Scan(&r.ChannelID, &title, &r.FromAt, &r.ToAt, &s0, &s1, &v0, &v1, &n0, &n1, &snaps); err != nil {
			_ = rows.Close()
			return items, err
		}
		r.Title = title.String
		r.Snapshots = snaps
		if snaps < 2 || r.FromAt == r.ToAt {
			r.Note = "need at least 2 snapshots — rerun monitor/backfill later"
			r.CurrentSubs = s1
			items = append(items, r)
			continue
		}
		fromT, e1 := time.Parse(time.RFC3339, r.FromAt)
		toT, e2 := time.Parse(time.RFC3339, r.ToAt)
		r.SubsDelta, r.ViewsDelta, r.VideosDelta = s1-s0, v1-v0, n1-n0
		r.CurrentSubs = s1
		if e1 == nil && e2 == nil && toT.After(fromT) {
			span := toT.Sub(fromT).Hours() / 24
			if span > 0 {
				r.SubsPerDay = float64(r.SubsDelta) / span
				r.ViewsPerDay = float64(r.ViewsDelta) / span
			}
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return items, err
	}
	_ = rows.Close()
	return items, nil
}
