package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// `tldr` produces a deterministic structured digest of a thread:
// top authors by reply count, root vs reply ratio, comment heat
// metric. We avoid AI summaries deliberately — the goal is
// reproducible, scriptable signal, not opinion.

// tldrAuthor reports per-author signal in a thread digest. We do NOT
// emit a total_points field — Algolia's /items endpoint exposes
// per-comment Points only as a possibly-null integer that's effectively
// always zero for HN comments (HN doesn't expose comment scores via the
// public API). Reporting "total_points: 0" everywhere would mislead
// agents.
type tldrAuthor struct {
	Author       string `json:"author"`
	CommentCount int    `json:"comment_count"`
}

type tldrResult struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	URL            string         `json:"url"`
	StoryAuthor    string         `json:"story_author"`
	StoryPoints    int            `json:"story_points"`
	TotalComments  int            `json:"total_comments"`
	UniqueAuthors  int            `json:"unique_authors"`
	RootComments   int            `json:"root_comments"`
	ReplyComments  int            `json:"reply_comments"`
	HeatMetric     float64        `json:"heat_metric"` // comments per point — higher = hotter
	TopAuthors     []tldrAuthor   `json:"top_authors"`
	DepthHistogram map[string]int `json:"depth_histogram"`
}

func newTldrCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tldr <id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Print a deterministic thread digest (top authors, reply ratio, heat metric)",
		Long: `Walk a thread's full comment tree and emit measurable signals.

No prose summarization — the digest contains structured fields:
top authors by comment count, root vs reply split, depth histogram,
and a comments-per-point heat metric.`,
		Example: strings.Trim(`
  hackernews-pp-cli tldr 12345678
  hackernews-pp-cli tldr 12345678 --json --select heat_metric,top_authors
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			ac := algolia.New(flags.timeout)
			node, err := ac.Item(id)
			if err != nil {
				return apiErr(err)
			}

			result := tldrResult{
				ID:             fmt.Sprintf("%d", node.ID),
				Title:          node.Title,
				URL:            node.URL,
				StoryAuthor:    node.Author,
				StoryPoints:    node.Points,
				DepthHistogram: map[string]int{},
			}
			if result.URL == "" {
				result.URL = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", result.ID)
			}

			authorStats := map[string]*tldrAuthor{}
			var visit func(n *algolia.ItemNode, depth int)
			visit = func(n *algolia.ItemNode, depth int) {
				for _, c := range n.Children {
					if c.Type == "comment" {
						result.TotalComments++
						if depth == 0 {
							result.RootComments++
						} else {
							result.ReplyComments++
						}
						result.DepthHistogram[fmt.Sprintf("%d", depth)]++
						a, ok := authorStats[c.Author]
						if !ok {
							a = &tldrAuthor{Author: c.Author}
							authorStats[c.Author] = a
						}
						a.CommentCount++
					}
					visit(&c, depth+1)
				}
			}
			visit(node, 0)

			result.UniqueAuthors = len(authorStats)
			if result.StoryPoints > 0 {
				result.HeatMetric = float64(result.TotalComments) / float64(result.StoryPoints)
			}

			authors := make([]tldrAuthor, 0, len(authorStats))
			for _, a := range authorStats {
				authors = append(authors, *a)
			}
			sort.Slice(authors, func(i, j int) bool {
				if authors[i].CommentCount != authors[j].CommentCount {
					return authors[i].CommentCount > authors[j].CommentCount
				}
				return authors[i].Author < authors[j].Author
			})
			if len(authors) > 5 {
				result.TopAuthors = authors[:5]
			} else {
				result.TopAuthors = authors
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(result, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s\n  by %s — %d points — %d comments — heat %.2f\n",
				truncateAtRune(result.Title, 80), result.StoryAuthor, result.StoryPoints, result.TotalComments, result.HeatMetric)
			fmt.Fprintf(cmd.OutOrStdout(), "  unique authors: %d, root: %d, replies: %d\n", result.UniqueAuthors, result.RootComments, result.ReplyComments)
			fmt.Fprintln(cmd.OutOrStdout(), "\nTop authors by comments:")
			for _, a := range result.TopAuthors {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d comments\n", a.Author, a.CommentCount)
			}
			return nil
		},
	}
	return cmd
}
