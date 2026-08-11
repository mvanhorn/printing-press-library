// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored novel command: news-for.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// newNovelNewsForCmd builds `news-for <sc_id1,sc_id2,...>`: the latest tagged
// news for several stocks, fetched in parallel. Each sc_id maps to a tag slug
// (via the same table stock-watch uses); the user can override any of them
// with --tag-slugs=reliance-industries,infosys.
//
// Per the parallel-fetch rule: each stock's fetch error is preserved on its
// own entry (news_error field) and excluded from any aggregate, so one dead
// tag page does not silently drop others.
func newNovelNewsForCmd(flags *rootFlags) *cobra.Command {
	var tagSlugs string
	var perStockLimit int
	var scIDsFlag string

	cmd := &cobra.Command{
		Use:   "news-for",
		Short: "Latest tagged news for several stocks at once, fetched in parallel.",
		Long: "Latest tagged news for several stocks at once, fetched in parallel.\n\n" +
			"Pass --sc-ids (comma-separated moneycontrol sc_ids, e.g. RI,INF,ITC).\n" +
			"Tag slugs are derived from the sc_id table; override any of them with\n" +
			"--tag-slugs=reliance-industries,infosys (same order).",
		Example: `  moneycontrol-pp-cli news-for --sc-ids RI,INF,ITC --json
  moneycontrol-pp-cli news-for --sc-ids RI,HDF01 --tag-slugs reliance-industries,hdfc-bank --per-stock-limit 3`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--sc-ids=RI;--per-stock-limit=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "news-for")
			}
			scIDsSrc := scIDsFlag
			if scIDsSrc == "" && len(args) >= 1 {
				scIDsSrc = args[0]
			}
			if scIDsSrc == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("sc_ids is required (pass --sc-ids or a positional, e.g. RI,INF,ITC)"))
			}
			scIDs := splitCSV(scIDsSrc)
			if len(scIDs) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no sc_ids parsed from %q", scIDsSrc))
			}
			overrides := map[string]string{}
			if tagSlugs != "" {
				parts := splitCSV(tagSlugs)
				for i, s := range parts {
					if i < len(scIDs) {
						overrides[scIDs[i]] = s
					}
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				scID    string
				tagSlug string
				news    []articleLink
				err     error
			}
			resCh := make(chan result, len(scIDs))
			var wg sync.WaitGroup
			for _, id := range scIDs {
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					slug := overrides[id]
					if slug == "" {
						slug = deriveTagSlug(id)
					}
					if slug == "" {
						resCh <- result{scID: id, tagSlug: slug, err: fmt.Errorf("no tag slug known for sc_id %q (pass --tag-slugs)", id)}
						return
					}
					news, err := fetchNewsLinks(ctx, c, "/news/tags/"+slug+".html", perStockLimit)
					resCh <- result{scID: id, tagSlug: slug, news: news, err: err}
				}(id)
			}
			wg.Wait()
			close(resCh)

			type entry struct {
				ScID     string        `json:"sc_id"`
				TagSlug  string        `json:"tag_slug"`
				Count    int           `json:"count"`
				News     []articleLink `json:"news"`
				NewsErr  string        `json:"news_error,omitempty"`
			}
			entries := make([]entry, 0, len(scIDs))
			byID := make(map[string]int, len(scIDs))
			for r := range resCh {
				e := entry{ScID: r.scID, TagSlug: r.tagSlug, Count: len(r.news), News: r.news}
				if r.err != nil {
					e.NewsErr = r.err.Error()
				}
				byID[r.scID] = len(entries)
				entries = append(entries, e)
			}
			// Preserve input order.
			ordered := make([]entry, 0, len(scIDs))
			for _, id := range scIDs {
				if i, ok := byID[id]; ok {
					ordered = append(ordered, entries[i])
				}
			}

			view := struct {
				ScIDs    string  `json:"sc_ids"`
				Count    int     `json:"count"`
				Entries  []entry `json:"entries"`
			}{ScIDs: scIDsSrc, Count: len(ordered), Entries: ordered}

			// If EVERY entry failed (e.g. an invalid sc_ids argument with no
			// derivable slugs and no overrides), surface a non-zero exit so
			// callers and the dogfood error-path probe can detect bad input.
			allFailed := true
			for _, e := range ordered {
				if e.NewsErr == "" {
					allFailed = false
					break
				}
			}
			if allFailed && !wantsHumanTable(cmd.OutOrStdout(), flags) {
				_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
				return usageErr(fmt.Errorf("every sc_id failed; check sc_ids %q and --tag-slugs", scIDsSrc))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			for _, e := range ordered {
				if e.NewsErr != "" {
					fmt.Fprintf(out, "%s (tag=%s): unavailable — %s\n", e.ScID, e.TagSlug, e.NewsErr)
					continue
				}
				fmt.Fprintf(out, "%s (tag=%s, %d):\n", e.ScID, e.TagSlug, e.Count)
				for i, h := range e.News {
					fmt.Fprintf(out, "  %d. %s\n     %s\n", i+1, h.Title, h.URL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&scIDsFlag, "sc-ids", "", "comma-separated moneycontrol sc_ids (e.g. RI,INF,ITC); alternatively pass as positional")
	cmd.Flags().StringVar(&tagSlugs, "tag-slugs", "", "comma-separated tag slugs overriding per-sc_id derivation (same order as sc_ids)")
	cmd.Flags().IntVar(&perStockLimit, "per-stock-limit", 5, "max headlines per stock")
	return cmd
}

// splitCSV splits a comma-separated argument, trimming whitespace and dropping
// empties. Used for both --sc_ids positional and --tag-slugs.
func splitCSV(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
