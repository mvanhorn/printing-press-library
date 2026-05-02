package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "category",
		Short: "Marketer research desk: category-level snapshots and trends",
	}
	cmd.AddCommand(newCategorySnapshotCmd(flags))
	return cmd
}

func newCategorySnapshotCmd(flags *rootFlags) *cobra.Command {
	var (
		topic   string
		window  string
		count   int
		topTags int
	)
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Slide-deck-ready brief for a topic over a window: leaderboard, momentum delta, active handles, emerging tagline tags",
		Long: strings.Trim(`
Renders a single-output brief for a topic over a window (default weekly):
  - Leaderboard: top launches by votes in the window
  - Momentum delta: total votes / total posts vs the prior window of equal length
  - Active poster handles: most-frequent posters in the window
  - Emerging tagline tokens: lowercase tokens (>=4 chars) that appear in this
    window's taglines but not in the prior window's

Use this for weekly category-research cadence — replaces opening 30 launch
pages by hand.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli category snapshot --topic artificial-intelligence --window weekly
  producthunt-pp-cli category snapshot --topic developer-tools --window monthly --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if topic == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			windowDays, err := parseWindowDays(window)
			if err != nil {
				return err
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)

			now := time.Now().UTC()
			start := now.AddDate(0, 0, -windowDays)
			priorStart := start.AddDate(0, 0, -windowDays)

			pull := func(after, before string) ([]phgql.Post, error) {
				vars := map[string]any{"first": count, "topic": topic, "order": "VOTES", "postedAfter": after}
				if before != "" {
					vars["postedBefore"] = before
				}
				var resp phgql.PostsResponse
				if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
					return nil, err
				}
				out := make([]phgql.Post, 0, len(resp.Posts.Edges))
				for _, e := range resp.Posts.Edges {
					out = append(out, e.Node)
				}
				return out, nil
			}

			cur, err := pull(start.Format(time.RFC3339), "")
			if err != nil {
				return fmt.Errorf("current window: %w", err)
			}
			prior, err := pull(priorStart.Format(time.RFC3339), start.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("prior window: %w", err)
			}

			out := snapshotOut{
				Topic:       topic,
				Window:      window,
				WindowDays:  windowDays,
				Leaderboard: leaderboard(cur, 10),
				Momentum: momentum{
					CurrentPostCount:  len(cur),
					PriorPostCount:    len(prior),
					CurrentTotalVotes: sumVotes(cur),
					PriorTotalVotes:   sumVotes(prior),
				},
				ActiveHandles:  topPosterHandles(cur, 10),
				EmergingTokens: emergingTokens(cur, prior, topTags),
				Note:           "Active handles are poster usernames, not maker handles (PH redacts makers globally). Emerging tokens are lowercase >=4-char words in current taglines absent from the prior window.",
			}
			out.Momentum.PostCountDelta = out.Momentum.CurrentPostCount - out.Momentum.PriorPostCount
			out.Momentum.VotesDelta = out.Momentum.CurrentTotalVotes - out.Momentum.PriorTotalVotes
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Topic slug (required)")
	cmd.Flags().StringVar(&window, "window", "weekly", "Window: daily | weekly | monthly | <N>d")
	cmd.Flags().IntVar(&count, "count", 20, "Posts to fetch per window (max 20 per page)")
	cmd.Flags().IntVar(&topTags, "tags", 6, "Number of emerging tagline tokens to surface")
	return cmd
}

type snapshotOut struct {
	Topic          string        `json:"topic"`
	Window         string        `json:"window"`
	WindowDays     int           `json:"window_days"`
	Leaderboard    []postBrief   `json:"leaderboard"`
	Momentum       momentum      `json:"momentum"`
	ActiveHandles  []handleCount `json:"active_poster_handles"`
	EmergingTokens []string      `json:"emerging_tagline_tokens"`
	Note           string        `json:"note"`
}

type postBrief struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	VotesCount    int    `json:"votes_count"`
	CommentsCount int    `json:"comments_count"`
	Tagline       string `json:"tagline"`
	PosterHandle  string `json:"poster_handle"`
}

type momentum struct {
	CurrentPostCount  int `json:"current_post_count"`
	PriorPostCount    int `json:"prior_post_count"`
	PostCountDelta    int `json:"post_count_delta"`
	CurrentTotalVotes int `json:"current_total_votes"`
	PriorTotalVotes   int `json:"prior_total_votes"`
	VotesDelta        int `json:"votes_delta"`
}

type handleCount struct {
	Username string `json:"username"`
	Count    int    `json:"count"`
}

func parseWindowDays(s string) (int, error) {
	switch s {
	case "daily":
		return 1, nil
	case "weekly":
		return 7, nil
	case "monthly":
		return 30, nil
	}
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil && n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("--window: expected daily | weekly | monthly | <N>d, got %q", s)
}

func leaderboard(posts []phgql.Post, n int) []postBrief {
	sorted := make([]phgql.Post, len(posts))
	copy(sorted, posts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].VotesCount > sorted[j].VotesCount })
	if n > 0 && len(sorted) > n {
		sorted = sorted[:n]
	}
	out := make([]postBrief, 0, len(sorted))
	for _, p := range sorted {
		out = append(out, postBrief{
			Slug: p.Slug, Name: p.Name,
			VotesCount: p.VotesCount, CommentsCount: p.CommentsCount,
			Tagline: p.Tagline, PosterHandle: p.User.Username,
		})
	}
	return out
}

func sumVotes(posts []phgql.Post) int {
	n := 0
	for _, p := range posts {
		n += p.VotesCount
	}
	return n
}

func topPosterHandles(posts []phgql.Post, n int) []handleCount {
	counts := map[string]int{}
	for _, p := range posts {
		if p.User.Username != "" {
			counts[p.User.Username]++
		}
	}
	out := make([]handleCount, 0, len(counts))
	for u, c := range counts {
		out = append(out, handleCount{Username: u, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Username < out[j].Username
		}
		return out[i].Count > out[j].Count
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func emergingTokens(cur, prior []phgql.Post, n int) []string {
	curTokens := map[string]int{}
	for _, p := range cur {
		for tok := range tokenize(p.Tagline) {
			curTokens[tok]++
		}
	}
	priorTokens := map[string]bool{}
	for _, p := range prior {
		for tok := range tokenize(p.Tagline) {
			priorTokens[tok] = true
		}
	}
	type sortable struct {
		Token string
		Count int
	}
	picks := make([]sortable, 0)
	for tok, c := range curTokens {
		if !priorTokens[tok] && len(tok) >= 4 {
			picks = append(picks, sortable{tok, c})
		}
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].Count == picks[j].Count {
			return picks[i].Token < picks[j].Token
		}
		return picks[i].Count > picks[j].Count
	})
	if n > 0 && len(picks) > n {
		picks = picks[:n]
	}
	out := make([]string, 0, len(picks))
	for _, p := range picks {
		out = append(out, p.Token)
	}
	return out
}
