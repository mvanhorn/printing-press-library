// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel analytics: posts engagement + posts type-compare.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// newPostsCmd returns the `posts` top-level analytics command.
func newPostsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "posts",
		Short: "Post analytics (powered by local SQLite — run sync media first)",
	}
	cmd.AddCommand(newPostsEngagementCmd(flags))
	cmd.AddCommand(newPostsTypeCompareCmd(flags))
	return cmd
}

// parseSinceDays parses a --since flag like "30d", "7d", "90d" and returns the
// cutoff Unix timestamp, or 0 if since is empty.
func parseSinceDays(since string) (int64, error) {
	if since == "" {
		return 0, nil
	}
	s := since
	if len(s) > 1 && s[len(s)-1] == 'd' {
		s = s[:len(s)-1]
	}
	days, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since value %q: expected format like 30d or 90d", since)
	}
	return time.Now().AddDate(0, 0, -days).Unix(), nil
}

func newPostsEngagementCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "engagement",
		Short: "Rank posts by engagement rate (likes+comments; run sync media first)",
		Long: `Queries the local SQLite store and ranks synced posts by engagement.
Engagement is computed as (like_count + comment_count) — absolute rather than
rate-adjusted because follower count is not always available in post records.

To compute rate-adjusted engagement, sync the account profile first:
  instagram-pp-cli users profile --username <user>

Then this command will use the follower_count from the synced profile if available.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli posts engagement
  instagram-pp-cli posts engagement --since 30d --limit 20
  instagram-pp-cli posts engagement --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			cutoff, err := parseSinceDays(since)
			if err != nil {
				return err
			}

			query := `
				SELECT
					json_extract(data, '$.pk')                   AS id,
					json_extract(data, '$.caption.text')          AS caption,
					COALESCE(json_extract(data, '$.like_count'), 0)    AS likes,
					COALESCE(json_extract(data, '$.comment_count'), 0) AS comments,
					json_extract(data, '$.media_type')            AS media_type,
					COALESCE(json_extract(data, '$.taken_at'), 0) AS taken_at
				FROM resources
				WHERE resource_type = 'media'
			`
			var queryArgs []any
			if cutoff > 0 {
				query += " AND CAST(json_extract(data, '$.taken_at') AS INTEGER) >= ?"
				queryArgs = append(queryArgs, cutoff)
			}
			query += " ORDER BY (CAST(json_extract(data, '$.like_count') AS INTEGER) + CAST(json_extract(data, '$.comment_count') AS INTEGER)) DESC"
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type postRow struct {
				ID          string `json:"id"`
				Caption     string `json:"caption"`
				Likes       int64  `json:"likes"`
				Comments    int64  `json:"comments"`
				Engagement  int64  `json:"engagement"`
				MediaType   string `json:"media_type"`
				TakenAt     string `json:"taken_at"`
			}

			var posts []postRow
			for rows.Next() {
				var r postRow
				var captionNull *string
				var takenAt int64
				var mediaType *string
				if err := rows.Scan(&r.ID, &captionNull, &r.Likes, &r.Comments, &mediaType, &takenAt); err != nil {
					continue
				}
				if captionNull != nil {
					r.Caption = *captionNull
				}
				if mediaType != nil {
					r.MediaType = *mediaType
				}
				r.Engagement = r.Likes + r.Comments
				if takenAt > 0 {
					r.TakenAt = time.Unix(takenAt, 0).Format("2006-01-02")
				}
				if len(r.Caption) > 80 {
					r.Caption = r.Caption[:80] + "…"
				}
				posts = append(posts, r)
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"count": len(posts),
					"posts": func() any { if posts == nil { return []postRow{} }; return posts }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Posts ranked by engagement (likes+comments): %d\n\n", len(posts))
			if len(posts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No posts found. Run: instagram-pp-cli sync media <username>")
				return nil
			}

			tableRows := make([][]string, len(posts))
			for i, p := range posts {
				caption := p.Caption
				if len(caption) > 50 {
					caption = caption[:50] + "…"
				}
				tableRows[i] = []string{
					p.TakenAt,
					p.MediaType,
					strconv.FormatInt(p.Likes, 10),
					strconv.FormatInt(p.Comments, 10),
					strconv.FormatInt(p.Engagement, 10),
					caption,
				}
			}
			return flags.printTable(cmd, []string{"DATE", "TYPE", "LIKES", "COMMENTS", "TOTAL", "CAPTION"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Only include posts newer than this duration (e.g. 30d, 90d)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to show (0 = unlimited)")
	return cmd
}

func newPostsTypeCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	cmd := &cobra.Command{
		Use:   "type-compare",
		Short: "Compare engagement by content type: carousel vs reel vs photo (run sync media first)",
		Long: `Groups synced posts by media_type (1=photo, 2=video/reel, 8=carousel) and
computes average likes and comments per type. This answers the A/B question —
which content format performs best for this account.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli posts type-compare
  instagram-pp-cli posts type-compare --since 90d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			cutoff, err := parseSinceDays(since)
			if err != nil {
				return err
			}

			query := `
				SELECT
					CASE json_extract(data, '$.media_type')
						WHEN 1 THEN 'photo'
						WHEN 2 THEN 'video/reel'
						WHEN 8 THEN 'carousel'
						ELSE 'unknown(' || json_extract(data, '$.media_type') || ')'
					END AS type_label,
					COUNT(*) AS post_count,
					ROUND(AVG(COALESCE(json_extract(data,'$.like_count'), 0)), 1)    AS avg_likes,
					ROUND(AVG(COALESCE(json_extract(data,'$.comment_count'), 0)), 1) AS avg_comments,
					ROUND(AVG(COALESCE(json_extract(data,'$.like_count'), 0) +
					          COALESCE(json_extract(data,'$.comment_count'), 0)), 1) AS avg_engagement
				FROM resources
				WHERE resource_type = 'media'
			`
			var queryArgs []any
			if cutoff > 0 {
				query += " AND CAST(json_extract(data, '$.taken_at') AS INTEGER) >= ?"
				queryArgs = append(queryArgs, cutoff)
			}
			query += " GROUP BY json_extract(data, '$.media_type') ORDER BY avg_engagement DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type typeRow struct {
				TypeLabel     string  `json:"type"`
				PostCount     int64   `json:"post_count"`
				AvgLikes      float64 `json:"avg_likes"`
				AvgComments   float64 `json:"avg_comments"`
				AvgEngagement float64 `json:"avg_engagement"`
			}
			var types []typeRow
			for rows.Next() {
				var r typeRow
				if err := rows.Scan(&r.TypeLabel, &r.PostCount, &r.AvgLikes, &r.AvgComments, &r.AvgEngagement); err == nil {
					types = append(types, r)
				}
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"count": len(types),
					"types": func() any { if types == nil { return []typeRow{} }; return types }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Engagement by content type:\n\n")
			if len(types) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No posts found. Run: instagram-pp-cli sync media <username>")
				return nil
			}
			tableRows := make([][]string, len(types))
			for i, t := range types {
				tableRows[i] = []string{
					t.TypeLabel,
					strconv.FormatInt(t.PostCount, 10),
					strconv.FormatFloat(t.AvgLikes, 'f', 1, 64),
					strconv.FormatFloat(t.AvgComments, 'f', 1, 64),
					strconv.FormatFloat(t.AvgEngagement, 'f', 1, 64),
				}
			}
			return flags.printTable(cmd, []string{"TYPE", "POSTS", "AVG LIKES", "AVG COMMENTS", "AVG ENGAGEMENT"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Only include posts newer than this duration (e.g. 30d, 90d)")
	return cmd
}
