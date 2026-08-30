// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: chained-filter breakout finder. Implemented body — regeneration preserves this file.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"

	"github.com/spf13/cobra"
)

type breakoutRow struct {
	VideoID     string  `json:"videoId"`
	Title       string  `json:"title"`
	ChannelID   string  `json:"channelId"`
	Channel     string  `json:"channelTitle"`
	PublishedAt string  `json:"publishedAt"`
	Views       int64   `json:"views"`
	Subs        int64   `json:"channelSubscribers"`
	ViewsPerSub float64 `json:"views_per_subscriber"`
	ViewsPerDay float64 `json:"views_per_day"`
	WatchURL    string  `json:"watchUrl"`
}

type breakoutsResult struct {
	Items          []breakoutRow `json:"items"`
	CellsSearched  int           `json:"searchCellsUsed"`
	SearchBudget   string        `json:"searchBudgetNote"`
	ScannedVideos  int           `json:"scanned_videos"`
	Window         string        `json:"window"`
	RotatedKeys    []string      `json:"rotatedKeys,omitempty"`
	FetchFailures  []string      `json:"fetch_failures,omitempty"`
	Note           string        `json:"note,omitempty"`
	StoredLocally  int           `json:"storedLocally"`
	MinSubsApplied int64         `json:"minSubsApplied"`
}

func newNovelBreakoutsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, region, duration, category string
	var days, perCell, limit int
	var minSubs int64
	var rotate bool
	cmd := &cobra.Command{
		Use:     "breakouts [terms...]",
		Short:   "Chain search filters into a matrix, join results to channel size, and rank fresh high-momentum videos.",
		Long:    "Use this command to discover fresh high-momentum videos across the market.\nDo NOT use it for tracked-competitor movement; use 'velocity' instead.",
		Example: "  youtube-pp-cli breakouts \"berlin history\" --days 14 --json\n  youtube-pp-cli breakouts \"ancient rome\" \"roman empire\" --days 7 --duration long --region DE",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "term=documentary;--days=14;--per-cell=5",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "breakout search matrix")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one search term is required"))
			}
			if days <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be > 0"))
			}
			terms := args
			cells := len(terms)
			if cliutil.IsDogfoodEnv() && cells > 1 {
				terms = terms[:1]
				cells = 1
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			g, err := newRotatingGetter(flags, rotate)
			if err != nil {
				return err
			}
			publishedAfter := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
			videoIDs := make([]string, 0)
			failures := make([]string, 0)
			for _, term := range terms {
				params := map[string]string{
					"part":           "id",
					"type":           "video",
					"q":              term,
					"order":          "viewCount",
					"publishedAfter": publishedAfter,
					"maxResults":     fmt.Sprintf("%d", clampInt(perCell, 1, 50)),
				}
				if duration != "" && duration != "any" {
					params["videoDuration"] = duration
				}
				if region != "" {
					params["regionCode"] = region
				}
				if category != "" {
					params["videoCategoryId"] = category
				}
				data, err := g.Get(ctx, "/youtube/v3/search", params)
				if err != nil {
					failures = append(failures, term+": "+err.Error())
					continue
				}
				var resp struct {
					Items []struct {
						ID struct {
							VideoID string `json:"videoId"`
						} `json:"id"`
					} `json:"items"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					failures = append(failures, term+": parse: "+err.Error())
					continue
				}
				for _, it := range resp.Items {
					if it.ID.VideoID != "" {
						videoIDs = append(videoIDs, it.ID.VideoID)
					}
				}
			}
			videoIDs = dedupeStrings(videoIDs)
			if len(videoIDs) == 0 {
				note := "no videos matched the filter matrix; widen --days or drop filters"
				if len(failures) > 0 {
					note = "all search fetches failed; see fetch_failures (check the API key before touching filters)"
				}
				res := breakoutsResult{Items: make([]breakoutRow, 0), CellsSearched: cells,
					Window: fmt.Sprintf("last %d days", days), FetchFailures: failures,
					SearchBudget: fmt.Sprintf("%d call(s) from the 100/day search bucket", cells),
					Note:         note}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			rows, err := fetchVideoStats(ctx, g, videoIDs)
			if err != nil && len(rows) == 0 {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			if err != nil {
				failures = append(failures, "partial stats fetch: "+err.Error())
			}
			chIDs := make([]string, 0, len(rows))
			for _, r := range rows {
				chIDs = append(chIDs, r.ChannelID)
			}
			chMap, chErr := fetchChannelsByIDs(ctx, g, dedupeStrings(chIDs))
			if chErr != nil {
				failures = append(failures, "channels batch: "+chErr.Error())
			}
			now := time.Now().UTC()
			items := make([]breakoutRow, 0, len(rows))
			for _, r := range rows {
				ch := chMap[r.ChannelID]
				var subs int64
				chTitle := ""
				if ch != nil {
					subs = ch.SubscriberCount
					chTitle = ch.Title
				}
				if subs < minSubs {
					continue
				}
				vpd := 0.0
				if t, err := time.Parse(time.RFC3339, r.PublishedAt); err == nil {
					if age := now.Sub(t).Hours() / 24; age > 0.25 {
						vpd = float64(r.ViewCount) / age
					} else {
						vpd = float64(r.ViewCount) * 4 // <6h old: floor the age at a quarter day
					}
				}
				vps := 0.0
				if subs > 0 {
					vps = float64(r.ViewCount) / float64(subs)
				}
				items = append(items, breakoutRow{
					VideoID: r.VideoID, Title: r.Title, ChannelID: r.ChannelID, Channel: chTitle,
					PublishedAt: r.PublishedAt, Views: r.ViewCount, Subs: subs,
					ViewsPerSub: vps, ViewsPerDay: vpd,
					WatchURL: "https://www.youtube.com/watch?v=" + r.VideoID,
				})
			}
			// Rank: views-per-subscriber first (breakout signal), views/day tiebreak.
			sort.Slice(items, func(i, j int) bool {
				if items[i].ViewsPerSub != items[j].ViewsPerSub {
					return items[i].ViewsPerSub > items[j].ViewsPerSub
				}
				return items[i].ViewsPerDay > items[j].ViewsPerDay
			})
			scanned := len(rows)
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			stored := 0
			if db, _, serr := openAnalystStore(ctx, dbPath); serr != nil {
				failures = append(failures, "local store: "+serr.Error())
			} else {
				if n, werr := writeVideoRows(ctx, db, rows, nowUTC()); werr != nil {
					failures = append(failures, "local store write: "+werr.Error())
				} else {
					stored = n
				}
				db.Close()
			}
			res := breakoutsResult{
				Items: items, CellsSearched: cells, ScannedVideos: scanned,
				Window:       fmt.Sprintf("last %d days", days),
				SearchBudget: fmt.Sprintf("%d call(s) from the 100/day search bucket", cells),
				RotatedKeys:  g.Rotated, FetchFailures: failures, StoredLocally: stored,
				MinSubsApplied: minSubs,
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d search cells or batches failed; ranking covers the rest\n", len(failures), cells+1)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%6.1fx views/sub  %8.0f views/day  %-30s  %s\n", it.ViewsPerSub, it.ViewsPerDay, truncateStr(it.Channel, 30), it.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&days, "days", 14, "Upload window in days (publishedAfter)")
	cmd.Flags().StringVar(&duration, "duration", "any", "Video duration filter: any, short, medium, long")
	cmd.Flags().StringVar(&region, "region", "", "Region code filter, e.g. DE or US")
	cmd.Flags().StringVar(&category, "category", "", "videoCategoryId filter (see 'youtube video-categories-list')")
	cmd.Flags().IntVar(&perCell, "per-cell", 25, "Search results per term (max 50; each term costs 1 search call)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum ranked rows to return")
	cmd.Flags().Int64Var(&minSubs, "min-subs", 0, "Ignore channels below this subscriber count")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
