package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/quota"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

// newTrendingCmd: parent for `trending snapshot` and `trending diff`.
func newTrendingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Snapshot and diff regional trending videos over time",
	}
	cmd.AddCommand(newTrendingSnapshotCmd(flags))
	cmd.AddCommand(newTrendingDiffCmd(flags))
	return cmd
}

func newTrendingSnapshotCmd(flags *rootFlags) *cobra.Command {
	var region, category string
	var maxResults int
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture today's mostPopular trending list to local store",
		Long: `Pulls videos.list?chart=mostPopular for a region (and optional category)
and writes the position-ranked list to yt_trending_snapshots. Run periodically
(e.g. via cron) to build a history that ` + "`trending diff`" + ` can compare.`,
		Example: "  youtube-pp-cli trending snapshot --region US --max-results 50",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cfg, _ := config.Load(flags.configPath)
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}

			params := map[string]string{
				"part":       "snippet,statistics",
				"chart":      "mostPopular",
				"regionCode": region,
				"maxResults": fmt.Sprintf("%d", maxResults),
			}
			if category != "" {
				params["videoCategoryId"] = category
			}
			raw, err := c.Get("/youtube/v3/videos", params)
			if err != nil {
				return fmt.Errorf("trending fetch: %w", err)
			}
			_ = db.LogQuota(cmd.Context(), hashKeyFromConfig(cfg), "trending snapshot", "videos.list", quota.Cost("videos", "list"), 200, "")
			var resp struct {
				Items []struct {
					ID      string `json:"id"`
					Snippet struct {
						Title        string `json:"title"`
						ChannelID    string `json:"channelId"`
						ChannelTitle string `json:"channelTitle"`
					} `json:"snippet"`
					Statistics struct {
						ViewCount string `json:"viewCount"`
					} `json:"statistics"`
				} `json:"items"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return err
			}
			capturedAt := time.Now().UTC().Format(time.RFC3339)
			catKey := category
			if catKey == "" {
				catKey = "0"
			}
			for i, it := range resp.Items {
				views, _ := parseInt(it.Statistics.ViewCount)
				_, _ = db.DB().ExecContext(cmd.Context(),
					`INSERT INTO yt_trending_snapshots(region, category_id, captured_at, position, video_id, title, channel_id, channel_title, view_count)
					 VALUES(?,?,?,?,?,?,?,?,?)
					 ON CONFLICT DO NOTHING`,
					region, catKey, capturedAt, i+1, it.ID, it.Snippet.Title, it.Snippet.ChannelID, it.Snippet.ChannelTitle, views)
			}
			env := map[string]any{
				"region":      region,
				"category":    catKey,
				"captured_at": capturedAt,
				"items":       len(resp.Items),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&region, "region", "US", "ISO 3166-1 alpha-2 region code")
	cmd.Flags().StringVar(&category, "category", "", "Video category ID (default: all categories)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Top N trending videos to capture (max 50)")
	return cmd
}

func newTrendingDiffCmd(flags *rootFlags) *cobra.Command {
	var region, category, since string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff today's trending snapshot against an earlier date",
		Long: `Compare the latest trending snapshot for a (region, category) against the
snapshot closest to --since and emit entered/exited/moved buckets.

Requires at least 2 snapshots — capture them with ` + "`trending snapshot`" + `
on a recurring schedule first.`,
		Example:     "  youtube-pp-cli trending diff --region US --since 2026-04-01 --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			catKey := category
			if catKey == "" {
				catKey = "0"
			}
			latest, err := pickSnapshot(cmd.Context(), db.DB(), region, catKey, time.Now())
			if err != nil {
				return err
			}
			cutoff, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			earlier, err := pickSnapshot(cmd.Context(), db.DB(), region, catKey, cutoff)
			if err != nil {
				return err
			}
			if latest == "" || earlier == "" || latest == earlier {
				return fmt.Errorf("need at least 2 distinct snapshots for region=%s category=%s; have latest=%q earlier=%q. Run `trending snapshot` first", region, catKey, latest, earlier)
			}
			latestRows, err := loadSnapshotRows(cmd.Context(), db.DB(), region, catKey, latest)
			if err != nil {
				return err
			}
			earlierRows, err := loadSnapshotRows(cmd.Context(), db.DB(), region, catKey, earlier)
			if err != nil {
				return err
			}
			diff := buildTrendingDiff(latestRows, earlierRows)
			env := map[string]any{
				"region":          region,
				"category":        catKey,
				"latest_capture":  latest,
				"earlier_capture": earlier,
				"diff":            diff,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&region, "region", "US", "Region code")
	cmd.Flags().StringVar(&category, "category", "", "Category ID")
	cmd.Flags().StringVar(&since, "since", "", "Compare against the snapshot closest to this date (YYYY-MM-DD or window like 7d). Default: 7 days ago.")
	return cmd
}

type trendingRow struct {
	Position     int
	VideoID      string
	Title        string
	ChannelID    string
	ChannelTitle string
	ViewCount    int
}

func loadSnapshotRows(ctx context.Context, db *sql.DB, region, category, capturedAt string) ([]trendingRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT position, video_id, title, channel_id, channel_title, view_count
		   FROM yt_trending_snapshots
		   WHERE region = ? AND category_id = ? AND captured_at = ?
		   ORDER BY position ASC`, region, category, capturedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []trendingRow
	for rows.Next() {
		var r trendingRow
		var title, ch, chTitle sql.NullString
		var views sql.NullInt64
		if err := rows.Scan(&r.Position, &r.VideoID, &title, &ch, &chTitle, &views); err != nil {
			return nil, err
		}
		r.Title = title.String
		r.ChannelID = ch.String
		r.ChannelTitle = chTitle.String
		r.ViewCount = int(views.Int64)
		out = append(out, r)
	}
	return out, nil
}

