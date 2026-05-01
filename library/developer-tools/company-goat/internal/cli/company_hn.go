// Hand-written: launches and mentions commands. Hacker News Algolia.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/hn"
	"github.com/spf13/cobra"
)

type launchEntry struct {
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	Author    string `json:"author"`
	Points    int    `json:"points"`
	Comments  int    `json:"num_comments"`
	CreatedAt string `json:"created_at"`
	StoryID   int    `json:"story_id"`
	HNURL     string `json:"hn_url"`
}

func newLaunchesCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var maxHits int
	var minPoints int

	cmd := &cobra.Command{
		Use:   "launches [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show HN posts mentioning the company, sorted by points. Includes year hints to spot dead vs. active launches.",
		Long: `launches searches the Hacker News Algolia index for "Show HN" posts where the title or content mentions the resolved company. Results are sorted by points descending.

Use this to gauge launch story strength, find the canonical Show HN post for a product, or spot when a startup pivoted/relaunched.`,
		Example: strings.Trim(`
  company-goat-pp-cli launches replit
  company-goat-pp-cli launches stripe --json --max 10
  company-goat-pp-cli launches vercel --min-points 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			if maxHits <= 0 {
				maxHits = 20
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			stem := strings.SplitN(domain, ".", 2)[0]

			c := hn.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()

			// Use the original args (more semantic) plus the stem fallback.
			query := strings.Join(args, " ")
			if query == "" {
				query = stem
			}
			resp, err := c.SearchShowHN(ctx, query, maxHits)
			if err != nil {
				return fmt.Errorf("hn: %w", err)
			}
			entries := make([]launchEntry, 0, len(resp.Hits))
			for _, h := range resp.Hits {
				if h.Points < minPoints {
					continue
				}
				entries = append(entries, launchEntry{
					Title:     h.Title,
					URL:       h.URL,
					Author:    h.Author,
					Points:    h.Points,
					Comments:  h.NumComments,
					CreatedAt: h.CreatedAt,
					StoryID:   h.StoryID,
					HNURL:     fmt.Sprintf("https://news.ycombinator.com/item?id=%d", h.StoryID),
				})
			}
			sort.SliceStable(entries, func(i, j int) bool { return entries[i].Points > entries[j].Points })

			out := map[string]any{
				"domain":        domain,
				"query":         query,
				"launches":      entries,
				"total_matches": resp.NbHits,
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(w, "Show HN posts for %q (top %d of %d total):\n\n", domain, len(entries), resp.NbHits)
			if len(entries) == 0 {
				fmt.Fprintln(w, "no Show HN posts found")
				return nil
			}
			for _, e := range entries {
				yr := ""
				if len(e.CreatedAt) >= 4 {
					yr = e.CreatedAt[:4]
				}
				fmt.Fprintf(w, "  %s  %4d↑  %3d💬  %s\n", yr, e.Points, e.Comments, fundingTruncate(e.Title, 80))
				fmt.Fprintf(w, "    %s\n", e.HNURL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().IntVar(&maxHits, "max", 20, "Maximum hits to return")
	cmd.Flags().IntVar(&minPoints, "min-points", 0, "Filter to posts with at least this many points")
	return cmd
}

func newMentionsCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var maxHits int
	var topStories int

	cmd := &cobra.Command{
		Use:   "mentions [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Hacker News mention timeline plus the top N stories by points, via Algolia full-text search.",
		Long: `mentions searches HN's full-text Algolia index for any story containing the resolved company name. Returns two views in one call: a year-month histogram for a quick "is this still talked about?" view, and the top N stories sorted by points so an agent can dive into the most-discussed mentions without a second query.

The Show HN flavor lives under the launches command; mentions covers all stories — third-party reviews, debate threads, Ask HNs, polls.`,
		Example: strings.Trim(`
  company-goat-pp-cli mentions stripe
  company-goat-pp-cli mentions anthropic --top 20 --json
  company-goat-pp-cli mentions "june oven" --top 15
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			if maxHits <= 0 {
				maxHits = 100
			}
			if topStories < 0 {
				topStories = 0
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			if query == "" {
				query = strings.SplitN(domain, ".", 2)[0]
			}
			c := hn.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			resp, err := c.SearchByDate(ctx, query, maxHits)
			if err != nil {
				return fmt.Errorf("hn: %w", err)
			}
			type bucket struct {
				Month string `json:"month"`
				Count int    `json:"count"`
			}
			counts := map[string]int{}
			for _, h := range resp.Hits {
				if len(h.CreatedAt) < 7 {
					continue
				}
				counts[h.CreatedAt[:7]]++
			}
			months := make([]string, 0, len(counts))
			for m := range counts {
				months = append(months, m)
			}
			sort.Strings(months)
			timeline := make([]bucket, 0, len(months))
			for _, m := range months {
				timeline = append(timeline, bucket{Month: m, Count: counts[m]})
			}

			// Top stories — by-date search returns chronological hits; resort by
			// points so the agent's first stories are the most-discussed mentions.
			// launchEntry is reused here so the JSON shape matches the launches
			// command's stories block.
			ranked := make([]launchEntry, 0, len(resp.Hits))
			for _, h := range resp.Hits {
				ranked = append(ranked, launchEntry{
					Title:     h.Title,
					URL:       h.URL,
					Author:    h.Author,
					Points:    h.Points,
					Comments:  h.NumComments,
					CreatedAt: h.CreatedAt,
					StoryID:   h.StoryID,
					HNURL:     fmt.Sprintf("https://news.ycombinator.com/item?id=%d", h.StoryID),
				})
			}
			sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Points > ranked[j].Points })
			if topStories > 0 && len(ranked) > topStories {
				ranked = ranked[:topStories]
			}

			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, map[string]any{
					"domain":         domain,
					"query":          query,
					"timeline":       timeline,
					"top_stories":    ranked,
					"total_mentions": resp.NbHits,
					"sampled_hits":   len(resp.Hits),
				})
			}
			fmt.Fprintf(w, "HN mentions for %q (sampled %d of %d total):\n\n", domain, len(resp.Hits), resp.NbHits)
			if len(timeline) == 0 {
				fmt.Fprintln(w, "no mentions found")
				return nil
			}
			for _, b := range timeline {
				bar := strings.Repeat("█", b.Count)
				if len(bar) > 40 {
					bar = bar[:40] + "..."
				}
				fmt.Fprintf(w, "  %s  %3d  %s\n", b.Month, b.Count, bar)
			}
			if len(ranked) > 0 {
				fmt.Fprintf(w, "\nTop %d stories by points:\n\n", len(ranked))
				for _, e := range ranked {
					yr := ""
					if len(e.CreatedAt) >= 4 {
						yr = e.CreatedAt[:4]
					}
					fmt.Fprintf(w, "  %s  %4d↑  %3d💬  %s\n", yr, e.Points, e.Comments, fundingTruncate(e.Title, 80))
					fmt.Fprintf(w, "    %s\n", e.HNURL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().IntVar(&maxHits, "max", 100, "Maximum hits to sample for the timeline (max 1000)")
	cmd.Flags().IntVar(&topStories, "top", 5, "Top N stories by points to include alongside the timeline (0 = timeline only)")
	return cmd
}
