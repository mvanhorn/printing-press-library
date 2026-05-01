// Package cli — hand-built transcendence commands for scrape-creators-pp-cli.
// These commands compound multiple API calls or analyze synced data to produce
// intelligence that no single API endpoint can provide.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func fetchTikTokVideos(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, handle string) ([]map[string]any, error) {
	data, err := c.Get("/v3/tiktok/profile/videos", map[string]string{"handle": handle})
	if err != nil {
		return nil, err
	}
	// Response: {"aweme_list":[...], "max_cursor":...}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil {
		return nil, fmt.Errorf("unexpected response shape from profile/videos")
	}
	raw, ok := envelope["aweme_list"]
	if !ok {
		return nil, fmt.Errorf("no aweme_list in response")
	}
	var videos []map[string]any
	if err := json.Unmarshal(raw, &videos); err != nil {
		return nil, fmt.Errorf("parsing videos: %w", err)
	}
	return videos, nil
}

func videoStats(v map[string]any) (play, like, comment, share float64) {
	stats, _ := v["statistics"].(map[string]any)
	if stats == nil {
		return 0, 0, 0, 0
	}
	toF := func(k string) float64 {
		switch n := stats[k].(type) {
		case float64:
			return n
		case json.Number:
			f, _ := n.Float64()
			return f
		}
		return 0
	}
	return toF("play_count"), toF("digg_count"), toF("comment_count"), toF("share_count")
}

func engagementRate(play, like, comment, share float64) float64 {
	if play == 0 {
		return 0
	}
	return (like + comment + share) / play * 100
}

func firstNumericField(obj map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch v := obj[key].(type) {
		case float64:
			return v
		case json.Number:
			f, _ := v.Float64()
			return f
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
	}
	return 0
}

func nestedMap(obj map[string]any, key string) map[string]any {
	child, _ := obj[key].(map[string]any)
	return child
}

func parseAccountBudgetData(balanceData, dailyData json.RawMessage) (float64, float64) {
	var balance map[string]any
	_ = json.Unmarshal(balanceData, &balance)

	creditsRemaining := firstNumericField(balance, "credits_remaining", "creditCount")
	if results := nestedMap(balance, "results"); results != nil {
		if nested := firstNumericField(results, "credits_remaining", "creditCount"); nested > 0 {
			creditsRemaining = nested
		}
	}

	var daily map[string]any
	_ = json.Unmarshal(dailyData, &daily)

	dailyBurn := 0.0
	var usage []any
	if arrayRoot := []any(nil); json.Unmarshal(dailyData, &arrayRoot) == nil && len(arrayRoot) > 0 {
		usage = arrayRoot
	} else {
		usageRoot := daily
		if results := nestedMap(daily, "results"); results != nil {
			usageRoot = results
		}
		usage, _ = usageRoot["usage"].([]any)
	}
	if len(usage) > 0 {
		total := 0.0
		for _, u := range usage {
			if m, ok := u.(map[string]any); ok {
				total += firstNumericField(m, "count", "credits", "creditCount", "total_credits", "request_count")
			}
		}
		dailyBurn = total / float64(len(usage))
	}

	return creditsRemaining, dailyBurn
}

// ── spikes ───────────────────────────────────────────────────────────────────

func newTiktokSpikesCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "spikes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find videos that outperformed a creator's average engagement rate",
		Long: `Fetches all available videos for a creator, computes the average engagement
rate (likes+comments+shares / views), then returns videos that exceeded the
threshold multiplier of that average.

Engagement rate = (likes + comments + shares) / views × 100`,
		Example: `  # Find videos 2× above @charlidamelio's average
  scrape-creators-pp-cli tiktok spikes --handle charlidamelio --threshold 2

  # Output for agents
  scrape-creators-pp-cli tiktok spikes --handle charlidamelio --threshold 1.5 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				return fmt.Errorf("required flag \"handle\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			videos, err := fetchTikTokVideos(c, handle)
			if err != nil {
				return classifyAPIError(err)
			}
			if len(videos) == 0 {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("no videos found for @%s", handle)
			}

			// Compute per-video engagement rates
			type vidResult struct {
				ID             string  `json:"id"`
				Description    string  `json:"description"`
				PlayCount      float64 `json:"play_count"`
				LikeCount      float64 `json:"like_count"`
				CommentCount   float64 `json:"comment_count"`
				ShareCount     float64 `json:"share_count"`
				EngagementRate float64 `json:"engagement_rate"`
				CreatedAt      int64   `json:"created_at,omitempty"`
			}

			var results []vidResult
			var totalER float64
			for _, v := range videos {
				play, like, comment, share := videoStats(v)
				er := engagementRate(play, like, comment, share)
				totalER += er
				id, _ := v["aweme_id"].(string)
				desc, _ := v["desc"].(string)
				var createdAt int64
				if ct, ok := v["create_time"].(float64); ok {
					createdAt = int64(ct)
				}
				results = append(results, vidResult{
					ID:             id,
					Description:    desc,
					PlayCount:      play,
					LikeCount:      like,
					CommentCount:   comment,
					ShareCount:     share,
					EngagementRate: math.Round(er*100) / 100,
					CreatedAt:      createdAt,
				})
			}
			avgER := totalER / float64(len(results))
			cutoff := avgER * threshold

			var spikes []vidResult
			for _, r := range results {
				if r.EngagementRate >= cutoff {
					spikes = append(spikes, r)
				}
			}
			sort.Slice(spikes, func(i, j int) bool {
				return spikes[i].EngagementRate > spikes[j].EngagementRate
			})

			fmt.Fprintf(cmd.ErrOrStderr(), "@%s: %d videos analyzed, avg engagement %.2f%%, threshold %.1f× = %.2f%%, spikes found: %d\n",
				handle, len(results), avgER, threshold, cutoff, len(spikes))

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(map[string]any{
					"handle":                  handle,
					"total_videos":            len(results),
					"average_engagement_rate": math.Round(avgER*100) / 100,
					"threshold_multiplier":    threshold,
					"threshold_rate":          math.Round(cutoff*100) / 100,
					"spikes":                  spikes,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(spikes) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No videos exceeded %.1f× the average engagement rate (%.2f%%).\n", threshold, avgER)
				return nil
			}
			var rows []map[string]any
			for _, s := range spikes {
				rows = append(rows, map[string]any{
					"id":          s.ID,
					"engagement%": fmt.Sprintf("%.2f", s.EngagementRate),
					"plays":       int64(s.PlayCount),
					"likes":       int64(s.LikeCount),
					"desc":        truncate(s.Description, 60),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "TikTok handle (without @) (required)")
	cmd.Flags().Float64Var(&threshold, "threshold", 2.0, "Multiplier above average engagement rate (e.g. 2 = 2× average)")
	return cmd
}

// ── analyze ───────────────────────────────────────────────────────────────────

func newTiktokAnalyzeCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var limit int

	cmd := &cobra.Command{
		Use:   "analyze",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Rank a creator's videos by engagement rate (likes+comments+shares / views)",
		Example: `  scrape-creators-pp-cli tiktok analyze --handle charlidamelio --limit 10
  scrape-creators-pp-cli tiktok analyze --handle charlidamelio --json --select id,engagement_rate,play_count`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				return fmt.Errorf("required flag \"handle\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			videos, err := fetchTikTokVideos(c, handle)
			if err != nil {
				return classifyAPIError(err)
			}
			if len(videos) == 0 {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("no videos found for @%s", handle)
			}

			type ranked struct {
				Rank           int     `json:"rank"`
				ID             string  `json:"id"`
				EngagementRate float64 `json:"engagement_rate"`
				PlayCount      float64 `json:"play_count"`
				LikeCount      float64 `json:"like_count"`
				CommentCount   float64 `json:"comment_count"`
				ShareCount     float64 `json:"share_count"`
				Description    string  `json:"description"`
			}

			var items []ranked
			for _, v := range videos {
				play, like, comment, share := videoStats(v)
				er := math.Round(engagementRate(play, like, comment, share)*100) / 100
				id, _ := v["aweme_id"].(string)
				desc, _ := v["desc"].(string)
				items = append(items, ranked{
					ID: id, EngagementRate: er,
					PlayCount: play, LikeCount: like,
					CommentCount: comment, ShareCount: share,
					Description: desc,
				})
			}
			sort.Slice(items, func(i, j int) bool {
				return items[i].EngagementRate > items[j].EngagementRate
			})
			if limit > 0 && limit < len(items) {
				items = items[:limit]
			}
			for i := range items {
				items[i].Rank = i + 1
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(items, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			var rows []map[string]any
			for _, it := range items {
				rows = append(rows, map[string]any{
					"rank":        it.Rank,
					"engagement%": fmt.Sprintf("%.2f", it.EngagementRate),
					"plays":       int64(it.PlayCount),
					"likes":       int64(it.LikeCount),
					"desc":        truncate(it.Description, 60),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "TikTok handle (required)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of top videos to show")
	return cmd
}

// ── cadence ───────────────────────────────────────────────────────────────────

func newTiktokCadenceCmd(flags *rootFlags) *cobra.Command {
	var handle string

	cmd := &cobra.Command{
		Use:   "cadence",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a creator's posting frequency by day of week and hour of day",
		Example: `  scrape-creators-pp-cli tiktok cadence --handle charlidamelio
  scrape-creators-pp-cli tiktok cadence --handle charlidamelio --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				return fmt.Errorf("required flag \"handle\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			videos, err := fetchTikTokVideos(c, handle)
			if err != nil {
				return classifyAPIError(err)
			}
			if len(videos) == 0 {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("no videos found for @%s", handle)
			}

			dayCount := make(map[string]int)
			hourCount := make(map[int]int)
			days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

			for _, v := range videos {
				ct, ok := v["create_time"].(float64)
				if !ok {
					continue
				}
				t := time.Unix(int64(ct), 0).UTC()
				dayCount[days[t.Weekday()]]++
				hourCount[t.Hour()]++
			}

			type dayStat struct {
				Day   string `json:"day"`
				Count int    `json:"count"`
			}
			type hourStat struct {
				Hour  int    `json:"hour"`
				Label string `json:"label"`
				Count int    `json:"count"`
			}

			var dayStats []dayStat
			for _, d := range days {
				dayStats = append(dayStats, dayStat{Day: d, Count: dayCount[d]})
			}
			var hourStats []hourStat
			for h := 0; h < 24; h++ {
				label := fmt.Sprintf("%02d:00", h)
				hourStats = append(hourStats, hourStat{Hour: h, Label: label, Count: hourCount[h]})
			}

			result := map[string]any{
				"handle":  handle,
				"total":   len(videos),
				"by_day":  dayStats,
				"by_hour": hourStats,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "@%s posting cadence (%d videos analyzed)\n\n", handle, len(videos))
			fmt.Fprintln(cmd.OutOrStdout(), "By day of week:")
			for _, d := range dayStats {
				bar := strings.Repeat("█", d.Count)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %3d  %s\n", d.Day, d.Count, bar)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nPeak hours (UTC):")
			var topHours []hourStat
			for _, h := range hourStats {
				if h.Count > 0 {
					topHours = append(topHours, h)
				}
			}
			sort.Slice(topHours, func(i, j int) bool { return topHours[i].Count > topHours[j].Count })
			if len(topHours) > 5 {
				topHours = topHours[:5]
			}
			for _, h := range topHours {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d posts\n", h.Label, h.Count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "TikTok handle (required)")
	return cmd
}

// ── compare ───────────────────────────────────────────────────────────────────

func newTiktokCompareCmd(flags *rootFlags) *cobra.Command {
	var handles []string

	cmd := &cobra.Command{
		Use:   "compare",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare multiple TikTok creators side-by-side on followers, engagement, and posting cadence",
		Example: `  scrape-creators-pp-cli tiktok compare --handle charlidamelio --handle addisonre
  scrape-creators-pp-cli tiktok compare --handle charlidamelio --handle addisonre --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(handles) < 2 {
				return fmt.Errorf("at least two --handle flags required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type creatorStat struct {
				Handle        string  `json:"handle"`
				Nickname      string  `json:"nickname"`
				Followers     float64 `json:"followers"`
				Following     float64 `json:"following"`
				Likes         float64 `json:"total_likes"`
				VideoCount    float64 `json:"video_count"`
				AvgEngagement float64 `json:"avg_engagement_rate"`
				Verified      bool    `json:"verified"`
			}

			var stats []creatorStat
			for _, h := range handles {
				profileData, err := c.Get("/v1/tiktok/profile", map[string]string{"handle": h})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to fetch @%s: %v\n", h, err)
					continue
				}
				var profile map[string]any
				json.Unmarshal(profileData, &profile)

				user, _ := profile["user"].(map[string]any)
				st, _ := profile["stats"].(map[string]any)
				if user == nil {
					user = map[string]any{}
				}
				if st == nil {
					st = map[string]any{}
				}

				toF := func(m map[string]any, k string) float64 {
					switch n := m[k].(type) {
					case float64:
						return n
					case json.Number:
						f, _ := n.Float64()
						return f
					}
					return 0
				}
				nickname, _ := user["nickname"].(string)
				verified, _ := user["verified"].(bool)

				// Compute avg engagement from videos
				videos, _ := fetchTikTokVideos(c, h)
				var totalER float64
				for _, v := range videos {
					play, like, comment, share := videoStats(v)
					totalER += engagementRate(play, like, comment, share)
				}
				avgER := 0.0
				if len(videos) > 0 {
					avgER = math.Round(totalER/float64(len(videos))*100) / 100
				}

				stats = append(stats, creatorStat{
					Handle:        h,
					Nickname:      nickname,
					Followers:     toF(st, "followerCount"),
					Following:     toF(st, "followingCount"),
					Likes:         toF(st, "heartCount"),
					VideoCount:    toF(st, "videoCount"),
					AvgEngagement: avgER,
					Verified:      verified,
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			var rows []map[string]any
			for _, s := range stats {
				rows = append(rows, map[string]any{
					"handle":    "@" + s.Handle,
					"name":      s.Nickname,
					"followers": formatCount(s.Followers),
					"likes":     formatCount(s.Likes),
					"videos":    int(s.VideoCount),
					"avg_eng%":  fmt.Sprintf("%.2f", s.AvgEngagement),
					"verified":  s.Verified,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringArrayVar(&handles, "handle", nil, "TikTok handle to compare (repeat for each creator)")
	return cmd
}

// ── transcripts ───────────────────────────────────────────────────────────────

func newTiktokTranscriptsCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var query string
	var limit int

	cmd := &cobra.Command{
		Use:   "transcripts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch and search across a creator's video transcripts",
		Example: `  # Search transcripts for a keyword
  scrape-creators-pp-cli tiktok transcripts --handle charlidamelio --search "morning routine"

  # Fetch all transcripts as JSON
  scrape-creators-pp-cli tiktok transcripts --handle charlidamelio --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				return fmt.Errorf("required flag \"handle\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			videos, err := fetchTikTokVideos(c, handle)
			if err != nil {
				return classifyAPIError(err)
			}
			if len(videos) == 0 {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("no videos found for @%s", handle)
			}

			if limit > 0 && limit < len(videos) {
				videos = videos[:limit]
			}

			type transcriptResult struct {
				VideoID    string `json:"video_id"`
				VideoURL   string `json:"video_url,omitempty"`
				Transcript string `json:"transcript"`
				Snippet    string `json:"snippet,omitempty"`
				Match      bool   `json:"match,omitempty"`
			}

			var results []transcriptResult
			fmt.Fprintf(cmd.ErrOrStderr(), "Fetching transcripts for %d videos...\n", len(videos))
			for i, v := range videos {
				// Build video URL from aweme_id
				id, _ := v["aweme_id"].(string)
				author, _ := v["author"].(map[string]any)
				authorHandle := handle
				if author != nil {
					if uid, ok := author["unique_id"].(string); ok {
						authorHandle = uid
					}
				}
				videoURL := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", authorHandle, id)

				fmt.Fprintf(cmd.ErrOrStderr(), "  [%d/%d] fetching transcript for video %s\n", i+1, len(videos), id)
				tData, tErr := c.Get("/v1/tiktok/video/transcript", map[string]string{"url": videoURL})
				if tErr != nil {
					continue
				}
				var tResp map[string]any
				json.Unmarshal(tData, &tResp)
				transcript, _ := tResp["transcript"].(string)

				r := transcriptResult{
					VideoID:    id,
					VideoURL:   videoURL,
					Transcript: transcript,
				}
				if query != "" {
					lower := strings.ToLower(transcript)
					needle := strings.ToLower(query)
					if strings.Contains(lower, needle) {
						r.Match = true
						idx := strings.Index(lower, needle)
						start := max(0, idx-50)
						end := min(len(transcript), idx+len(query)+50)
						r.Snippet = "..." + transcript[start:end] + "..."
					}
				}
				results = append(results, r)
			}

			if query != "" {
				var matches []transcriptResult
				for _, r := range results {
					if r.Match {
						matches = append(matches, r)
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Found %d matches for %q in %d transcripts\n", len(matches), query, len(results))
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					out, _ := json.MarshalIndent(matches, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					return nil
				}
				if len(matches) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No transcripts contain %q.\n", query)
					return nil
				}
				for _, m := range matches {
					fmt.Fprintf(cmd.OutOrStdout(), "Video %s\n  %s\n\n", m.VideoID, m.Snippet)
				}
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(results, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "--- %s ---\n%s\n\n", r.VideoID, r.Transcript)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "TikTok handle (required)")
	cmd.Flags().StringVar(&query, "search", "", "Search term to find in transcripts")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max number of videos to fetch transcripts for (transcript API costs credits)")
	return cmd
}

// ── account budget ────────────────────────────────────────────────────────────

func newAccountBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show API credit balance and projected days remaining at current burn rate",
		Example: `  scrape-creators-pp-cli account budget
  scrape-creators-pp-cli account budget --agent --select credits_remaining,days_remaining`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Get current balance
			balanceData, err := c.Get("/v1/account/credit-balance", map[string]string{})
			if err != nil {
				// Fallback: try profile endpoint which returns credits_remaining
				balanceData, err = c.Get("/v1/tiktok/profile", map[string]string{"handle": "tiktok"})
				if err != nil {
					return classifyAPIError(err)
				}
			}

			// Get daily usage
			dailyData, _ := c.Get("/v1/account/get-daily-usage-count", map[string]string{})
			creditsRemaining, dailyBurn := parseAccountBudgetData(balanceData, dailyData)

			daysRemaining := 0.0
			if dailyBurn > 0 {
				daysRemaining = math.Round(creditsRemaining / dailyBurn)
			}

			result := map[string]any{
				"credits_remaining": creditsRemaining,
				"daily_burn_rate":   math.Round(dailyBurn),
				"days_remaining":    daysRemaining,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Credits remaining: %.0f\n", creditsRemaining)
			if dailyBurn > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Daily burn rate:   %.0f credits/day\n", dailyBurn)
				fmt.Fprintf(cmd.OutOrStdout(), "Days remaining:    %.0f days\n", daysRemaining)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Daily burn rate:   insufficient history")
			}
			return nil
		},
	}
	return cmd
}

// ── hashtag trends ────────────────────────────────────────────────────────────

type storedTrendVideo struct {
	ID        string  `json:"id"`
	PlayCount float64 `json:"play_count,omitempty"`
	Rank      int     `json:"rank,omitempty"`
}

type searchTrendTopVideo struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	PlayCount   float64 `json:"play_count"`
	LikeCount   float64 `json:"like_count"`
}

