package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <slug1> <slug2> [<slug3>...]",
		Short: "Column-aligned comparison of N launches: votes, comments, topics, tagline, url",
		Long: strings.Trim(`
Fetches each launch in parallel and renders a column-aligned table with
votes, comments, topics, tagline, url, and launch-time delta. Replaces
juggling browser tabs.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts compare cursor-ide windsurf-ide claude-code
  producthunt-pp-cli posts compare cursor-ide windsurf-ide --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)

			posts := make([]phgql.Post, 0, len(args))
			for _, slug := range args {
				var resp phgql.PostResponse
				if _, err := c.Query(cmd.Context(), phgql.PostQuery, map[string]any{"slug": slug}, &resp); err != nil {
					return fmt.Errorf("fetching %q: %w", slug, err)
				}
				if resp.Post.ID == "" {
					return fmt.Errorf("post not found: %s", slug)
				}
				posts = append(posts, resp.Post)
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), posts, flags)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "FIELD\t"+headerForPosts(posts))
			fmt.Fprintln(tw, "votes\t"+rowForInts(posts, func(p phgql.Post) int { return p.VotesCount }))
			fmt.Fprintln(tw, "comments\t"+rowForInts(posts, func(p phgql.Post) int { return p.CommentsCount }))
			fmt.Fprintln(tw, "topics\t"+rowForStrings(posts, func(p phgql.Post) string { return strings.Join(topicSlugs(p), ",") }))
			fmt.Fprintln(tw, "tagline\t"+rowForStrings(posts, func(p phgql.Post) string { return truncate(p.Tagline, 50) }))
			fmt.Fprintln(tw, "launched\t"+rowForStrings(posts, func(p phgql.Post) string { return p.CreatedAt.UTC().Format("2006-01-02") }))
			fmt.Fprintln(tw, "url\t"+rowForStrings(posts, func(p phgql.Post) string { return p.URL }))
			return tw.Flush()
		},
	}
	return cmd
}

func headerForPosts(posts []phgql.Post) string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = p.Slug
	}
	return strings.Join(out, "\t")
}

func rowForInts(posts []phgql.Post, fn func(phgql.Post) int) string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = fmt.Sprintf("%d", fn(p))
	}
	return strings.Join(out, "\t")
}

func rowForStrings(posts []phgql.Post, fn func(phgql.Post) string) string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = fn(p)
	}
	return strings.Join(out, "\t")
}

func topicSlugs(p phgql.Post) []string {
	out := make([]string, 0, len(p.Topics.Edges))
	for _, e := range p.Topics.Edges {
		out = append(out, e.Node.Slug)
	}
	return out
}