func pickSnapshot(ctx context.Context, db *sql.DB, region, category string, target time.Time) (string, error) {
	row := db.QueryRowContext(ctx,
		`SELECT captured_at FROM yt_trending_snapshots
		   WHERE region = ? AND category_id = ?
		   ORDER BY ABS(strftime('%s', captured_at) - strftime('%s', ?)) ASC LIMIT 1`,
		region, category, target.UTC().Format(time.RFC3339))
	var capturedAt sql.NullString
	if err := row.Scan(&capturedAt); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return capturedAt.String, nil
}

// buildTrendingDiff produces entered/exited/moved buckets from two ordered
// trending lists.
func buildTrendingDiff(latest, earlier []trendingRow) map[string]any {
	earlierByID := map[string]trendingRow{}
	for _, r := range earlier {
		earlierByID[r.VideoID] = r
	}
	latestByID := map[string]trendingRow{}
	for _, r := range latest {
		latestByID[r.VideoID] = r
	}
	var entered, moved []map[string]any
	var exited []map[string]any
	for _, r := range latest {
		if old, ok := earlierByID[r.VideoID]; ok {
			if old.Position != r.Position {
				moved = append(moved, map[string]any{
					"video_id":       r.VideoID,
					"title":          r.Title,
					"channel_title":  r.ChannelTitle,
					"old_position":   old.Position,
					"new_position":   r.Position,
					"position_delta": old.Position - r.Position,
				})
			}
		} else {
			entered = append(entered, map[string]any{
				"video_id":      r.VideoID,
				"title":         r.Title,
				"channel_title": r.ChannelTitle,
				"position":      r.Position,
			})
		}
	}
	for _, r := range earlier {
		if _, ok := latestByID[r.VideoID]; !ok {
			exited = append(exited, map[string]any{
				"video_id":      r.VideoID,
				"title":         r.Title,
				"channel_title": r.ChannelTitle,
				"old_position":  r.Position,
			})
		}
	}
	return map[string]any{
		"entered": entered,
		"exited":  exited,
		"moved":   moved,
	}
}

func parseSinceDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now().Add(-7 * 24 * time.Hour), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return parseWindow(s)
}

