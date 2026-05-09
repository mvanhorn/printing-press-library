// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/store"
	"github.com/spf13/cobra"
)

func newPostsCrossTopicCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var topicsCSV string
	var limit int

	cmd := &cobra.Command{
		Use:   "cross-topic",
		Short: "Find posts that appear in multiple topics simultaneously",
		Long: `Queries the local store for posts tagged with ALL of the specified topics.
Answers questions like: "which AI tools are also productivity tools?"

Reads from the local store (run 'sync' first).`,
		Example: strings.Trim(`
  product-hunt-pp-cli posts cross-topic --topics ai,productivity
  product-hunt-pp-cli posts cross-topic --topics developer-tools,open-source --json
  product-hunt-pp-cli posts cross-topic --topics saas,analytics --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topicsCSV == "" {
				return fmt.Errorf("--topics is required (comma-separated list of topic slugs, e.g. --topics ai,productivity)")
			}

			required := strings.Split(topicsCSV, ",")
			for i, t := range required {
				required[i] = strings.TrimSpace(strings.ToLower(t))
			}
			if len(required) < 2 {
				return fmt.Errorf("--topics requires at least two topic slugs")
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

			type postTopics struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
			}
			type postData struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				Slug          string `json:"slug"`
				Tagline       string `json:"tagline"`
				VotesCount    int    `json:"votesCount"`
				CommentsCount int    `json:"commentsCount"`
				FeaturedAt    string `json:"featuredAt,omitempty"`
				URL           string `json:"url,omitempty"`
				Topics        struct {
					Edges []struct {
						Node postTopics `json:"node"`
					} `json:"edges"`
				} `json:"topics"`
			}

			var matches []map[string]any
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var post postData
				if err := json.Unmarshal([]byte(raw), &post); err != nil {
					continue
				}
				if post.Slug == "" {
					continue
				}

				// Build a set of topic slugs on this post
				postTopicSet := make(map[string]bool)
				for _, e := range post.Topics.Edges {
					postTopicSet[strings.ToLower(e.Node.Slug)] = true
				}

				// Check all required topics are present
				found := true
				for _, req := range required {
					if !postTopicSet[req] {
						found = false
						break
					}
				}
				if !found {
					continue
				}

				matches = append(matches, map[string]any{
					"id":            post.ID,
					"name":          post.Name,
					"slug":          post.Slug,
					"tagline":       post.Tagline,
					"votesCount":    post.VotesCount,
					"commentsCount": post.CommentsCount,
					"featuredAt":    post.FeaturedAt,
					"url":           post.URL,
				})

				if limit > 0 && len(matches) >= limit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading posts: %w", err)
			}

			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No posts found in all topics: %s\n(Run 'sync' first if the store is empty.)\n", topicsCSV)
				return nil
			}

			data, err := json.Marshal(matches)
			if err != nil {
				return err
			}

			prov := DataProvenance{Source: "store"}
			printProvenance(cmd, len(matches), prov)

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
				if json.Unmarshal(data, &matches) == nil && len(matches) > 0 {
					return printAutoTable(cmd.OutOrStdout(), matches)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&topicsCSV, "topics", "", "Comma-separated topic slugs that posts must appear in (all required)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum posts to return")
	return cmd
}
