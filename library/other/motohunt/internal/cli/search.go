// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: rich goquery `search` command (parsed cards + auto-paging).

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"

	"github.com/spf13/cobra"
)

const cardsPerPage = 24

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		q, location, mk, style, model, state, sort string
		start, limit, maxPages                     int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search listings as parsed cards (title, price, mileage, badges, deal rating, location, dealer)",
		Long: `Search motorcycle (motohunt.com) or ATV/UTV (atvhunt.com via --site atv) listings,
returning fully parsed cards as a JSON array.

Facets ride the URL as a SINGLE path segment; the site honors only one. When
multiple facet flags are given, priority is: --make (+--model as "Make-Model")
> --model > --style > --state. The applied facet and any ignored ones are noted
on stderr. Other knobs (--q, --location, --sort) ride as query params.

Pagination: 24 cards per page via ?start=N. The command auto-pages (start += 24)
until --limit cards are collected, --max-pages is hit, or a page returns 0 cards.`,
		Example: `  motohunt-pp-cli search --make Harley-Davidson --location 33705 --sort c --limit 30 --agent
  motohunt-pp-cli --site atv search --location 33705 --limit 10 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if sort != "" && !validSort(sort) {
				return usageErr(fmt.Errorf("--sort %q invalid: use t (recent), p (high$), a (low$), or c (best deal)", sort))
			}
			if dryRunOK(flags) {
				url, applied, _ := site.BuildSearchURL(motohunt.SearchParams{
					Q: q, Location: location, Make: mk, Model: model, Style: style, State: state, Sort: sort, Start: start,
				})
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s (facet: %s, up to %d cards across %d pages)\n", url, applied, limit, maxPages)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would search (verify env)")
				return printJSONFiltered(cmd.OutOrStdout(), make([]motohunt.Listing, 0), flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			client := scrapeClient(flags)

			if limit <= 0 {
				limit = cardsPerPage
			}
			if maxPages <= 0 {
				maxPages = 5
			}

			collected := make([]motohunt.Listing, 0, limit)
			scanned := 0
			pages := 0
			curStart := start
			appliedFacet := ""
			var ignored []string

			for pages < maxPages && len(collected) < limit {
				url, applied, ign := site.BuildSearchURL(motohunt.SearchParams{
					Q: q, Location: location, Make: mk, Model: model, Style: style, State: state, Sort: sort, Start: curStart,
				})
				appliedFacet, ignored = applied, ign
				doc, ferr := client.Fetch(ctx, url)
				if ferr != nil {
					if len(collected) > 0 {
						fmt.Fprintf(os.Stderr, "warning: page %d fetch failed (%v); returning %d cards collected so far\n", pages+1, ferr, len(collected))
						break
					}
					return apiErr(ferr)
				}
				cards := motohunt.ParseCards(doc, site)
				pages++
				scanned += len(cards)
				if len(cards) == 0 {
					break
				}
				for _, c := range cards {
					collected = append(collected, c)
					if len(collected) >= limit {
						break
					}
				}
				curStart += cardsPerPage
			}

			// Provenance + paging notes to stderr (kept off stdout so JSON stays an array).
			fmt.Fprintf(os.Stderr, "scanned %d cards across %d page(s); returning %d (facet applied: %s)\n",
				scanned, pages, len(collected), facetOrNone(appliedFacet))
			if len(ignored) > 0 {
				fmt.Fprintf(os.Stderr, "note: ignored conflicting facets (site honors one path segment): %v\n", ignored)
			}

			return printDomainJSON(cmd.OutOrStdout(), collected, flags)
		},
	}

	cmd.Flags().StringVar(&q, "q", "", "Free-text query, e.g. 'softail'")
	cmd.Flags().StringVar(&location, "location", "", "US ZIP code; results sort by distance")
	cmd.Flags().StringVar(&mk, "make", "", "Make facet, e.g. Harley-Davidson (see 'makes')")
	cmd.Flags().StringVar(&style, "style", "", "Style facet, e.g. Cruiser, Sport, Touring")
	cmd.Flags().StringVar(&model, "model", "", "Model facet; combined with --make as Make-Model (see 'models')")
	cmd.Flags().StringVar(&state, "state", "", "US state facet, e.g. Florida")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort: t=recent, p=high$, a=low$, c=best-deal")
	cmd.Flags().IntVar(&start, "start", 0, "Pagination start offset (0, 24, 48, ...)")
	cmd.Flags().IntVar(&limit, "limit", cardsPerPage, "Max cards to collect")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Max pages to fetch (24 cards/page)")

	return cmd
}

func validSort(s string) bool {
	switch s {
	case "t", "p", "a", "c":
		return true
	}
	return false
}

func facetOrNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
