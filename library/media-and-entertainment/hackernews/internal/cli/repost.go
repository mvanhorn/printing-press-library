package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

type repostHit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAt   string `json:"created_at"`
}

type repostResult struct {
	Query string      `json:"url"`
	Count int         `json:"count"`
	Hits  []repostHit `json:"submissions"`
}

func newRepostCmd(flags *rootFlags) *cobra.Command {
	var includeComments bool

	cmd := &cobra.Command{
		Use:   "repost <url>",
		Short: "Has this URL been posted on HN before? Lists prior submissions with scores and dates",
		Long: `Search Algolia for prior submissions of a URL.

Algolia's URL field is the canonical story URL — the same URL won't
match if a prior submission used a slightly different path or query.
Pre-flight check before posting; works on partial URLs too.`,
		Example: strings.Trim(`
  hackernews-pp-cli repost https://example.com/article
  hackernews-pp-cli repost https://example.com/article --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if !strings.HasPrefix(query, "http://") && !strings.HasPrefix(query, "https://") {
				return usageErr(fmt.Errorf("argument must be an http(s) URL (got %q)", query))
			}
			ac := algolia.New(flags.timeout)
			tags := "story"
			if includeComments {
				tags = ""
			}
			resp, err := ac.Search(query, algolia.SearchOpts{
				Tags:        tags,
				HitsPerPage: 50,
			})
			if err != nil {
				return apiErr(err)
			}

			out := repostResult{Query: query, Hits: []repostHit{}}
			for _, h := range resp.Hits {
				// Algolia's relevance match can include unrelated text-overlap;
				// trust the URL field if present.
				if !strings.Contains(strings.ToLower(h.URL), strings.ToLower(query)) &&
					!strings.Contains(strings.ToLower(h.Title), strings.ToLower(query)) {
					continue
				}
				out.Hits = append(out.Hits, repostHit{
					ID:          h.ObjectID,
					Title:       h.Title,
					URL:         h.URL,
					Author:      h.Author,
					Points:      h.Points,
					NumComments: h.NumComments,
					CreatedAt:   h.CreatedAt,
				})
			}
			out.Count = len(out.Hits)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}

			if out.Count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no prior submissions found for %s\n", query)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d prior submission(s) for %s\n\n", out.Count, query)
			rows := make([][]string, 0, out.Count)
			for _, h := range out.Hits {
				rows = append(rows, []string{
					h.ID,
					strconv.Itoa(h.Points),
					strconv.Itoa(h.NumComments),
					h.Author,
					strings.SplitN(h.CreatedAt, "T", 2)[0],
					truncateAtRune(h.Title, 60),
				})
			}
			return flags.printTable(cmd, []string{"ID", "PTS", "CMTS", "BY", "DATE", "TITLE"}, rows)
		},
	}
	cmd.Flags().BoolVar(&includeComments, "include-comments", false, "Also search comments for the URL (broader, noisier)")
	return cmd
}
