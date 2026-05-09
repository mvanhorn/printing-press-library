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

type trendingTopic struct {
	Slug         string  `json:"slug"`
	Name         string  `json:"name"`
	ThisWeek     int     `json:"thisWeekPosts"`
	LastWeek     int     `json:"lastWeekPosts"`
	PostGrowth   float64 `json:"postGrowthPct"`
	ThisWeekVotes int    `json:"thisWeekVotes"`
	LastWeekVotes int    `json:"lastWeekVotes"`
	VoteGrowth   float64 `json:"voteGrowthPct"`
}

func newTopicsTrendingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var days int

	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Which topics have the fastest-growing post volume this week vs last week",
		Long: `Reads the local store to compute per-topic post counts and vote volumes for
this period vs the prior period. Topics with the highest growth ratio rank first.

Use --days to set the comparison window (default 7). With --days 7, posts from
the last 7 days are "this period" and days 8-14 ago are "last period".

Run 'sync' first to populate the store.`,
		Example: strings.Trim(`
  product-hunt-pp-cli topics trending
  product-hunt-pp-cli topics trending --days 7 --agent
  product-hunt-pp-cli topics trending --limit 10 --json`, "\n"),
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

			now := time.Now().UTC()
			if days <= 0 {
				days = 7
			}
			thisWeekStart := now.AddDate(0, 0, -days)
			lastWeekStart := now.AddDate(0, 0, -days*2)

			type topicStats struct {
				name          string
				thisWeekPosts int
				lastWeekPosts int
				thisWeekVotes int
				lastWeekVotes int
			}
			topics := make(map[string]*topicStats)

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM resources WHERE resource_type = 'posts'`)
			if err != nil {
				return fmt.Errorf("querying posts: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var post struct {
					VotesCount int    `json:"votesCount"`
					CreatedAt  string `json:"createdAt"`
					Topics     struct {
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
				ts, err := time.Parse(time.RFC3339, post.CreatedAt)
				if err != nil {
					continue
				}

				isThisWeek := ts.After(thisWeekStart)
				isLastWeek := ts.After(lastWeekStart) && !isThisWeek

				if !isThisWeek && !isLastWeek {
					continue
				}

				for _, e := range post.Topics.Edges {
					slug := strings.ToLower(e.Node.Slug)
					if _, ok := topics[slug]; !ok {
						topics[slug] = &topicStats{name: e.Node.Name}
					}
					t := topics[slug]
					if isThisWeek {
						t.thisWeekPosts++
						t.thisWeekVotes += post.VotesCount
					} else {
						t.lastWeekPosts++
						t.lastWeekVotes += post.VotesCount
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading posts: %w", err)
			}

			var results []trendingTopic
			for slug, t := range topics {
				var postGrowth, voteGrowth float64
				if t.lastWeekPosts > 0 {
					postGrowth = float64(t.thisWeekPosts-t.lastWeekPosts) / float64(t.lastWeekPosts) * 100
				} else if t.thisWeekPosts > 0 {
					postGrowth = 100
				}
				if t.lastWeekVotes > 0 {
					voteGrowth = float64(t.thisWeekVotes-t.lastWeekVotes) / float64(t.lastWeekVotes) * 100
				} else if t.thisWeekVotes > 0 {
					voteGrowth = 100
				}
				results = append(results, trendingTopic{
					Slug:          slug,
					Name:          t.name,
					ThisWeek:      t.thisWeekPosts,
					LastWeek:      t.lastWeekPosts,
					PostGrowth:    postGrowth,
					ThisWeekVotes: t.thisWeekVotes,
					LastWeekVotes: t.lastWeekVotes,
					VoteGrowth:    voteGrowth,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].PostGrowth > results[j].PostGrowth
			})

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No posts in local store from the past 2 weeks. Run 'sync' first.")
				return nil
			}

			data, err := json.Marshal(results)
			if err != nil {
				return err
			}

			prov := DataProvenance{Source: "store"}
			printProvenance(cmd, len(results), prov)

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
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum topics to return")
	cmd.Flags().IntVar(&days, "days", 7, "Comparison window in days (this period vs prior period)")
	return cmd
}
