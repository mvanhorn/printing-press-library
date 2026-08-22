// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// rdCount is a name/count pair used for the digest's rollups.
type rdCount struct {
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Upvotes int    `json:"upvotes"`
}

// rdDigestPost is a headline entry in the digest.
type rdDigestPost struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	UpvoteCount int    `json:"upvoteCount"`
	Author      string `json:"author"`
	Age         string `json:"age"`
	URL         string `json:"url"`
}

// rdDigestResult is the whole rollup for one window.
type rdDigestResult struct {
	Window        string         `json:"window"`
	NewWorkflows  int            `json:"newWorkflows"`
	TotalUpvotes  int            `json:"totalUpvotes"`
	TotalComments int            `json:"totalComments"`
	Featured      int            `json:"newsletterFeatured"`
	TopPosts      []rdDigestPost `json:"topPosts"`
	TopTools      []rdCount      `json:"topTools"`
	TopAuthors    []rdCount      `json:"topAuthors"`
	TopIndustries []rdCount      `json:"topIndustries"`
	MirroredTotal int            `json:"mirroredTotal"`
	Note          string         `json:"note,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince  string
		flagTop    int
		flagDBPath string
	)

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Summarise a time window: new workflows, top posts, most-used tools, busiest authors",
		Long: strings.Trim(`
Roll up the community over a time window.

Reports how many workflows landed, their combined upvotes and comments, how
many were featured in the newsletter, and the leading posts, tools, authors
and industries for the period.

Every figure is computed from the local mirror, so run 'rundown-pp-cli sync'
first. For a single ranked list rather than a rollup, use
'rundown-pp-cli top'.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli digest --since 7d
  rundown-pp-cli digest --since 30d --top 10
  rundown-pp-cli digest --since 7d --agent --select newWorkflows,topTools
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--since=30d",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "digest")
			}
			if flagTop <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--top must be greater than zero"))
			}
			cutoff, err := rdWindowStart(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := rdResolveDBPath(flagDBPath)
			if stop, err := rdMirrorMissing(cmd, flags, dbPath, rdDigestResult{
				Window:        flagSince,
				TopPosts:      make([]rdDigestPost, 0),
				TopTools:      make([]rdCount, 0),
				TopAuthors:    make([]rdCount, 0),
				TopIndustries: make([]rdCount, 0),
			}); stop {
				return err
			}

			db, err := rdOpenMirrorStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			posts, err := rdLoadPosts(ctx, db)
			if err != nil {
				return err
			}

			window := make([]rdPost, 0, len(posts))
			for _, p := range posts {
				if rdInWindow(p, cutoff) {
					window = append(window, p)
				}
			}

			toolAgg := map[string]*rdCount{}
			authorAgg := map[string]*rdCount{}
			industryAgg := map[string]*rdCount{}
			totalUpvotes, totalComments, featured := 0, 0, 0

			bump := func(agg map[string]*rdCount, key string, upvotes int) {
				if key == "" {
					return
				}
				if c, ok := agg[key]; ok {
					c.Count++
					c.Upvotes += upvotes
					return
				}
				agg[key] = &rdCount{Name: key, Count: 1, Upvotes: upvotes}
			}

			for _, p := range window {
				totalUpvotes += p.UpvoteCount
				totalComments += p.CommentCount
				if p.featured() {
					featured++
				}
				for _, slug := range p.toolSlugs() {
					bump(toolAgg, slug, p.UpvoteCount)
				}
				bump(authorAgg, p.authorName(), p.UpvoteCount)
				for _, ind := range p.Industries {
					bump(industryAgg, ind.Slug, p.UpvoteCount)
				}
			}

			ranked := window
			rdSortByUpvotes(ranked)
			topN := ranked
			if len(topN) > flagTop {
				topN = topN[:flagTop]
			}
			topPosts := make([]rdDigestPost, 0, len(topN))
			for _, p := range topN {
				topPosts = append(topPosts, rdDigestPost{
					ID:          p.ID,
					Title:       p.Title,
					UpvoteCount: p.UpvoteCount,
					Author:      p.authorName(),
					Age:         rdAgo(p.CreatedAt),
					URL:         rdPostURL(p.ID),
				})
			}

			label := flagSince
			if cutoff.IsZero() {
				label = "all time"
			}
			result := rdDigestResult{
				Window:        label,
				NewWorkflows:  len(window),
				TotalUpvotes:  totalUpvotes,
				TotalComments: totalComments,
				Featured:      featured,
				TopPosts:      topPosts,
				TopTools:      rdTopCounts(toolAgg, flagTop),
				TopAuthors:    rdTopCounts(authorAgg, flagTop),
				TopIndustries: rdTopCounts(industryAgg, flagTop),
				MirroredTotal: len(posts),
			}
			switch {
			case len(posts) == 0:
				result.Note = "the local mirror holds no workflows, so this is not a quiet week — run 'rundown-pp-cli sync' first."
			case len(window) == 0:
				result.Note = fmt.Sprintf("no workflows in the last %s across %d mirrored posts; widen --since or re-sync.", label, len(posts))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n\n", bold(fmt.Sprintf("The Rundown community — last %s", label)))
			if len(window) == 0 {
				fmt.Fprintf(out, "%s\n", result.Note)
				return nil
			}
			fmt.Fprintf(out, "%d new workflows · %d upvotes · %d comments · %d featured in the newsletter\n\n",
				result.NewWorkflows, result.TotalUpvotes, result.TotalComments, result.Featured)

			fmt.Fprintf(out, "%s\n", bold("Top workflows"))
			for i, p := range result.TopPosts {
				fmt.Fprintf(out, "  %d. [%d] %s — %s\n", i+1, p.UpvoteCount, truncate(p.Title, 66), p.Author)
			}

			renderCounts := func(heading string, items []rdCount, unit string) {
				if len(items) == 0 {
					return
				}
				fmt.Fprintf(out, "\n%s\n", bold(heading))
				for _, c := range items {
					fmt.Fprintf(out, "  %-32s %d %s · %d upvotes\n", truncate(c.Name, 32), c.Count, unit, c.Upvotes)
				}
			}
			renderCounts("Most-used tools", result.TopTools, "workflows")
			renderCounts("Busiest authors", result.TopAuthors, "workflows")
			renderCounts("Industries", result.TopIndustries, "workflows")

			if len(result.TopPosts) > 0 {
				fmt.Fprintf(out, "\nRead the leader: rundown-pp-cli show %s\n", result.TopPosts[0].ID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "7d", "Time window to summarise, e.g. 7d, 2w, 30d (use 'all' for no cutoff)")
	cmd.Flags().IntVar(&flagTop, "top", 5, "How many entries to show in each ranking")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// rdTopCounts flattens an aggregation map into a deterministic top-N slice.
func rdTopCounts(agg map[string]*rdCount, limit int) []rdCount {
	out := make([]rdCount, 0, len(agg))
	for _, c := range agg {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Upvotes != out[j].Upvotes {
			return out[i].Upvotes > out[j].Upvotes
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
