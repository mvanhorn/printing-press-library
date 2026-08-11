// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored novel command: news digest.
// pp:data-source live
package cli

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

// newNovelNewsDigestCmd builds `news digest`: deduped headlines pulled from
// both /news/latest-news/ and /news/business/markets/, trimmed to a count.
// Dedupe is by URL (canonical article id) and by normalized title, so the same
// story syndicated across both feeds appears once.
func newNovelNewsDigestCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Deduped market headlines from latest-news and markets feeds.",
		Long: "Deduped market headlines from latest-news and markets feeds.\n\n" +
			"Pulls /news/latest-news/ and /news/business/markets/, removes duplicates by URL\n" +
			"and title, and trims to --limit. Fastest path to 'what moved today'.",
		Example: `  moneycontrol-pp-cli news digest
  moneycontrol-pp-cli news digest --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "news digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch both feeds concurrently — they are independent and each is a
			// full round-trip to www.moneycontrol.com, so parallelizing roughly
			// halves wall time and keeps the command inside the probe's 10s budget.
			perFeed := limit * 3
			if perFeed < 30 {
				perFeed = 30
			}
			type feedResult struct {
				items []articleLink
				err   error
			}
			resCh := make(chan feedResult, 2)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				items, err := fetchNewsLinks(ctx, c, "/news/latest-news/", perFeed)
				resCh <- feedResult{items, err}
			}()
			go func() {
				defer wg.Done()
				items, err := fetchNewsLinks(ctx, c, "/news/business/markets/", perFeed)
				resCh <- feedResult{items, err}
			}()
			wg.Wait()
			close(resCh)
			var latest, markets []articleLink
			var errL, errM error
			reads := 0
			for r := range resCh {
				if reads == 0 {
					latest, errL = r.items, r.err
				} else {
					markets, errM = r.items, r.err
				}
				reads++
			}
			if errL != nil && errM != nil {
				return fmt.Errorf("both news feeds failed: latest=%v markets=%v", errL, errM)
			}

			seenURL := make(map[string]bool)
			seenTitle := make(map[string]bool)
			out := make([]articleLink, 0, limit)
			add := func(items []articleLink) {
				for _, it := range items {
					if seenURL[it.URL] {
						continue
					}
					norm := normalizeTitle(it.Title)
					if seenTitle[norm] {
						continue
					}
					seenURL[it.URL] = true
					seenTitle[norm] = true
					out = append(out, it)
					if limit > 0 && len(out) >= limit {
						return
					}
				}
			}
			// Markets first (more relevant to the user's market-news focus),
			// then latest-news as backfill.
			add(markets)
			if limit == 0 || len(out) < limit {
				add(latest)
			}

			view := struct {
				Count    int           `json:"count"`
				Articles []articleLink `json:"articles"`
				Sources  struct {
					Latest  int `json:"latest_news_fetched"`
					Markets int `json:"markets_fetched"`
				} `json:"sources"`
			}{}
			view.Count = len(out)
			view.Articles = out
			view.Sources.Latest = len(latest)
			view.Sources.Markets = len(markets)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out2 := cmd.OutOrStdout()
			fmt.Fprintf(out2, "NEWS DIGEST (%d)\n", view.Count)
			for i, h := range out {
				fmt.Fprintf(out2, "  %d. %s\n     %s\n", i+1, h.Title, h.URL)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 12, "max deduped headlines to return")
	return cmd
}

// normalizeTitle collapses whitespace and lowercases for dedup. It intentionally
// keeps punctuation so that genuinely different headlines do not collapse.
func normalizeTitle(s string) string {
	var b []byte
	prevSpace := false
	for _, r := range s {
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if !prevSpace {
				b = append(b, ' ')
				prevSpace = true
			}
		default:
			if r >= 'A' && r <= 'Z' {
				r = r + 32
			}
			b = append(b, byte(r))
			prevSpace = false
		}
	}
	return string(b)
}
