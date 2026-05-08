// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/store"
)

func newNewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "news",
		Short: "FTS5 search across synced Steam news posts",
	}
	cmd.AddCommand(newNewsSearchCmd(flags))
	return cmd
}

type newsHit struct {
	Gid      string `json:"gid"`
	Appid    int    `json:"appid,omitempty"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	Date     int    `json:"date,omitempty"`
	Author   string `json:"author,omitempty"`
	Contents string `json:"contents,omitempty"`
}

type newsSearchOutput struct {
	Query string    `json:"query"`
	Hits  []newsHit `json:"hits"`
	Note  string    `json:"note,omitempty"`
}

func newNewsSearchCmd(flags *rootFlags) *cobra.Command {
	var queryArg, sinceFlag, appidFlag string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search news titles + contents using SQLite FTS5 over the local store",
		Long: `Searches the local store's FTS5 index over news posts. To populate, sync via:
  steam-web-pp-cli isteam-news get-news-for-app --appid 1245620

Returns empty hit list with a note if nothing has been synced yet.`,
		Example:     "  steam-web-pp-cli news search 'patch notes' --since 2026-04-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				// Join all positional args so unquoted multi-word queries
				// (e.g. `news search patch notes`) are treated as one phrase
				// rather than silently dropping every token after the first.
				queryArg = strings.TrimSpace(strings.Join(args, " "))
			}
			if queryArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenReadOnly(defaultDBPath("steam-web-pp-cli"))
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), newsSearchOutput{
					Query: queryArg,
					Note:  "no_news_in_store: local store not initialized; sync news with `steam-web-pp-cli isteam-news get-news-for-app --appid <APPID>` first",
				}, flags)
			}
			defer db.Close()
			results, err := db.Search(sanitizeFTSQuery(queryArg), 0)
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), newsSearchOutput{
					Query: queryArg,
					Note:  fmt.Sprintf("fts_query_invalid: %v; try wrapping in double quotes for phrase search or stripping punctuation", err),
				}, flags)
			}
			out := newsSearchOutput{Query: queryArg, Hits: []newsHit{}}
			var sinceTS int
			if sinceFlag != "" {
				if ts, err := parseSinceFlag(sinceFlag); err == nil {
					sinceTS = ts
				}
			}
			var appidNum int
			if appidFlag != "" {
				appidNum, _ = strconv.Atoi(appidFlag)
			}
			for _, raw := range results {
				var item newsHit
				if err := json.Unmarshal(raw, &item); err != nil {
					continue
				}
				if item.Title == "" && item.Gid == "" {
					continue
				}
				if appidNum > 0 && item.Appid != 0 && item.Appid != appidNum {
					continue
				}
				if sinceTS > 0 && item.Date != 0 && item.Date < sinceTS {
					continue
				}
				out.Hits = append(out.Hits, item)
			}
			if len(out.Hits) == 0 {
				out.Note = "no matches; populate news first via `steam-web-pp-cli isteam-news get-news-for-app --appid <APPID>`"
			}
			if limit > 0 && len(out.Hits) > limit {
				out.Hits = out.Hits[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&queryArg, "query", "", "FTS5 query string")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only include posts on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&appidFlag, "app", "", "Scope to a single appid")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum hits to return (0 unlimited)")
	return cmd
}

func sanitizeFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	replacer := strings.NewReplacer(
		"'", "", "\"", "", "(", "", ")", "",
		":", "", "^", "", "+", "", "*", "",
	)
	q = replacer.Replace(q)
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	if strings.Contains(q, " ") {
		return "\"" + q + "\""
	}
	return q
}

func parseSinceFlag(s string) (int, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	return int(t.Unix()), nil
}

type reviewBucket struct {
	Date         string  `json:"date"`
	Reviews      int     `json:"reviews"`
	VotedUp      int     `json:"voted_up"`
	VotedUpShare float64 `json:"voted_up_share"`
}

type reviewVelocityOutput struct {
	Appid   string         `json:"appid"`
	Window  string         `json:"window"`
	Buckets []reviewBucket `json:"buckets"`
	Total   int            `json:"total_reviews"`
	Note    string         `json:"note,omitempty"`
}

func newReviewVelocityCmd(flags *rootFlags) *cobra.Command {
	var appidArg, windowFlag string
	cmd := &cobra.Command{
		Use:         "review-velocity [appid]",
		Short:       "Reviews/day and voted-up share for one app over a rolling window",
		Long:        "Fetches one cursor page of recent appreviews from store.steampowered.com/api/appreviews and buckets results by day.",
		Example:     "  steam-web-pp-cli review-velocity 1245620 --window 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				appidArg = args[0]
			}
			if appidArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if n, err := strconv.Atoi(appidArg); err != nil || n <= 0 {
				return fmt.Errorf("invalid appid %q: must be a positive integer", appidArg)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			windowDays := 30
			switch windowFlag {
			case "7d":
				windowDays = 7
			case "14d":
				windowDays = 14
			case "30d", "":
				windowDays = 30
			default:
				return fmt.Errorf("unknown --window value %q (use 7d, 14d, 30d)", windowFlag)
			}
			cutoff := time.Now().AddDate(0, 0, -windowDays).Unix()
			path := fmt.Sprintf("/appreviews/%s", appidArg)
			c.BaseURL = "https://store.steampowered.com"
			data, err := c.Get(path, map[string]string{
				"json": "1", "num_per_page": "100", "filter": "recent", "language": "english",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				QuerySummary struct {
					NumReviews int `json:"num_reviews"`
				} `json:"query_summary"`
				Reviews []struct {
					Recommendationid string `json:"recommendationid"`
					VotedUp          bool   `json:"voted_up"`
					Timestamp        int64  `json:"timestamp_created"`
				} `json:"reviews"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parse appreviews: %w", err)
			}
			bucketMap := map[string]*reviewBucket{}
			total := 0
			for _, r := range resp.Reviews {
				if r.Timestamp < cutoff {
					continue
				}
				date := time.Unix(r.Timestamp, 0).UTC().Format("2006-01-02")
				if _, ok := bucketMap[date]; !ok {
					bucketMap[date] = &reviewBucket{Date: date}
				}
				bucketMap[date].Reviews++
				if r.VotedUp {
					bucketMap[date].VotedUp++
				}
				total++
			}
			buckets := make([]reviewBucket, 0, len(bucketMap))
			for _, b := range bucketMap {
				if b.Reviews > 0 {
					b.VotedUpShare = float64(b.VotedUp) / float64(b.Reviews)
				}
				buckets = append(buckets, *b)
			}
			sort.Slice(buckets, func(i, j int) bool { return buckets[i].Date < buckets[j].Date })
			out := reviewVelocityOutput{
				Appid: appidArg, Window: fmt.Sprintf("%dd", windowDays),
				Buckets: buckets, Total: total,
			}
			if total == 0 {
				out.Note = "no_reviews_in_window: increase --window or check that appid is correct"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&appidArg, "appid", "", "App ID")
	cmd.Flags().StringVar(&windowFlag, "window", "30d", "Rolling window (7d, 14d, 30d)")
	return cmd
}

