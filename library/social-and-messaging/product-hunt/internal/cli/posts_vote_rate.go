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

type voteRateResult struct {
	Slug            string  `json:"slug"`
	Name            string  `json:"name"`
	VotesCount      int     `json:"votesCount"`
	DaysSinceLaunch float64 `json:"daysSinceLaunch"`
	VotesPerDay     float64 `json:"votesPerDay"`
	FeaturedAt      string  `json:"featuredAt,omitempty"`
	URL             string  `json:"url,omitempty"`
}

func newPostsVoteRateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var minDays int
	var topicSlug string
	var days int

	cmd := &cobra.Command{
		Use:   "vote-rate",
		Short: "Rank posts by votes-per-day to surface underrated recent launches",
		Long: `Ranks posts by votes ÷ days since launch rather than raw vote totals.
A post with 200 votes in 2 days beats one with 300 votes in 30 days.

Reads from local store (run 'sync' first). Posts with fewer than --min-days
days since launch are excluded to avoid day-0 noise. Use --topic to filter
to a specific topic slug, and --days to limit to posts from the last N days.`,
		Example: strings.Trim(`
  product-hunt-pp-cli posts vote-rate
  product-hunt-pp-cli posts vote-rate --topic developer-tools --days 30 --json
  product-hunt-pp-cli posts vote-rate --limit 20 --min-days 2 --json`, "\n"),
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

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM resources WHERE resource_type = 'posts'`)
			if err != nil {
				return fmt.Errorf("querying posts: %w", err)
			}
			defer rows.Close()

			now := time.Now().UTC()
			var results []voteRateResult
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var post struct {
					Slug       string `json:"slug"`
					Name       string `json:"name"`
					VotesCount int    `json:"votesCount"`
					CreatedAt  string `json:"createdAt"`
					FeaturedAt string `json:"featuredAt"`
					URL        string `json:"url"`
					Topics     struct {
						Edges []struct {
							Node struct {
								Slug string `json:"slug"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"topics"`
				}
				if err := json.Unmarshal([]byte(raw), &post); err != nil {
					continue
				}
				if post.Slug == "" {
					continue
				}

				// Topic filter
				if topicSlug != "" {
					found := false
					for _, e := range post.Topics.Edges {
						if strings.ToLower(e.Node.Slug) == strings.ToLower(topicSlug) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				ts, err := time.Parse(time.RFC3339, post.CreatedAt)
				if err != nil {
					continue
				}
				ageDays := now.Sub(ts).Hours() / 24
				if ageDays < float64(minDays) {
					continue
				}
				// Days window filter
				if days > 0 && ageDays > float64(days) {
					continue
				}

				results = append(results, voteRateResult{
					Slug:            post.Slug,
					Name:            post.Name,
					VotesCount:      post.VotesCount,
					DaysSinceLaunch: ageDays,
					VotesPerDay:     float64(post.VotesCount) / ageDays,
					FeaturedAt:      post.FeaturedAt,
					URL:             post.URL,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading posts: %w", err)
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].VotesPerDay > results[j].VotesPerDay
			})

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No posts in local store. Run 'sync' first.")
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
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum posts to return")
	cmd.Flags().IntVar(&minDays, "min-days", 1, "Exclude posts with fewer than N days since launch")
	cmd.Flags().StringVar(&topicSlug, "topic", "", "Filter to posts in this topic slug")
	cmd.Flags().IntVar(&days, "days", 0, "Limit to posts from the last N days (0 = no limit)")
	return cmd
}
