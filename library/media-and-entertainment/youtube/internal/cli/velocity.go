// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: between-snapshot video velocity. Implemented body — regeneration preserves this file.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

type velocityRow struct {
	VideoID      string  `json:"videoId"`
	ChannelID    string  `json:"channelId"`
	Title        string  `json:"title"`
	PublishedAt  string  `json:"publishedAt"`
	FromAt       string  `json:"windowFrom"`
	ToAt         string  `json:"windowTo"`
	ViewsGained  int64   `json:"viewsGained"`
	ViewsPerDay  float64 `json:"views_per_day"`
	CurrentViews int64   `json:"currentViews"`
}

type velocityResult struct {
	Items     []velocityRow `json:"items"`
	Window    string        `json:"window"`
	Evaluated int           `json:"videosEvaluated"`
	Skipped   int           `json:"videosSkippedSingleSnapshot"`
	Note      string        `json:"note,omitempty"`
}

func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath, channel string
	var days, limit int
	cmd := &cobra.Command{
		Use:     "velocity",
		Short:   "See which tracked videos are gaining views fastest right now, computed from real between-snapshot deltas.",
		Long:    "Use this command to see which videos are gaining views fastest right now.\nDo NOT use it for lifetime channel deltas; use 'growth' instead.",
		Example: "  youtube-pp-cli velocity --json\n  youtube-pp-cli velocity --days 7 --channel @mkbhd --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--days=30;--limit=5",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "velocity report")
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
			cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
			chFilter := ""
			if channel != "" {
				chFilter, err = resolveLocalChannelID(ctx, db, channel)
				if err != nil {
					return notFoundErr(err)
				}
			}
			q := `
				WITH windowed AS (
					SELECT video_id, captured_at, view_count,
					       ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY captured_at ASC)  AS rn_asc,
					       ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY captured_at DESC) AS rn_desc
					FROM yt_video_snapshots
					WHERE captured_at >= ?
				),
				firsts AS (SELECT video_id, captured_at AS from_at, view_count AS from_views FROM windowed WHERE rn_asc = 1),
				lasts  AS (SELECT video_id, captured_at AS to_at,   view_count AS to_views   FROM windowed WHERE rn_desc = 1)
				SELECT v.video_id, v.channel_id, COALESCE(v.title,''), COALESCE(v.published_at,''),
				       f.from_at, l.to_at, f.from_views, l.to_views
				FROM firsts f
				JOIN lasts l ON l.video_id = f.video_id AND l.to_at > f.from_at
				JOIN yt_videos v ON v.video_id = f.video_id`
			argsq := []any{cutoff}
			if chFilter != "" {
				q += ` WHERE v.channel_id = ?`
				argsq = append(argsq, chFilter)
			}
			rows, err := db.DB().QueryContext(ctx, q, argsq...)
			if err != nil {
				return apiErr(err)
			}
			type raw struct {
				id, ch, title, pub, fromAt, toAt string
				fromViews, toViews               int64
			}
			raws := make([]raw, 0)
			for rows.Next() {
				var r raw
				var title, pub sql.NullString
				if err := rows.Scan(&r.id, &r.ch, &title, &pub, &r.fromAt, &r.toAt, &r.fromViews, &r.toViews); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				r.title, r.pub = title.String, pub.String
				raws = append(raws, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			_ = rows.Close()

			// Count single-snapshot videos honestly (they cannot produce a
			// delta), scoped to the same channel filter as the items.
			var singles int
			singleQ := `SELECT COUNT(*) FROM (
				SELECT s.video_id FROM yt_video_snapshots s`
			sargs := []any{}
			if chFilter != "" {
				singleQ += ` JOIN yt_videos v ON v.video_id = s.video_id AND v.channel_id = ?`
				sargs = append(sargs, chFilter)
			}
			singleQ += ` WHERE s.captured_at >= ? GROUP BY s.video_id HAVING COUNT(*) = 1)`
			sargs = append(sargs, cutoff)
			srow := db.DB().QueryRowContext(ctx, singleQ, sargs...)
			if err := srow.Scan(&singles); err != nil {
				singles = 0
			}

			items := make([]velocityRow, 0, len(raws))
			for _, r := range raws {
				fromT, e1 := time.Parse(time.RFC3339, r.fromAt)
				toT, e2 := time.Parse(time.RFC3339, r.toAt)
				if e1 != nil || e2 != nil || !toT.After(fromT) {
					continue
				}
				daysSpan := toT.Sub(fromT).Hours() / 24
				if daysSpan <= 0 {
					continue
				}
				gained := r.toViews - r.fromViews
				items = append(items, velocityRow{
					VideoID: r.id, ChannelID: r.ch, Title: r.title, PublishedAt: r.pub,
					FromAt: r.fromAt, ToAt: r.toAt, ViewsGained: gained,
					ViewsPerDay: float64(gained) / daysSpan, CurrentViews: r.toViews,
				})
			}
			sortVelocityDesc(items)
			evaluated := len(items)
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			res := velocityResult{Items: items, Window: fmt.Sprintf("last %d days", days), Evaluated: evaluated, Skipped: singles}
			if evaluated == 0 {
				res.Note = fmt.Sprintf("no videos with 2+ snapshots in the window (databank %s); run 'monitor' now and again later — deltas appear from the second run onward", path)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if evaluated == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%8.0f views/day  %s  %s\n", it.ViewsPerDay, it.VideoID, it.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().StringVar(&channel, "channel", "", "Restrict to one channel (@handle or channel id)")
	cmd.Flags().IntVar(&days, "days", 30, "Snapshot window in days")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return (0 = all)")
	return cmd
}

func sortVelocityDesc(items []velocityRow) {
	sort.Slice(items, func(i, j int) bool { return items[i].ViewsPerDay > items[j].ViewsPerDay })
}

// resolveLocalChannelID maps @handle/UC id to a channel id using only the
// local watchlist; bare UC… ids pass through. Backfilled-but-unwatched
// channels must be addressed by their UC… id (matching the error hint).
func resolveLocalChannelID(ctx context.Context, db *store.Store, input string) (string, error) {
	in := input
	var id string
	row := db.DB().QueryRowContext(ctx, `SELECT channel_id FROM yt_watchlist
		WHERE channel_id = ? OR handle = ? COLLATE NOCASE OR handle = ? COLLATE NOCASE LIMIT 1`, in, in, "@"+in)
	if err := row.Scan(&id); err == nil && id != "" {
		return id, nil
	}
	if len(in) == 24 && in[:2] == "UC" {
		return in, nil
	}
	return "", fmt.Errorf("channel %q not found locally; use its UC… id, or add it with 'watch add' / seed it with 'backfill'", input)
}
