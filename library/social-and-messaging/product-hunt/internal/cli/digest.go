// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/store"
	"github.com/spf13/cobra"
)

type digestPost struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Tagline       string `json:"tagline"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	URL           string `json:"url,omitempty"`
}

type digestTopicGroup struct {
	Topic string       `json:"topic"`
	Posts []digestPost `json:"posts"`
}

type digestResult struct {
	Date       string             `json:"date"`
	TotalPosts int                `json:"totalPosts"`
	Topics     []digestTopicGroup `json:"topics"`
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var dateStr string
	var limit int
	var yesterday bool

	cmd := &cobra.Command{
		Use:   "digest [topic]",
		Short: "Daily digest of top launches per topic, works offline after sync",
		Long: `Groups the top posts from a specific date by topic and shows them as a
structured digest. Works fully offline from the local store — no API call
needed after syncing.

Default date is today (UTC). Use --yesterday or --date to view other days.
Pass an optional topic slug to limit the digest to that topic.`,
		Example: strings.Trim(`
  product-hunt-pp-cli digest
  product-hunt-pp-cli digest --yesterday --json
  product-hunt-pp-cli digest developer-tools --yesterday --json
  product-hunt-pp-cli digest --date 2026-05-08 --limit 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Optional topic filter from positional arg
			topicFilter := ""
			if len(args) > 0 {
				topicFilter = strings.ToLower(strings.TrimSpace(args[0]))
			}

			// Parse target date
			var targetDate time.Time
			if yesterday {
				targetDate = time.Now().UTC().AddDate(0, 0, -1)
			} else if dateStr == "" {
				targetDate = time.Now().UTC()
			} else {
				targetDate, err = time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", dateStr)
				}
			}

			dayStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
			dayEnd := dayStart.Add(24 * time.Hour)

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM resources WHERE resource_type = 'posts'`)
			if err != nil {
				return fmt.Errorf("querying posts: %w", err)
			}
			defer rows.Close()

			// topic slug -> posts
			topicPosts := make(map[string][]digestPost)
			topicOrder := []string{}

			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var post struct {
					Name          string `json:"name"`
					Slug          string `json:"slug"`
					Tagline       string `json:"tagline"`
					VotesCount    int    `json:"votesCount"`
					CommentsCount int    `json:"commentsCount"`
					FeaturedAt    string `json:"featuredAt"`
					CreatedAt     string `json:"createdAt"`
					URL           string `json:"url"`
					Topics        struct {
						Edges []struct {
							Node struct {
								Slug string `json:"slug"`
								Name string `json:"name"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"topics"`
				}
				if err := json.Unmarshal([]byte(raw), &post); err != nil {
					continue
				}

				// Use featuredAt if available, otherwise createdAt
				tsStr := post.FeaturedAt
				if tsStr == "" {
					tsStr = post.CreatedAt
				}
				ts, err := time.Parse(time.RFC3339, tsStr)
				if err != nil {
					continue
				}
				if ts.Before(dayStart) || !ts.Before(dayEnd) {
					continue
				}

				dp := digestPost{
					Name:          post.Name,
					Slug:          post.Slug,
					Tagline:       post.Tagline,
					VotesCount:    post.VotesCount,
					CommentsCount: post.CommentsCount,
					URL:           post.URL,
				}

				for _, e := range post.Topics.Edges {
					slug := e.Node.Slug
					if topicFilter != "" && strings.ToLower(slug) != topicFilter {
						continue
					}
					if _, ok := topicPosts[slug]; !ok {
						topicOrder = append(topicOrder, slug)
					}
					topicPosts[slug] = append(topicPosts[slug], dp)
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading posts: %w", err)
			}

			if len(topicPosts) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No posts found for %s. Run 'sync' first.\n", dayStart.Format("2006-01-02"))
				return nil
			}

			sort.Strings(topicOrder)

			var groups []digestTopicGroup
			totalPosts := 0
			for _, topic := range topicOrder {
				posts := topicPosts[topic]
				sort.Slice(posts, func(i, j int) bool {
					return posts[i].VotesCount > posts[j].VotesCount
				})
				if limit > 0 && len(posts) > limit {
					posts = posts[:limit]
				}
				groups = append(groups, digestTopicGroup{
					Topic: topic,
					Posts: posts,
				})
				totalPosts += len(posts)
			}

			result := digestResult{
				Date:       dayStart.Format("2006-01-02"),
				TotalPosts: totalPosts,
				Topics:     groups,
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			prov := DataProvenance{Source: "store"}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, err := wrapWithProvenance(filtered, prov)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			// Human-friendly output
			fmt.Fprintf(cmd.OutOrStdout(), "=== Product Hunt Digest: %s ===\n\n", result.Date)
			for _, g := range result.Topics {
				fmt.Fprintf(cmd.OutOrStdout(), "[ %s ]\n", strings.ToUpper(g.Topic))
				for i, p := range g.Posts {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (%d votes)\n     %s\n", i+1, p.Name, p.VotesCount, p.Tagline)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total: %d posts across %d topics\n", result.TotalPosts, len(result.Topics))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&dateStr, "date", "", "Date to digest in YYYY-MM-DD format (default: today)")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum posts per topic in the digest")
	cmd.Flags().BoolVar(&yesterday, "yesterday", false, "Show digest for yesterday instead of today")
	return cmd
}
