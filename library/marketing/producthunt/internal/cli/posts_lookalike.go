package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsLookalikeCmd(flags *rootFlags) *cobra.Command {
	var (
		count      int
		windowDays int
	)
	cmd := &cobra.Command{
		Use:   "lookalike <slug>",
		Short: "Find prior launches in the same topic with overlapping tagline tokens (competitive set)",
		Long: strings.Trim(`
Given a launch slug, fetches its topics, then queries each topic for recent
launches and ranks them by topic-overlap + tagline-token Jaccard similarity.
The top N matches are your competitive set.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts lookalike notion --json
  producthunt-pp-cli posts lookalike my-launch --count 5 --window-days 90
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
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
			var src phgql.PostResponse
			if _, err := c.Query(cmd.Context(), phgql.PostQuery, map[string]any{"slug": args[0]}, &src); err != nil {
				return err
			}
			if src.Post.ID == "" {
				return fmt.Errorf("source post not found: %s", args[0])
			}
			srcTokens := tokenize(src.Post.Tagline)
			srcTopics := topicSlugs(src.Post)
			if len(srcTopics) == 0 {
				return fmt.Errorf("source post %s has no topics — cannot compute lookalike", args[0])
			}
			postedAfter, _ := parseSinceDurationISO(fmt.Sprintf("%dd", windowDays))
			seen := map[string]phgql.Post{}
			for _, topic := range srcTopics {
				vars := map[string]any{"first": 20, "topic": topic, "order": "VOTES", "postedAfter": postedAfter}
				var resp phgql.PostsResponse
				if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
					return fmt.Errorf("topic %s: %w", topic, err)
				}
				for _, e := range resp.Posts.Edges {
					if e.Node.ID == src.Post.ID {
						continue
					}
					seen[e.Node.ID] = e.Node
				}
			}

			matches := make([]lookalikeMatch, 0, len(seen))
			for _, p := range seen {
				score := overlap(srcTokens, tokenize(p.Tagline))*0.7 +
					float64(commonStrings(srcTopics, topicSlugs(p)))/float64(len(srcTopics))*0.3
				matches = append(matches, lookalikeMatch{
					Slug: p.Slug, Name: p.Name, Tagline: p.Tagline,
					VotesCount: p.VotesCount, Score: score,
				})
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
			if count > 0 && len(matches) > count {
				matches = matches[:count]
			}
			out := lookalikeOut{
				Source:  src.Post.Slug,
				Topics:  srcTopics,
				Matches: matches,
				Note:    fmt.Sprintf("Scored across %d candidate launches in %d topics over the last %dd window.", len(seen), len(srcTopics), windowDays),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 5, "Max lookalike matches to return")
	cmd.Flags().IntVar(&windowDays, "window-days", 365, "Look back this many days for candidate launches")
	return cmd
}

type lookalikeMatch struct {
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	Tagline    string  `json:"tagline"`
	VotesCount int     `json:"votes_count"`
	Score      float64 `json:"score"`
}

type lookalikeOut struct {
	Source  string           `json:"source_slug"`
	Topics  []string         `json:"topics"`
	Matches []lookalikeMatch `json:"matches"`
	Note    string           `json:"note"`
}

func tokenize(s string) map[string]bool {
	tokens := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?:;()[]{}\"'`")
		if len(w) >= 3 {
			tokens[w] = true
		}
	}
	return tokens
}

func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersect := 0
	for k := range a {
		if b[k] {
			intersect++
		}
	}
	union := len(a) + len(b) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}

func commonStrings(a, b []string) int {
	set := map[string]bool{}
	for _, s := range a {
		set[s] = true
	}
	n := 0
	for _, s := range b {
		if set[s] {
			n++
		}
	}
	return n
}