type playTrendSample struct {
	SampledAt   string `json:"sampled_at"`
	PlayerCount int    `json:"player_count"`
}

type playTrendOutput struct {
	Appid      string            `json:"appid"`
	Window     string            `json:"window"`
	Samples    []playTrendSample `json:"samples"`
	CurrentNow int               `json:"current_now"`
	WindowMin  int               `json:"window_min"`
	WindowMax  int               `json:"window_max"`
	Sparkline  string            `json:"sparkline"`
	Note       string            `json:"note,omitempty"`
}

func newPlayTrendCmd(flags *rootFlags) *cobra.Command {
	var appidArg, windowFlag string
	cmd := &cobra.Command{
		Use:         "play-trend [appid]",
		Short:       "Concurrent-player trend for one app (sample on each run; cron-builds-trend)",
		Long:        "Calls GetNumberOfCurrentPlayers, persists the sample, then reads stored samples in the requested window.",
		Example:     "  steam-web-pp-cli play-trend 1245620 --window 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				appidArg = args[0]
			}
			if appidArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if n, err := strconv.Atoi(appidArg); err != nil || n <= 0 {
				return fmt.Errorf("invalid appid %q: must be a positive integer", appidArg)
			}
			windowDays := 7
			switch windowFlag {
			case "24h", "1d":
				windowDays = 1
			case "7d":
				windowDays = 7
			case "30d":
				windowDays = 30
			case "":
				windowDays = 7
			default:
				return fmt.Errorf("unknown --window value %q (use 24h, 7d, 30d)", windowFlag)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/ISteamUserStats/GetNumberOfCurrentPlayers/v1", map[string]string{"appid": appidArg})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var live struct {
				Response struct {
					PlayerCount int `json:"player_count"`
					Result      int `json:"result"`
				} `json:"response"`
			}
			if err := json.Unmarshal(data, &live); err != nil {
				return fmt.Errorf("parse player count: %w", err)
			}
			// Skip persisting failed samples. Steam's GetNumberOfCurrentPlayers
			// returns result==1 on success; on failure (or for delisted/unreleased
			// apps) it returns result!=1 with player_count=0. Persisting that 0
			// produces phantom-zero troughs that drag window_min to 0 and put
			// fake low bars in the sparkline. Live games don't hit 0 abruptly,
			// so we treat result!=1 as a transient fetch failure rather than data.
			persistSample := live.Response.Result == 1
			db, err := store.Open(defaultDBPath("steam-web-pp-cli"))
			if err == nil {
				now := time.Now().UTC()
				if persistSample {
					sampleID := fmt.Sprintf("app_player_count:%s:%d", appidArg, now.Unix())
					doc, _ := json.Marshal(map[string]any{
						"appid": appidArg, "sampled_at": now.Format(time.RFC3339),
						"player_count": live.Response.PlayerCount,
					})
					_ = db.Upsert(fmt.Sprintf("app_player_count:%s", appidArg), sampleID, doc)
				}
				defer db.Close()
				cutoff := now.Add(-time.Duration(windowDays) * 24 * time.Hour)
				samples := []playTrendSample{}
				rows, qerr := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM resources WHERE resource_type = ? ORDER BY synced_at`,
					fmt.Sprintf("app_player_count:%s", appidArg))
				if qerr == nil {
					defer rows.Close()
					for rows.Next() {
						var jsonBlob string
						if err := rows.Scan(&jsonBlob); err == nil {
							var s playTrendSample
							if err := json.Unmarshal([]byte(jsonBlob), &s); err == nil {
								// Filter phantom-zero samples written by older
								// versions (pre-fix) that persisted failed
								// fetches. Live games with 0 concurrent players
								// shouldn't appear in this view.
								if s.PlayerCount <= 0 {
									continue
								}
								if t, err := time.Parse(time.RFC3339, s.SampledAt); err == nil && !t.Before(cutoff) {
									samples = append(samples, s)
								}
							}
						}
					}
				}
				out := playTrendOutput{
					Appid: appidArg, Window: fmt.Sprintf("%dd", windowDays),
					Samples: samples, CurrentNow: live.Response.PlayerCount,
				}
				if len(samples) > 0 {
					out.WindowMin, out.WindowMax = samples[0].PlayerCount, samples[0].PlayerCount
					for _, s := range samples {
						if s.PlayerCount < out.WindowMin {
							out.WindowMin = s.PlayerCount
						}
						if s.PlayerCount > out.WindowMax {
							out.WindowMax = s.PlayerCount
						}
					}
					out.Sparkline = sparkline(samplePoints(samples))
				}
				if len(samples) <= 1 {
					out.Note = "single_sample: run on a cron (every 1-6h) to build a trend."
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			out := playTrendOutput{
				Appid: appidArg, Window: fmt.Sprintf("%dd", windowDays),
				CurrentNow: live.Response.PlayerCount,
				Note:       "store_unavailable: only live current_now reported",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&appidArg, "appid", "", "App ID")
	cmd.Flags().StringVar(&windowFlag, "window", "7d", "Rolling window (24h, 7d, 30d)")
	return cmd
}

func samplePoints(samples []playTrendSample) []float64 {
	pts := make([]float64, 0, len(samples))
	for _, s := range samples {
		pts = append(pts, float64(s.PlayerCount))
	}
	return pts
}

func sparkline(points []float64) string {
	if len(points) <= 1 {
		if len(points) == 1 {
			return "▆"
		}
		return ""
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	min, max := points[0], points[0]
	for _, p := range points {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}
	rng := max - min
	if rng == 0 {
		rng = 1
	}
	var b strings.Builder
	for _, p := range points {
		idx := int((p - min) / rng * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		b.WriteRune(bars[idx])
	}
	return b.String()
}
