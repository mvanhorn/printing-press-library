// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel analytics: hashtag performance.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

var hashtagRe = regexp.MustCompile(`#([a-zA-Z0-9_]+)`)

// newHashtagAnalyticsCmd returns the `hashtag` top-level analytics command.
func newHashtagAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hashtag",
		Short: "Hashtag analytics (powered by local SQLite — run sync media first)",
	}
	cmd.AddCommand(newHashtagPerformanceCmd(flags))
	return cmd
}

func newHashtagPerformanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Correlate hashtags with engagement on synced posts",
		Long: `Parses hashtags from post captions in the local SQLite store, then joins with
each post's like_count and comment_count to show which hashtags appear in
above-average-performing posts.

No Instagram API exposes this correlation. Results are computed entirely from
synced post data. Run 'sync media <username>' first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli hashtag performance
  instagram-pp-cli hashtag performance --since 90d --limit 20
  instagram-pp-cli hashtag performance --json`,
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

			// Fetch all posts from store
			query := `
				SELECT
					COALESCE(json_extract(data, '$.caption.text'), '')    AS caption,
					COALESCE(json_extract(data, '$.like_count'), 0)       AS likes,
					COALESCE(json_extract(data, '$.comment_count'), 0)    AS comments,
					COALESCE(json_extract(data, '$.taken_at'), 0)         AS taken_at
				FROM resources
				WHERE resource_type = 'media'
			`
			var queryArgs []any
			if cutoff > 0 {
				query += " AND CAST(json_extract(data, '$.taken_at') AS INTEGER) >= ?"
				queryArgs = append(queryArgs, cutoff)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type tagStats struct {
				Tag       string
				PostCount int
				TotalEng  int64
			}
			tagMap := map[string]*tagStats{}
			var totalEng int64
			var totalPosts int

			for rows.Next() {
				var caption string
				var likes, comments int64
				var takenAt int64
				if err := rows.Scan(&caption, &likes, &comments, &takenAt); err != nil {
					continue
				}
				eng := likes + comments
				totalEng += eng
				totalPosts++

				tags := hashtagRe.FindAllStringSubmatch(caption, -1)
				seen := map[string]bool{}
				for _, match := range tags {
					tag := strings.ToLower(match[1])
					if seen[tag] {
						continue
					}
					seen[tag] = true
					if _, ok := tagMap[tag]; !ok {
						tagMap[tag] = &tagStats{Tag: tag}
					}
					tagMap[tag].PostCount++
					tagMap[tag].TotalEng += eng
				}
			}

			if totalPosts == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No posts found. Run: instagram-pp-cli sync media <username>")
				return nil
			}

			avgEng := float64(totalEng) / float64(totalPosts)

			type resultRow struct {
				Tag     string  `json:"tag"`
				Posts   int     `json:"posts"`
				AvgEng  float64 `json:"avg_engagement"`
				VsAvg   float64 `json:"vs_avg_pct"`
			}
			var results []resultRow
			for _, s := range tagMap {
				if s.PostCount < 2 {
					continue // skip one-off tags
				}
				tagAvg := float64(s.TotalEng) / float64(s.PostCount)
				var vsAvg float64
				if avgEng > 0 {
					vsAvg = (tagAvg/avgEng - 1) * 100
				}
				results = append(results, resultRow{
					Tag:    s.Tag,
					Posts:  s.PostCount,
					AvgEng: tagAvg,
					VsAvg:  vsAvg,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i].AvgEng > results[j].AvgEng
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"total_posts":      totalPosts,
					"overall_avg_eng":  avgEng,
					"count":            len(results),
					"hashtags":         func() any { if results == nil { return []resultRow{} }; return results }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Hashtag performance across %d posts (overall avg engagement: %.1f):\n\n",
				totalPosts, avgEng)
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hashtags found (need ≥2 posts with a tag to compute averages).")
				return nil
			}

			tableRows := make([][]string, len(results))
			for i, r := range results {
				sign := "+"
				if r.VsAvg < 0 {
					sign = ""
				}
				tableRows[i] = []string{
					"#" + r.Tag,
					strconv.Itoa(r.Posts),
					strconv.FormatFloat(r.AvgEng, 'f', 1, 64),
					fmt.Sprintf("%s%.0f%%", sign, r.VsAvg),
				}
			}
			return flags.printTable(cmd, []string{"HASHTAG", "POSTS", "AVG ENG", "VS ACCOUNT AVG"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "", "Only include posts newer than this duration (e.g. 30d, 90d)")
	cmd.Flags().IntVar(&limit, "limit", 30, "Max hashtags to show (0 = unlimited)")
	return cmd
}