// newVelocityCmd: per-video Δ over a rolling window.
func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var window string
	var orderBy string
	var limit int
	cmd := &cobra.Command{
		Use:   "velocity <channel-id>",
		Short: "Per-video view/like/comment velocity from local snapshot history",
		Long: `Compute Δviews/Δlikes/Δcomments per day per video using yt_video_snapshots
captured by sync-channel. Requires at least 2 snapshots to produce a delta.`,
		Example:     "  youtube-pp-cli velocity UCBJycsmduvYEL83R_U4JriQ --window 30d --order delta_views --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			channelID := args[0]
			cutoff, err := parseWindow(window)
			if err != nil {
				return err
			}
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT video_id, captured_at, title, view_count, like_count, comment_count
				   FROM yt_video_snapshots
				   WHERE channel_id = ? AND captured_at >= ?
				   ORDER BY video_id, captured_at ASC`,
				channelID, cutoff.UTC().Format(time.RFC3339))
			if err != nil {
				return err
			}
			defer rows.Close()
			type snap struct {
				At                     time.Time
				Views, Likes, Comments int
				Title                  string
			}
			perVideo := map[string][]snap{}
			titles := map[string]string{}
			for rows.Next() {
				var vid, at, title sql.NullString
				var views, likes, comments sql.NullInt64
				if err := rows.Scan(&vid, &at, &title, &views, &likes, &comments); err != nil {
					return err
				}
				ts, _ := time.Parse(time.RFC3339, at.String)
				perVideo[vid.String] = append(perVideo[vid.String], snap{
					At: ts, Views: int(views.Int64), Likes: int(likes.Int64), Comments: int(comments.Int64), Title: title.String,
				})
				titles[vid.String] = title.String
			}
			result := []map[string]any{}
			for vid, snaps := range perVideo {
				if len(snaps) < 2 {
					continue
				}
				first := snaps[0]
				last := snaps[len(snaps)-1]
				dur := last.At.Sub(first.At).Hours() / 24
				if dur <= 0 {
					continue
				}
				result = append(result, map[string]any{
					"video_id":         vid,
					"title":            titles[vid],
					"snapshots":        len(snaps),
					"window_days":      dur,
					"delta_views":      last.Views - first.Views,
					"delta_likes":      last.Likes - first.Likes,
					"delta_comments":   last.Comments - first.Comments,
					"views_per_day":    float64(last.Views-first.Views) / dur,
					"likes_per_day":    float64(last.Likes-first.Likes) / dur,
					"comments_per_day": float64(last.Comments-first.Comments) / dur,
				})
			}
			sortVelocity(result, orderBy)
			if limit > 0 && len(result) > limit {
				result = result[:limit]
			}
			env := map[string]any{
				"channel_id": channelID,
				"window":     window,
				"order":      orderBy,
				"videos":     result,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 30d, 24h)")
	cmd.Flags().StringVar(&orderBy, "order", "views_per_day", "Order key: views_per_day | delta_views | likes_per_day | delta_likes")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max videos to return")
	return cmd
}

func sortVelocity(rows []map[string]any, key string) {
	sort.SliceStable(rows, func(i, j int) bool {
		return toFloat(rows[i][key]) > toFloat(rows[j][key])
	})
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// newTopicCrossoverCmd: 3-way join trending × videos × transcripts.
// Flat single command (use Cobra's "topic crossover" via direct AddCommand
// of the parent without an intermediate "topic" container that would
// require flag inheritance).
func newTopicCrossoverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Topic-driven analysis across local store",
	}
	cmd.AddCommand(newTopicCrossoverLeafCmd(flags))
	return cmd
}

func newTopicCrossoverLeafCmd(flags *rootFlags) *cobra.Command {
	var regions string
	var category string
	var limit int
	cmd := &cobra.Command{
		Use:   "crossover <keyword>",
		Short: "Find trending videos whose title/description/transcript mentions a keyword",
		Long: `Three-way join: yt_trending_snapshots × yt_videos × yt_transcripts.
Returns videos in the latest trending snapshot per region whose metadata
or transcript matches the keyword, ordered by trending position.`,
		Example:     "  youtube-pp-cli topic crossover \"AI safety\" --regions US,GB --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			keyword := strings.Join(args, " ")
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			catKey := category
			if catKey == "" {
				catKey = "0"
			}
			regionList := []string{"US"}
			if r := strings.TrimSpace(regions); r != "" {
				regionList = strings.Split(r, ",")
			}
			results := []map[string]any{}
			for _, region := range regionList {
				region = strings.TrimSpace(region)
				latest, err := pickSnapshot(cmd.Context(), db.DB(), region, catKey, time.Now())
				if err != nil || latest == "" {
					continue
				}
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT t.position, t.video_id, t.title, t.channel_title
					   FROM yt_trending_snapshots t
					   WHERE t.region = ? AND t.category_id = ? AND t.captured_at = ?
					   ORDER BY t.position ASC`,
					region, catKey, latest)
				if err != nil {
					continue
				}
				for rows.Next() {
					var pos int
					var vid, title, chTitle sql.NullString
					if err := rows.Scan(&pos, &vid, &title, &chTitle); err != nil {
						continue
					}
					if !mentionsKeyword(cmd.Context(), db.DB(), vid.String, keyword) {
						continue
					}
					results = append(results, map[string]any{
						"region":        region,
						"trending_rank": pos,
						"video_id":      vid.String,
						"title":         title.String,
						"channel_title": chTitle.String,
						"captured_at":   latest,
					})
				}
				rows.Close()
			}
			sort.SliceStable(results, func(i, j int) bool {
				return toFloat(results[i]["trending_rank"]) < toFloat(results[j]["trending_rank"])
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			env := map[string]any{
				"keyword":  keyword,
				"regions":  regionList,
				"category": catKey,
				"results":  results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&regions, "regions", "US", "Comma list of region codes to scan")
	cmd.Flags().StringVar(&category, "category", "", "Category ID (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to return")
	return cmd
}

func mentionsKeyword(ctx context.Context, db *sql.DB, videoID, keyword string) bool {
	q := ftsQuote(keyword)
	var n int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM yt_videos_fts WHERE video_id = ? AND yt_videos_fts MATCH ?`,
		videoID, q).Scan(&n)
	if n > 0 {
		return true
	}
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM yt_transcripts_fts WHERE video_id = ? AND yt_transcripts_fts MATCH ?`,
		videoID, q).Scan(&n)
	return n > 0
}

var _ = json.RawMessage(nil)