type hashtagTrendSnapshot struct {
	Date       string             `json:"date"`
	SnapshotAt string             `json:"snapshot_at"`
	Hashtag    string             `json:"hashtag"`
	VideoCount int                `json:"video_count"`
	TopVideos  []storedTrendVideo `json:"top_videos,omitempty"`
}

type hashtagTrendHistoryRow struct {
	Date                string   `json:"date"`
	SnapshotAt          string   `json:"snapshot_at"`
	VideoCount          int      `json:"video_count"`
	Delta               int      `json:"delta"`
	Direction           string   `json:"direction"`
	PersistentTopVideos int      `json:"persistent_top_videos"`
	NewTopVideos        int      `json:"new_top_videos"`
	TopVideoIDs         []string `json:"top_video_ids,omitempty"`
}

type stickyTrendVideo struct {
	ID            string `json:"id"`
	Appearances   int    `json:"appearances"`
	LongestStreak int    `json:"longest_streak"`
	BestRank      int    `json:"best_rank"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
}

type hashtagTrendHistoryResponse struct {
	Hashtag         string                   `json:"hashtag"`
	Snapshots       []hashtagTrendHistoryRow `json:"snapshots"`
	SnapshotsCount  int                      `json:"snapshots_count"`
	StickyTopVideos []stickyTrendVideo       `json:"sticky_top_videos,omitempty"`
}

var fetchSearchTrendData = func(flags *rootFlags, tag string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return c.Get("/v1/tiktok/search/hashtag", map[string]string{"hashtag": tag})
}

func searchTrendTrackingDir() string {
	dbPath := defaultDBPath("scrape-creators-pp-cli")
	return strings.TrimSuffix(dbPath, "/data.db") + "/tracking"
}

func searchTrendSnapshotFile(hashtag string) string {
	return filepath.Join(searchTrendTrackingDir(), "hashtag-"+normalizeHashtagKey(hashtag)+".json")
}

func persistHashtagTrendSnapshots(snapshotFile string, snapshots []hashtagTrendSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(snapshotFile), 0o755); err != nil {
		return fmt.Errorf("creating tracking dir: %w", err)
	}
	out, marshalErr := json.MarshalIndent(snapshots, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("serializing trend history: %w", marshalErr)
	}
	if writeErr := os.WriteFile(snapshotFile, out, 0o644); writeErr != nil {
		return fmt.Errorf("saving trend snapshot: %w", writeErr)
	}
	return nil
}

func normalizeHashtagKey(hashtag string) string {
	trimmed := strings.ToLower(NormalizeHashtag(hashtag))
	var b strings.Builder
	prevDash := false
	for _, r := range trimmed {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if b.Len() == 0 || prevDash {
			continue
		}
		b.WriteByte('-')
		prevDash = true
	}
	key := strings.Trim(b.String(), "-")
	if key == "" {
		return "hashtag"
	}
	return key
}

func displayHashtag(hashtag string) string {
	tag := NormalizeHashtag(hashtag)
	if tag == "" {
		tag = normalizeHashtagKey(hashtag)
	}
	return "#" + tag
}

func storedTopVideos(top []searchTrendTopVideo) []storedTrendVideo {
	stored := make([]storedTrendVideo, 0, len(top))
	for i, v := range top {
		if v.ID == "" {
			continue
		}
		stored = append(stored, storedTrendVideo{
			ID:        v.ID,
			PlayCount: v.PlayCount,
			Rank:      i + 1,
		})
	}
	return stored
}

func uniqueTrendVideoIDs(videos []storedTrendVideo) []string {
	ids := make([]string, 0, len(videos))
	seen := make(map[string]struct{}, len(videos))
	for _, video := range videos {
		if video.ID == "" {
			continue
		}
		if _, ok := seen[video.ID]; ok {
			continue
		}
		seen[video.ID] = struct{}{}
		ids = append(ids, video.ID)
	}
	return ids
}

func trendVideoIDSet(videos []storedTrendVideo) map[string]struct{} {
	set := make(map[string]struct{}, len(videos))
	for _, id := range uniqueTrendVideoIDs(videos) {
		set[id] = struct{}{}
	}
	return set
}

func intersectTrendVideoSets(a, b map[string]struct{}) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	count := 0
	for id := range a {
		if _, ok := b[id]; ok {
			count++
		}
	}
	return count
}

func trendDirection(delta int) string {
	switch {
	case delta > 0:
		return "up"
	case delta < 0:
		return "down"
	default:
		return "flat"
	}
}

func buildTrendHistory(display string, snapshots []hashtagTrendSnapshot) hashtagTrendHistoryResponse {
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Date < snapshots[j].Date
	})

	resp := hashtagTrendHistoryResponse{
		Hashtag:   display,
		Snapshots: make([]hashtagTrendHistoryRow, 0, len(snapshots)),
	}

	appearances := map[string]int{}
	currentStreak := map[string]int{}
	longestStreak := map[string]int{}
	bestRank := map[string]int{}
	firstSeen := map[string]string{}
	lastSeen := map[string]string{}
	prevSet := map[string]struct{}{}

	for i, snapshot := range snapshots {
		if snapshot.Hashtag != "" {
			resp.Hashtag = snapshot.Hashtag
		}

		currSet := trendVideoIDSet(snapshot.TopVideos)
		topIDs := uniqueTrendVideoIDs(snapshot.TopVideos)
		delta := 0
		direction := "baseline"
		persistent := 0
		newVideos := len(currSet)
		if i > 0 {
			delta = snapshot.VideoCount - snapshots[i-1].VideoCount
			direction = trendDirection(delta)
			persistent = intersectTrendVideoSets(prevSet, currSet)
			newVideos = len(currSet) - persistent
		}

		resp.Snapshots = append(resp.Snapshots, hashtagTrendHistoryRow{
			Date:                snapshot.Date,
			SnapshotAt:          snapshot.SnapshotAt,
			VideoCount:          snapshot.VideoCount,
			Delta:               delta,
			Direction:           direction,
			PersistentTopVideos: persistent,
			NewTopVideos:        newVideos,
			TopVideoIDs:         topIDs,
		})

		seenThisSnapshot := map[string]struct{}{}
		for _, video := range snapshot.TopVideos {
			if video.ID == "" {
				continue
			}
			if _, ok := seenThisSnapshot[video.ID]; ok {
				continue
			}
			seenThisSnapshot[video.ID] = struct{}{}
			appearances[video.ID]++
			if firstSeen[video.ID] == "" {
				firstSeen[video.ID] = snapshot.Date
			}
			lastSeen[video.ID] = snapshot.Date
			if rank, ok := bestRank[video.ID]; !ok || (video.Rank > 0 && video.Rank < rank) {
				bestRank[video.ID] = video.Rank
			}
			if _, ok := prevSet[video.ID]; ok {
				currentStreak[video.ID]++
			} else {
				currentStreak[video.ID] = 1
			}
			if currentStreak[video.ID] > longestStreak[video.ID] {
				longestStreak[video.ID] = currentStreak[video.ID]
			}
		}
		for id := range prevSet {
			if _, ok := currSet[id]; !ok {
				currentStreak[id] = 0
			}
		}
		prevSet = currSet
	}

	for id, appearanceCount := range appearances {
		if appearanceCount < 2 {
			continue
		}
		resp.StickyTopVideos = append(resp.StickyTopVideos, stickyTrendVideo{
			ID:            id,
			Appearances:   appearanceCount,
			LongestStreak: longestStreak[id],
			BestRank:      bestRank[id],
			FirstSeen:     firstSeen[id],
			LastSeen:      lastSeen[id],
		})
	}
	sort.Slice(resp.StickyTopVideos, func(i, j int) bool {
		if resp.StickyTopVideos[i].LongestStreak != resp.StickyTopVideos[j].LongestStreak {
			return resp.StickyTopVideos[i].LongestStreak > resp.StickyTopVideos[j].LongestStreak
		}
		if resp.StickyTopVideos[i].Appearances != resp.StickyTopVideos[j].Appearances {
			return resp.StickyTopVideos[i].Appearances > resp.StickyTopVideos[j].Appearances
		}
		return resp.StickyTopVideos[i].ID < resp.StickyTopVideos[j].ID
	})
	resp.SnapshotsCount = len(resp.Snapshots)
	return resp
}

func newSearchTrendsCmd(flags *rootFlags) *cobra.Command {
	var hashtag string
	var history bool

	cmd := &cobra.Command{
		Use:   "trends",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Search a hashtag, record trend snapshots, and inspect stored history",
		Example: `  scrape-creators-pp-cli search trends --hashtag BookTok
  scrape-creators-pp-cli search trends --hashtag BookTok --json
  scrape-creators-pp-cli search trends --hashtag BookTok --history`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := NormalizeHashtag(hashtag)
			if tag == "" {
				return fmt.Errorf("required flag \"hashtag\" not set")
			}

			snapshotFile := searchTrendSnapshotFile(hashtag)
			var snapshots []hashtagTrendSnapshot
			if raw, readErr := os.ReadFile(snapshotFile); readErr == nil {
				json.Unmarshal(raw, &snapshots)
			}

			if history {
				resp := buildTrendHistory(displayHashtag(hashtag), snapshots)
				if len(resp.Snapshots) == 0 {
					if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
						out, _ := json.MarshalIndent(resp, "", "  ")
						fmt.Fprintln(cmd.OutOrStdout(), string(out))
						return nil
					}
					fmt.Fprintf(cmd.OutOrStdout(), "No snapshots yet for %s. Run without --history to record one.\n", resp.Hashtag)
					return nil
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					out, _ := json.MarshalIndent(resp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					return nil
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Trend history for %s (%d snapshots)\n\n", resp.Hashtag, len(resp.Snapshots))
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "DATE\tVIDEOS\tCHANGE\tTREND\tTOP-10 REUSE")
				for _, row := range resp.Snapshots {
					change := "n/a"
					switch {
					case row.Direction == "baseline":
						change = "n/a"
					case row.Delta > 0:
						change = fmt.Sprintf("+%s", formatCount(float64(row.Delta)))
					case row.Delta < 0:
						change = fmt.Sprintf("-%s", formatCount(float64(-row.Delta)))
					default:
						change = "="
					}

					reuse := "n/a"
					if row.Direction != "baseline" {
						reuse = fmt.Sprintf("%d/%d", row.PersistentTopVideos, row.PersistentTopVideos+row.NewTopVideos)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						row.Date,
						formatCount(float64(row.VideoCount)),
						change,
						row.Direction,
						reuse,
					)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				if len(resp.StickyTopVideos) > 0 {
					leaders := make([]string, 0, min(3, len(resp.StickyTopVideos)))
					for _, video := range resp.StickyTopVideos[:min(3, len(resp.StickyTopVideos))] {
						leaders = append(leaders, fmt.Sprintf("%s (%dx, streak %d)", video.ID, video.Appearances, video.LongestStreak))
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nSticky leaders: %s\n", strings.Join(leaders, ", "))
				}
				return nil
			}
			data, err := fetchSearchTrendData(flags, tag)
			if err != nil {
				return classifyAPIError(err)
			}
			var envelope map[string]json.RawMessage
			json.Unmarshal(data, &envelope)

			var videos []map[string]any
			if raw, ok := envelope["aweme_list"]; ok {
				json.Unmarshal(raw, &videos)
			}

			var top []searchTrendTopVideo
			for _, v := range videos {
				play, like, _, _ := videoStats(v)
				id, _ := v["aweme_id"].(string)
				desc, _ := v["desc"].(string)
				top = append(top, searchTrendTopVideo{ID: id, Description: desc, PlayCount: play, LikeCount: like})
			}
			sort.Slice(top, func(i, j int) bool { return top[i].PlayCount > top[j].PlayCount })
			if len(top) > 10 {
				top = top[:10]
			}

			snapshotAt := time.Now().UTC()
			record := hashtagTrendSnapshot{
				Date:       snapshotAt.Format("2006-01-02"),
				SnapshotAt: snapshotAt.Format(time.RFC3339),
				Hashtag:    "#" + tag,
				VideoCount: len(videos),
				TopVideos:  storedTopVideos(top),
			}
			updated := false
			for i := range snapshots {
				if snapshots[i].Date == record.Date {
					snapshots[i] = record
					updated = true
					break
				}
			}
			if !updated {
				snapshots = append(snapshots, record)
			}
			sort.Slice(snapshots, func(i, j int) bool {
				return snapshots[i].Date < snapshots[j].Date
			})
			if persistErr := persistHashtagTrendSnapshots(snapshotFile, snapshots); persistErr != nil && isTerminal(cmd.OutOrStdout()) {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save trend snapshot: %v\n", persistErr)
			}

			result := map[string]any{
				"hashtag":     "#" + tag,
				"video_count": len(videos),
				"snapshot_at": record.SnapshotAt,
				"top_videos":  top,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "#%s — %d videos found\n\nTop by play count:\n", tag, len(videos))
			for i, v := range top {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s  plays=%.0f  likes=%.0f\n",
					i+1, truncate(v.Description, 60), v.PlayCount, v.LikeCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&hashtag, "hashtag", "", "Hashtag to search (with or without #) (required)")
	cmd.Flags().BoolVar(&history, "history", false, "Show recorded hashtag snapshots without making an API call")
	return cmd
}

// ── profile track ────────────────────────────────────────────────────────────

func newTiktokProfileTrackCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var history bool

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Record daily follower snapshots and chart a creator's growth trajectory",
		Long: `Fetches the current follower count for a TikTok creator and stores it as a
dated snapshot. Run daily (e.g. via cron or CI) to build a growth history.
Pass --history to display all recorded snapshots and trend direction.`,
		Example: `  # Record today's snapshot
  scrape-creators-pp-cli tiktok track --handle charlidamelio

  # Show growth history
  scrape-creators-pp-cli tiktok track --handle charlidamelio --history`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handle == "" {
				return fmt.Errorf("--handle is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Where we store snapshots (~/.local/share/scrape-creators-pp-cli/tracking/)
			dbPath := defaultDBPath("scrape-creators-pp-cli")
			trackDir := strings.TrimSuffix(dbPath, "/data.db") + "/tracking"
			if mkErr := os.MkdirAll(trackDir, 0o755); mkErr != nil {
				return fmt.Errorf("creating tracking dir: %w", mkErr)
			}
			snapshotFile := trackDir + "/tiktok-" + strings.ToLower(handle) + ".json"

			// Load existing snapshots
			type snapshot struct {
				Date      string  `json:"date"`
				Followers float64 `json:"followers"`
				Delta     float64 `json:"delta,omitempty"`
			}
			var snapshots []snapshot
			if raw, readErr := os.ReadFile(snapshotFile); readErr == nil {
				json.Unmarshal(raw, &snapshots)
			}

			if history {
				if len(snapshots) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No snapshots yet for @%s. Run without --history to record one.\n", handle)
					return nil
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					out, _ := json.MarshalIndent(snapshots, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Growth history for @%s (%d snapshots)\n\n", handle, len(snapshots))
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %12s  %10s\n", "Date", "Followers", "Change")
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %12s  %10s\n", "────────────", "────────────", "──────────")
				for _, s := range snapshots {
					delta := ""
					if s.Delta > 0 {
						delta = fmt.Sprintf("+%s", formatCount(s.Delta))
					} else if s.Delta < 0 {
						delta = fmt.Sprintf("-%s", formatCount(-s.Delta))
					} else {
						delta = "—"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %12s  %10s\n", s.Date, formatCount(s.Followers), delta)
				}
				return nil
			}

			// Fetch current follower count
			profileData, err := c.Get("/v1/tiktok/profile", map[string]string{"handle": handle})
			if err != nil {
				return classifyAPIError(err)
			}
			var profile map[string]any
			json.Unmarshal(profileData, &profile)
			stats, _ := profile["stats"].(map[string]any)
			if stats == nil {
				stats = map[string]any{}
			}
			followers := 0.0
			if f, ok := stats["followerCount"].(float64); ok {
				followers = f
			}

			today := time.Now().UTC().Format("2006-01-02")

			// Compute delta vs last snapshot
			delta := 0.0
			if len(snapshots) > 0 {
				delta = followers - snapshots[len(snapshots)-1].Followers
			}

			// Upsert today's entry
			updated := false
			for i, s := range snapshots {
				if s.Date == today {
					snapshots[i].Followers = followers
					snapshots[i].Delta = delta
					updated = true
					break
				}
			}
			if !updated {
				snapshots = append(snapshots, snapshot{Date: today, Followers: followers, Delta: delta})
			}

			// Persist
			out, _ := json.MarshalIndent(snapshots, "", "  ")
			if writeErr := os.WriteFile(snapshotFile, out, 0o644); writeErr != nil {
				return fmt.Errorf("saving snapshot: %w", writeErr)
			}

			result := map[string]any{
				"handle":    handle,
				"date":      today,
				"followers": followers,
				"delta":     delta,
				"snapshots": len(snapshots),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				outJSON, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(outJSON))
				return nil
			}
			sign := ""
			if delta > 0 {
				sign = "+"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "@%s: %s followers today", handle, formatCount(followers))
			if len(snapshots) > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s%s vs yesterday)", sign, formatCount(delta))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot saved (%d total). Run --history to see trend.\n", len(snapshots))
			return nil
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "TikTok handle (without @) (required)")
	cmd.Flags().BoolVar(&history, "history", false, "Show all recorded snapshots and growth trend")
	return cmd
}

// ── utils ─────────────────────────────────────────────────────────────────────

func formatCount(f float64) string {
	switch {
	case f >= 1_000_000:
		return fmt.Sprintf("%.1fM", f/1_000_000)
	case f >= 1_000:
		return fmt.Sprintf("%.1fK", f/1_000)
	default:
		return fmt.Sprintf("%.0f", f)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
