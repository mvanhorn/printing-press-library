// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type auditMaker struct {
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Posts    []string `json:"posts"`
}

type auditResult struct {
	Topic       string      `json:"topic"`
	PostsFound  int         `json:"postsFound"`
	Posts       []map[string]any `json:"posts"`
	MakerOverlap []auditMaker   `json:"makerOverlap,omitempty"`
}

func newTopicsAuditCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "audit <topic-slug>",
		Short: "Rank recent launches in a topic and detect maker overlap across top posts",
		Long: `Fetches the most recent posts for a topic from the live API, ranks them by
votes, and detects makers who appear in multiple top posts. Useful for
competitive intelligence: who are the serial makers dominating this category?`,
		Example: strings.Trim(`
  product-hunt-pp-cli topics audit developer-tools
  product-hunt-pp-cli topics audit ai --limit 20 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			// pp:client-call — live API call via phgraphql client
			conn, err := phc.GetPosts(cmd.Context(), limit, "", args[0], "RANKING", false, "", "")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Build ranked posts list
			type postRow struct {
				ID            string   `json:"id"`
				Name          string   `json:"name"`
				Slug          string   `json:"slug"`
				Tagline       string   `json:"tagline"`
				VotesCount    int      `json:"votesCount"`
				CommentsCount int      `json:"commentsCount"`
				FeaturedAt    string   `json:"featuredAt,omitempty"`
				URL           string   `json:"url,omitempty"`
				Makers        []string `json:"makers,omitempty"`
			}

			posts := make([]postRow, 0, len(conn.Edges))
			// makerPosts: username -> list of post names they appear in
			makerPosts := make(map[string][]string)
			makerNames := make(map[string]string)

			for _, e := range conn.Edges {
				p := e.Node
				row := postRow{
					ID:            p.ID,
					Name:          p.Name,
					Slug:          p.Slug,
					Tagline:       p.Tagline,
					VotesCount:    p.VotesCount,
					CommentsCount: p.CommentsCount,
					FeaturedAt:    p.FeaturedAt,
					URL:           p.URL,
				}
				for _, m := range p.Makers {
					row.Makers = append(row.Makers, m.Username)
					makerPosts[m.Username] = append(makerPosts[m.Username], p.Name)
					makerNames[m.Username] = m.Name
				}
				posts = append(posts, row)
			}

			sort.Slice(posts, func(i, j int) bool {
				return posts[i].VotesCount > posts[j].VotesCount
			})

			// Detect overlap: makers in more than 1 post
			var overlap []auditMaker
			for username, postNames := range makerPosts {
				if len(postNames) > 1 {
					overlap = append(overlap, auditMaker{
						Username: username,
						Name:     makerNames[username],
						Posts:    postNames,
					})
				}
			}
			sort.Slice(overlap, func(i, j int) bool {
				return len(overlap[i].Posts) > len(overlap[j].Posts)
			})

			// Convert posts to []map[string]any for printAutoTable
			postMaps := make([]map[string]any, len(posts))
			for i, p := range posts {
				postMaps[i] = map[string]any{
					"name":          p.Name,
					"slug":          p.Slug,
					"votesCount":    p.VotesCount,
					"commentsCount": p.CommentsCount,
					"makers":        strings.Join(p.Makers, ", "),
					"featuredAt":    p.FeaturedAt,
				}
			}

			result := auditResult{
				Topic:        args[0],
				PostsFound:   len(posts),
				Posts:        postMaps,
				MakerOverlap: overlap,
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Number of posts to fetch and audit")
	return cmd
}
