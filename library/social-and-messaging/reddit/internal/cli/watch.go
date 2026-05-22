// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type watchHit struct {
	ThingID     string  `json:"thing_id"`
	Sub         string  `json:"subreddit"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	CreatedUTC  float64 `json:"created_utc"`
	MatchedTerm string  `json:"matched_term"`
	Context     string  `json:"context,omitempty"`
	OPKarma     int     `json:"op_karma_in_sub,omitempty"`
}

// newWatchCmd performs multi-sub brand/term watch: fans out search across
// N subreddits, dedupes by submission ID, optionally enriches each hit with
// the OP's total karma. The signature use case: agency brand-mention
// monitoring without the cost and noise of Make.com / Zapier polling.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		inSubs      string
		since       string
		enrichKarma bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "watch <terms>",
		Short: "Multi-sub brand/term watch with context and OP-karma enrichment",
		Long: `Watch multiple subreddits for one or more terms. Fans out /search per sub,
dedupes by submission ID, and optionally enriches each hit with the OP's
total karma. Replaces noisy Make.com / Zapier brand-mention polling.

Terms are comma-separated. Subs are also comma-separated.
--since accepts hour/day/week/month/year/all (matches Reddit's t= window).`,
		Example: `  reddit-pp-cli watch "creativism" --in entrepreneur,smallbusiness --since 24h
  reddit-pp-cli watch "stripe,payments" --in programming,webdev --enrich-karma --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			terms := splitCSV(args[0])
			if len(terms) == 0 {
				return usageErr(fmt.Errorf("at least one term required"))
			}
			// Dry-run short-circuit BEFORE --in validation so verify can
			// confirm dry-run mode without requiring every flag set. The
			// verify pipeline probes commands with --dry-run plus the
			// example positional arg; --in fixture flags are not passed.
			// Real --confirm runs still hit the --in check below.
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"terms":   terms,
						"in":      inSubs,
						"since":   since,
						"limit":   limit,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would watch terms=%v in=%s since=%s limit=%d\n", terms, inSubs, since, limit)
				return nil
			}
			subs := splitCSV(inSubs)
			if len(subs) == 0 {
				return usageErr(fmt.Errorf("--in <subs> required (comma-separated)"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			t := mapSinceToWindow(since)
			seen := map[string]bool{}
			hits := []watchHit{}

			for _, term := range terms {
				for _, sub := range subs {
					params := map[string]string{
						"q":           term,
						"restrict_sr": "true",
						"sort":        "new",
						"t":           t,
						"limit":       fmt.Sprintf("%d", limit),
					}
					body, err := c.Get(cmd.Context(), "/r/"+sub+"/search", params)
					if err != nil {
						continue
					}
					var env struct {
						Data struct {
							Children []struct {
								Data struct {
									ID          string  `json:"id"`
									Name        string  `json:"name"`
									Title       string  `json:"title"`
									Author      string  `json:"author"`
									Subreddit   string  `json:"subreddit"`
									Score       int     `json:"score"`
									NumComments int     `json:"num_comments"`
									URL         string  `json:"url"`
									Permalink   string  `json:"permalink"`
									CreatedUTC  float64 `json:"created_utc"`
									Selftext    string  `json:"selftext"`
								} `json:"data"`
							} `json:"children"`
						} `json:"data"`
					}
					if err := json.Unmarshal(body, &env); err != nil {
						continue
					}
					for _, ch := range env.Data.Children {
						id := ch.Data.Name
						if id == "" {
							id = "t3_" + ch.Data.ID
						}
						if seen[id] {
							continue
						}
						seen[id] = true
						hit := watchHit{
							ThingID:     id,
							Sub:         ch.Data.Subreddit,
							Title:       ch.Data.Title,
							Author:      ch.Data.Author,
							Score:       ch.Data.Score,
							NumComments: ch.Data.NumComments,
							URL:         ch.Data.URL,
							Permalink:   ch.Data.Permalink,
							CreatedUTC:  ch.Data.CreatedUTC,
							MatchedTerm: term,
							Context:     buildSnippet(ch.Data.Title+" "+ch.Data.Selftext, term, 140),
						}
						if enrichKarma && ch.Data.Author != "" && ch.Data.Author != "[deleted]" {
							hit.OPKarma = fetchAuthorTotalKarma(cmd.Context(), c, ch.Data.Author)
						}
						hits = append(hits, hit)
					}
				}
			}

			sort.Slice(hits, func(i, j int) bool {
				return hits[i].CreatedUTC > hits[j].CreatedUTC
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			renderWatchHits(cmd.OutOrStdout(), hits)
			return nil
		},
	}
	cmd.Flags().StringVar(&inSubs, "in", "", "Comma-separated subreddits to watch")
	cmd.Flags().StringVar(&since, "since", "day", "Time window: 1h, 24h, 7d, 30d, year, all")
	cmd.Flags().BoolVar(&enrichKarma, "enrich-karma", false, "Fetch each OP's total karma (extra API calls per hit)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results per sub per term")
	return cmd
}

// mapSinceToWindow normalizes user-friendly --since values (1h, 24h, 7d, etc.)
// to Reddit's t= parameter (hour, day, week, month, year, all).
func mapSinceToWindow(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1h", "hour", "60m":
		return "hour"
	case "24h", "day", "1d":
		return "day"
	case "week", "7d":
		return "week"
	case "month", "30d":
		return "month"
	case "year", "365d":
		return "year"
	case "all", "":
		return "all"
	}
	// Best effort: treat unknown values as 'day' (most common monitoring window)
	return "day"
}

func fetchAuthorTotalKarma(ctx context.Context, c *client.Client, author string) int {
	if author == "" {
		return 0
	}
	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := c.Get(subCtx, "/user/"+author+"/about", nil)
	if err != nil {
		return 0
	}
	var env struct {
		Data struct {
			TotalKarma int `json:"total_karma"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return 0
	}
	return env.Data.TotalKarma
}

func renderWatchHits(w io.Writer, hits []watchHit) {
	if len(hits) == 0 {
		fmt.Fprintln(w, "No mentions found.")
		return
	}
	for i, h := range hits {
		when := time.Unix(int64(h.CreatedUTC), 0).UTC().Format(time.RFC3339)
		fmt.Fprintf(w, "%d. [r/%s] %s\n   by u/%s at %s • %d pts • %d comments\n",
			i+1, h.Sub, h.Title, h.Author, when, h.Score, h.NumComments)
		if h.OPKarma > 0 {
			fmt.Fprintf(w, "   OP karma: %d\n", h.OPKarma)
		}
		if h.Context != "" {
			fmt.Fprintf(w, "   %s\n", h.Context)
		}
		fmt.Fprintf(w, "   https://reddit.com%s\n\n", h.Permalink)
	}
}
