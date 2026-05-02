package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsGrepCmd(flags *rootFlags) *cobra.Command {
	var (
		term  string
		topic string
		since string
		count int
	)
	cmd := &cobra.Command{
		Use:   "grep",
		Short: "Search synced launches' taglines and descriptions for a keyword (brand-mention tracker)",
		Long: strings.Trim(`
Brand-mention tracker. Pulls launches in the window (default last 7 days) and
greps tagline + description for the supplied --term, which is interpreted as a
Go regular expression. Use this nightly to catch competitive activity or to
find launches that mention your brand.

Note: --term is treated as a regex. Use \\b for word boundaries (e.g. \\bclaude\\b).
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts grep --term "\\bclaude\\b" --since 7d --json
  producthunt-pp-cli posts grep --term "agent|agentic" --topic developer-tools --since 14d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if term == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pat, err := regexp.Compile("(?i)" + term)
			if err != nil {
				return fmt.Errorf("--term invalid regex: %w", err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)

			postedAfter, err := parseSinceDurationISO(since)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			vars := map[string]any{"first": count, "order": "NEWEST", "postedAfter": postedAfter}
			if topic != "" {
				vars["topic"] = topic
			}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}

			matches := []postsGrepMatch{}
			for _, e := range resp.Posts.Edges {
				hayTagline := e.Node.Tagline
				hayDesc := e.Node.Description
				switch {
				case pat.MatchString(hayTagline):
					matches = append(matches, postsGrepMatch{Slug: e.Node.Slug, Name: e.Node.Name, Field: "tagline", Snippet: hayTagline, VotesCount: e.Node.VotesCount})
				case pat.MatchString(hayDesc):
					matches = append(matches, postsGrepMatch{Slug: e.Node.Slug, Name: e.Node.Name, Field: "description", Snippet: snippetAround(hayDesc, pat), VotesCount: e.Node.VotesCount})
				}
			}

			out := postsGrepOut{
				Term:        term,
				Topic:       topic,
				Since:       since,
				PostedAfter: postedAfter,
				Scanned:     len(resp.Posts.Edges),
				Matches:     matches,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Regex to search in tagline and description (case-insensitive). Required.")
	cmd.Flags().StringVar(&topic, "topic", "", "Limit search to a topic slug")
	cmd.Flags().StringVar(&since, "since", "7d", "Look back this far for launches (e.g. 7d, 24h, 30d)")
	cmd.Flags().IntVar(&count, "count", 20, "Max launches to scan in the window (per page)")
	return cmd
}

type postsGrepMatch struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Field      string `json:"field"`
	Snippet    string `json:"snippet"`
	VotesCount int    `json:"votes_count"`
}

type postsGrepOut struct {
	Term        string           `json:"term"`
	Topic       string           `json:"topic,omitempty"`
	Since       string           `json:"since"`
	PostedAfter string           `json:"posted_after_iso"`
	Scanned     int              `json:"scanned"`
	Matches     []postsGrepMatch `json:"matches"`
}

func snippetAround(text string, pat *regexp.Regexp) string {
	loc := pat.FindStringIndex(text)
	if loc == nil {
		return truncate(text, 80)
	}
	start := loc[0] - 30
	if start < 0 {
		start = 0
	}
	end := loc[1] + 30
	if end > len(text) {
		end = len(text)
	}
	return strings.TrimSpace(text[start:end])
}
