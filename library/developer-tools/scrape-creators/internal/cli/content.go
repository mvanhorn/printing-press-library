package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newContentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "content",
		Short: "Engagement analytics over locally synced content",
	}
	cmd.AddCommand(newContentSpikesCmd(flags))
	cmd.AddCommand(newContentAnalyzeCmd(flags))
	cmd.AddCommand(newContentCadenceCmd(flags))
	return cmd
}

// contentRow is a normalized row pulled from any per-platform content JSON.
type contentRow struct {
	ID         string  `json:"id"`
	Platform   string  `json:"platform"`
	Handle     string  `json:"handle"`
	URL        string  `json:"url,omitempty"`
	Views      int64   `json:"views"`
	Likes      int64   `json:"likes"`
	Comments   int64   `json:"comments"`
	Shares     int64   `json:"shares,omitempty"`
	Engagement float64 `json:"engagement,omitempty"`
	CreatedAt  string  `json:"created_at,omitempty"`
}

func loadContent(ctx interface{ Done() <-chan struct{} }, db *sql.DB, platform, handle string, limit int) ([]contentRow, error) {
	tables := []string{"tiktok", "youtube", "instagram", "facebook", "twitter"}
	if platform != "" && platform != "all" {
		tables = []string{platform}
	}
	var out []contentRow
	for _, t := range tables {
		q := `SELECT id, data FROM ` + t + ` WHERE handle = ? OR data LIKE ? LIMIT ?`
		rows, err := db.Query(q, handle, "%"+handle+"%", limit)
		if err != nil {
			continue // table absent
		}
		for rows.Next() {
			var id, data string
			if err := rows.Scan(&id, &data); err != nil {
				continue
			}
			var d map[string]any
			if json.Unmarshal([]byte(data), &d) != nil {
				continue
			}
			row := contentRow{ID: id, Platform: t, Handle: handle}
			row.Views = lookupInt(d, []string{"playCount", "view_count", "viewCount", "stats.playCount", "video_view_count", "play_count"})
			row.Likes = lookupInt(d, []string{"diggCount", "like_count", "likeCount", "stats.diggCount", "favorite_count"})
			row.Comments = lookupInt(d, []string{"commentCount", "comment_count", "stats.commentCount"})
			row.Shares = lookupInt(d, []string{"shareCount", "share_count", "stats.shareCount", "retweet_count"})
			row.URL = lookupString(d, []string{"url", "video_url", "share_url", "permalink"})
			row.CreatedAt = lookupString(d, []string{"createTime", "created_at", "create_time", "publish_time"})
			if row.Views > 0 {
				row.Engagement = float64(row.Likes+row.Comments+row.Shares) / float64(row.Views)
			}
			if row.Views > 0 || row.Likes > 0 || row.Comments > 0 {
				out = append(out, row)
			}
		}
		rows.Close()
	}
	return out, nil
}

func newContentSpikesCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var threshold float64
	var limit int
	cmd := &cobra.Command{
		Use:   "spikes <handle>",
		Short: "Find videos that performed at threshold× the creator's average",
		Long: `Reads synced content from the local store, computes the creator's average
engagement (views, likes, comments, shares), and returns rows where
engagement is at least --threshold× that average.

Sync first so the local store has content to analyze: e.g.
'scrape-creators-pp-cli tiktok list-profile-2 --handle <h>'.`,
		Example:     "  scrape-creators-pp-cli content spikes mrbeast --threshold 2.0 --platform tiktok --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			handle := strings.TrimPrefix(args[0], "@")
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := loadContent(cmd.Context(), s.DB(), platform, handle, 10000)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []contentRow{}, flags)
			}
			var totalViews, totalLikes int64
			for _, r := range rows {
				totalViews += r.Views
				totalLikes += r.Likes
			}
			avgViews := float64(totalViews) / float64(len(rows))
			var spikes []contentRow
			for _, r := range rows {
				if avgViews > 0 && float64(r.Views)/avgViews >= threshold {
					spikes = append(spikes, r)
				}
			}
			sort.Slice(spikes, func(i, j int) bool { return spikes[i].Views > spikes[j].Views })
			if len(spikes) > limit {
				spikes = spikes[:limit]
			}
			result := map[string]any{
				"handle":      handle,
				"platform":    platform,
				"corpus_size": len(rows),
				"avg_views":   avgViews,
				"threshold":   threshold,
				"spike_count": len(spikes),
				"spikes":      spikes,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spikes for %s on %s — corpus %d, avg views %.0f, threshold %.1fx\n", handle, platform, len(rows), avgViews, threshold)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tVIEWS\tLIKES\tCOMMENTS\tx-AVG")
			for _, r := range spikes {
				ratio := float64(r.Views) / avgViews
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1fx\n", truncate(r.ID, 16), r.Views, r.Likes, r.Comments, ratio)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to analyze")
	cmd.Flags().Float64Var(&threshold, "threshold", 2.0, "Minimum view ratio vs creator average to count as a spike")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum spike rows to return")
	return cmd
}

func newContentAnalyzeCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{
		Use:         "analyze <handle>",
		Short:       "Rank synced content by engagement rate (likes+comments+shares)/views",
		Example:     "  scrape-creators-pp-cli content analyze mrbeast --platform youtube --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			handle := strings.TrimPrefix(args[0], "@")
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := loadContent(cmd.Context(), s.DB(), platform, handle, 10000)
			if err != nil {
				return err
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Engagement > rows[j].Engagement })
			if len(rows) > limit {
				rows = rows[:limit]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tVIEWS\tLIKES\tCOMMENTS\tENGAGEMENT")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.4f\n", truncate(r.ID, 16), r.Views, r.Likes, r.Comments, r.Engagement)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to analyze")
	cmd.Flags().IntVar(&limit, "limit", 25, "Top N rows to return")
	return cmd
}

func newContentCadenceCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "cadence <handle>",
		Short:       "Posting frequency by day of week and hour",
		Example:     "  scrape-creators-pp-cli content cadence mrbeast --platform tiktok --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			handle := strings.TrimPrefix(args[0], "@")
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := loadContent(cmd.Context(), s.DB(), platform, handle, 10000)
			if err != nil {
				return err
			}
			byDow := map[string]int{}
			byHour := map[int]int{}
			for _, r := range rows {
				if r.CreatedAt == "" {
					continue
				}
				ts := parseTimeFlexible(r.CreatedAt)
				if ts.IsZero() {
					continue
				}
				byDow[ts.Weekday().String()]++
				byHour[ts.Hour()]++
			}
			result := map[string]any{
				"handle":      handle,
				"platform":    platform,
				"corpus_size": len(rows),
				"by_dow":      byDow,
				"by_hour":     byHour,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DAY\tCOUNT")
			for _, dow := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
				fmt.Fprintf(tw, "%s\t%d\n", dow, byDow[dow])
			}
			tw.Flush()
			fmt.Fprintln(cmd.OutOrStdout(), "")
			tw = newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "HOUR\tCOUNT")
			for h := 0; h < 24; h++ {
				fmt.Fprintf(tw, "%02d\t%d\n", h, byHour[h])
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to analyze")
	return cmd
}
