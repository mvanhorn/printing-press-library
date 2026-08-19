// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: watchlist monitor. Implemented body — regeneration preserves this file.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

type monitorChannelReport struct {
	ChannelID    string `json:"channelId"`
	Title        string `json:"title"`
	NewVideos    int    `json:"newVideos"`
	Resnapshoted int    `json:"videosResnapshoted"`
	Comments     int    `json:"commentsSynced"`
	Error        string `json:"error,omitempty"`
}

type monitorResult struct {
	RunAt         string                 `json:"runAt"`
	Channels      int                    `json:"channels"`
	NewVideos     int                    `json:"newVideos"`
	Snapshots     int                    `json:"videoSnapshots"`
	Comments      int                    `json:"commentsSynced"`
	QuotaUnitsEst int                    `json:"quotaUnitsEst"`
	PerChannel    []monitorChannelReport `json:"perChannel"`
	RotatedKeys   []string               `json:"rotatedKeys,omitempty"`
	FetchFailures []string               `json:"fetch_failures,omitempty"`
	Hint          string                 `json:"hint,omitempty"`
}

func newNovelMonitorCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var recentN, maxNew, commentPages int
	var withComments, rotate bool
	cmd := &cobra.Command{
		Use:     "monitor",
		Short:   "Refresh every watched channel in one run: stats snapshot, new uploads, re-snapshot of recent video statistics.",
		Long:    "Use this command to refresh all watched channels' data in one run.\nDo NOT use it for a single ad-hoc channel pull; use 'backfill' instead.",
		Example: "  youtube-pp-cli monitor --json\n  youtube-pp-cli monitor --comments --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--recent=3",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "monitor watchlist")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			recent := recentN
			if cliutil.IsDogfoodEnv() {
				if recent > 3 {
					recent = 3
				}
			}
			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			entries, err := watchlistEntries(ctx, db)
			if err != nil {
				return apiErr(err)
			}
			if len(entries) == 0 {
				res := monitorResult{RunAt: nowUTC(), PerChannel: []monitorChannelReport{},
					Hint: "watchlist is empty; add competitors with 'watch add @handle'"}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add competitors with: youtube-pp-cli watch add @handle")
				return nil
			}
			if cliutil.IsDogfoodEnv() && len(entries) > 1 {
				entries = entries[:1]
			}
			g, err := newRotatingGetter(flags, rotate)
			if err != nil {
				return err
			}
			now := nowUTC()
			res := monitorResult{RunAt: now, Channels: len(entries), PerChannel: make([]monitorChannelReport, 0, len(entries)), FetchFailures: make([]string, 0)}
			quota := 0

			// One batched channels.list for the whole watchlist (1 unit per 50).
			chMap, err := fetchChannelsByIDs(ctx, g, channelIDs(entries))
			if err != nil {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			quota += (len(entries) + 49) / 50

			for _, e := range entries {
				rep := monitorChannelReport{ChannelID: e.ChannelID, Title: e.Title}
				ch, ok := chMap[e.ChannelID]
				if !ok {
					rep.Error = "channel missing from batched channels.list response (terminated?)"
					res.PerChannel = append(res.PerChannel, rep)
					res.FetchFailures = append(res.FetchFailures, e.ChannelID+": "+rep.Error)
					continue
				}
				if err := writeChannelSnapshot(ctx, db, ch, now); err != nil {
					return apiErr(err)
				}
				// New uploads since the last monitored video we know about.
				known, err := knownVideoIDs(ctx, db, e.ChannelID, maxNew)
				if err != nil {
					return apiErr(err)
				}
				walkCap := maxNew
				if walkCap <= 0 {
					walkCap = 50
				}
				ids, werr := walkUploadsPlaylist(ctx, g, ch.UploadsPlaylist, walkCap)
				quota += (len(ids) + 49) / 50
				if werr != nil {
					rep.Error = "uploads walk: " + werr.Error()
					res.FetchFailures = append(res.FetchFailures, e.ChannelID+": "+rep.Error)
				}
				newIDs := make([]string, 0)
				for _, id := range ids {
					if _, seen := known[id]; !seen {
						newIDs = append(newIDs, id)
					}
				}
				// Re-snapshot the most recent stored videos so deltas accumulate.
				recentIDs, err := recentStoredVideoIDs(ctx, db, e.ChannelID, recent)
				if err != nil {
					return apiErr(err)
				}
				fetchIDs := dedupeStrings(append(newIDs, recentIDs...))
				if len(fetchIDs) > 0 {
					rows, ferr := fetchVideoStats(ctx, g, fetchIDs)
					quota += (len(fetchIDs) + 49) / 50
					if ferr != nil && len(rows) == 0 {
						rep.Error = strings.TrimSpace(rep.Error + "; stats fetch: " + ferr.Error())
						res.FetchFailures = append(res.FetchFailures, e.ChannelID+": stats fetch failed")
						res.PerChannel = append(res.PerChannel, rep)
						continue
					}
					stored, serr := writeVideoRows(ctx, db, rows, now)
					if serr != nil {
						return apiErr(serr)
					}
					rep.Resnapshoted = stored
					rep.NewVideos = len(newIDs)
					res.NewVideos += len(newIDs)
					res.Snapshots += stored
				}
				// Optional comment sync for new uploads.
				if withComments && len(newIDs) > 0 {
					pages := commentPages
					if pages <= 0 {
						pages = 1
					}
					for _, vid := range newIDs {
						n, cerr := syncVideoComments(ctx, g, db, vid, e.ChannelID, pages)
						quota += pages
						if cerr != nil {
							res.FetchFailures = append(res.FetchFailures, vid+": comments: "+cerr.Error())
							continue
						}
						rep.Comments += n
						res.Comments += n
					}
				}
				if _, err := db.DB().ExecContext(ctx, `UPDATE yt_watchlist SET last_monitored_at = ? WHERE channel_id = ?`, now, e.ChannelID); err != nil {
					return apiErr(err)
				}
				res.PerChannel = append(res.PerChannel, rep)
			}
			res.QuotaUnitsEst = quota
			res.RotatedKeys = g.Rotated
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO yt_monitor_runs
				(started_at, finished_at, channels, new_videos, video_snapshots, comments_synced, quota_units_est)
				VALUES (?,?,?,?,?,?,?)`,
				now, nowUTC(), res.Channels, res.NewVideos, res.Snapshots, res.Comments, quota); err != nil {
				return apiErr(err)
			}
			if len(res.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d channels had fetch problems; totals cover the successful ones\n",
					len(res.FetchFailures), res.Channels)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Monitored %d channels: %d new videos, %d snapshots, %d comments (~%d quota units)\n",
				res.Channels, res.NewVideos, res.Snapshots, res.Comments, res.QuotaUnitsEst)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&recentN, "recent", 15, "Recent stored videos per channel to re-snapshot")
	cmd.Flags().IntVar(&maxNew, "max-new", 50, "Maximum new uploads to ingest per channel per run")
	cmd.Flags().BoolVar(&withComments, "comments", false, "Also sync comments for newly discovered uploads")
	cmd.Flags().IntVar(&commentPages, "comment-pages", 1, "commentThreads pages (of 100) per new video when --comments is set")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}

func channelIDs(entries []watchEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ChannelID)
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// fetchChannelsByIDs batches channels.list by id CSV (1 unit per 50).
func fetchChannelsByIDs(ctx context.Context, g *rotatingGetter, ids []string) (map[string]*resolvedChannel, error) {
	out := make(map[string]*resolvedChannel, len(ids))
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		data, err := g.Get(ctx, "/youtube/v3/channels", map[string]string{
			"part":       "snippet,contentDetails,statistics",
			"id":         strings.Join(ids[start:end], ","),
			"maxResults": "50",
		})
		if err != nil {
			return out, err
		}
		items, perr := parseChannelListItems(data)
		if perr != nil {
			return out, perr
		}
		for _, ch := range items {
			out[ch.ChannelID] = ch
		}
	}
	return out, nil
}

// knownIDOverfetch widens the known-id window beyond the new-upload cap so
// re-ordered or re-dated uploads inside the recent window are still
// recognized as already stored instead of re-ingested as "new".
const knownIDOverfetch = 4

// knownVideoIDs returns recent stored video ids for a channel — up to
// capN*knownIDOverfetch of them (see knownIDOverfetch).
func knownVideoIDs(ctx context.Context, db *store.Store, channelID string, capN int) (map[string]struct{}, error) {
	if capN <= 0 {
		capN = 200
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT video_id FROM yt_videos WHERE channel_id = ? ORDER BY published_at DESC LIMIT ?`, channelID, capN*knownIDOverfetch)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return out, err
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	_ = rows.Close()
	return out, nil
}

func recentStoredVideoIDs(ctx context.Context, db *store.Store, channelID string, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT video_id FROM yt_videos WHERE channel_id = ? ORDER BY published_at DESC LIMIT ?`, channelID, n)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return out, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	_ = rows.Close()
	return out, nil
}

// syncVideoComments pulls up to `pages` pages of 100 top-level comments for a
// video into yt_comments. Returns rows written. Parsing and the insert run
// through storeCommentThreadsPage — the single yt_comments write path.
func syncVideoComments(ctx context.Context, g *rotatingGetter, db *store.Store, videoID, channelID string, pages int) (int, error) {
	total := 0
	pageToken := ""
	for p := 0; p < pages; p++ {
		params := map[string]string{
			"part":       "snippet",
			"videoId":    videoID,
			"maxResults": "100",
			"order":      "relevance",
			"textFormat": "plainText",
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := g.Get(ctx, "/youtube/v3/commentThreads", params)
		if err != nil {
			return total, err
		}
		n, next, err := storeCommentThreadsPage(ctx, db, data, channelID)
		total += n
		if err != nil {
			return total, err
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return total, nil
}
