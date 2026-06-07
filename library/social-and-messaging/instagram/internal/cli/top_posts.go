// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type topPostRow struct {
	Slug             string `json:"slug"`
	MediaID          string `json:"media_id"`
	Permalink        string `json:"permalink"`
	MediaProductType string `json:"media_product_type"`
	PostedAt         string `json:"posted_at"`
	Metric           int64  `json:"metric"`
	MetricName       string `json:"metric_name"`
	LikeCount        int64  `json:"like_count"`
	CommentsCount    int64  `json:"comments_count"`
	Caption          string `json:"caption"`
}

// topPostMetricColumns whitelists the rankable metric -> column so the
// metric name can be safely interpolated into the ORDER BY / SELECT.
var topPostMetricColumns = map[string]string{
	"reach":        "reach",
	"interactions": "total_interactions",
	"saved":        "saved",
	"shares":       "shares",
	"views":        "views",
}

func newNovelTopPostsCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceFlag, metricFlag, accountFlag string
	var limitFlag int

	cmd := &cobra.Command{
		Use:   "top-posts",
		Short: "Rank individual posts across your brands by reach, interactions, saves, or shares over a window.",
		Long: `Rank stored media within --since by the chosen --metric (reach,
interactions, saved, shares, or views). Captions are truncated to 60 chars.

Reads the local store. Run 'instagram-pp-cli pull' first to populate media.`,
		Example:     "  instagram-pp-cli top-posts --since 30d --metric reach --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank top posts from local store")
				return nil
			}
			window, err := parseLooseDuration(sinceFlag)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", sinceFlag, err))
			}
			metric := strings.ToLower(strings.TrimSpace(metricFlag))
			col, ok := topPostMetricColumns[metric]
			if !ok {
				return usageErr(fmt.Errorf("invalid --metric %q: use reach, interactions, saved, shares, or views", metricFlag))
			}
			if limitFlag <= 0 {
				limitFlag = 10
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().Add(-window).Format(time.RFC3339)
			query := fmt.Sprintf(`
				SELECT slug, media_id, COALESCE(permalink,''), COALESCE(media_product_type,''),
				       COALESCE(posted_at,''), COALESCE(%s,0),
				       COALESCE(like_count,0), COALESCE(comments_count,0), COALESCE(caption,'')
				FROM ig_brand_media
				WHERE COALESCE(posted_at,'') >= ?`, col)
			args2 := []any{cutoff}
			if accountFlag != "" {
				query += ` AND slug = ?`
				args2 = append(args2, slugify(accountFlag))
			}
			query += fmt.Sprintf(` ORDER BY COALESCE(%s,0) DESC, media_id LIMIT ?`, col)
			args2 = append(args2, limitFlag)

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			out := make([]topPostRow, 0)
			for rows.Next() {
				var r topPostRow
				if err := rows.Scan(&r.Slug, &r.MediaID, &r.Permalink, &r.MediaProductType, &r.PostedAt, &r.Metric, &r.LikeCount, &r.CommentsCount, &r.Caption); err != nil {
					return apiErr(err)
				}
				r.MetricName = metric
				r.Caption = truncateCaption(r.Caption, 60)
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				env := map[string]any{"posts": out, "count": len(out), "metric": metric}
				if len(out) == 0 {
					env["note"] = "no media in window; run 'instagram-pp-cli pull' first"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No media in window. Run 'instagram-pp-cli pull' first.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "SLUG\tTYPE\tPOSTED_AT\t%s\tLIKES\tCOMMENTS\tCAPTION\n", strings.ToUpper(metric))
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%s\n", r.Slug, r.MediaProductType, r.PostedAt, r.Metric, r.LikeCount, r.CommentsCount, r.Caption)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&sinceFlag, "since", "30d", "Only consider media newer than this (e.g. 7d, 8w, 24h)")
	cmd.Flags().StringVar(&metricFlag, "metric", "reach", "Ranking metric: reach, interactions, saved, shares, or views")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit to a single brand slug")
	cmd.Flags().IntVar(&limitFlag, "limit", 10, "Maximum posts to return")
	return cmd
}

func truncateCaption(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\t", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
