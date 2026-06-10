// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: `deal` command — rank live search results by under-market gap.

package cli

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"

	"github.com/spf13/cobra"
)

// Deal is a ranked listing with the under-market gap when it could be computed.
type Deal struct {
	motohunt.Listing
	BaseMSRP string  `json:"base_msrp,omitempty"`
	ALP      string  `json:"alp,omitempty"`
	GapPct   float64 `json:"gap_pct,omitempty"` // (alp - ask)/alp * 100; only set when enriched and both known
	Enriched bool    `json:"enriched"`
}

// dealRatingRank orders deal ratings best-first.
func dealRatingRank(r string) int {
	switch r {
	case "Great Price":
		return 0
	case "Good Price":
		return 1
	case "Fair Price":
		return 2
	case "High Price":
		return 4
	default:
		return 3 // unrated sits between fair and high
	}
}

func newDealCmd(flags *rootFlags) *cobra.Command {
	var (
		location, mk, style, model, state, sortFlag string
		limit, maxPages, enrich                     int
	)

	cmd := &cobra.Command{
		Use:   "deal",
		Short: "Rank search results by deal quality (deal rating, then price); optionally enrich with ALP gap",
		Long: `Run a live search and rank the matched listings by how good a deal they are:
deal_rating first (Great Price > Good Price > Fair Price > unrated > High Price),
then by ascending price within a rating.

Cards don't carry MSRP/ALP, so the under-market gap is computed only for the top
--enrich listings by calling 'get' to pull base_msrp/alp/price, then gap_pct =
(alp - ask) / alp * 100. Under the dogfood harness the enrich count is curtailed.
Mirrors 'search' facet/sort/paging flags.`,
		Example:     "  motohunt-pp-cli deal --make Harley-Davidson --location 33705 --limit 15 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if sortFlag != "" && !validSort(sortFlag) {
				return usageErr(fmt.Errorf("--sort %q invalid: use t, p, a, or c", sortFlag))
			}
			if dryRunOK(flags) {
				url, applied, _ := site.BuildSearchURL(motohunt.SearchParams{
					Location: location, Make: mk, Model: model, Style: style, State: state, Sort: sortFlag,
				})
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s (facet: %s), rank %d, enrich top %d\n", url, applied, limit, enrich)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), make([]Deal, 0), flags)
			}

			if limit <= 0 {
				limit = cardsPerPage
			}
			if maxPages <= 0 {
				maxPages = 5
			}
			// Default sort to best-deal when the caller didn't pick one.
			effSort := sortFlag
			if effSort == "" {
				effSort = "c"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			client := scrapeClient(flags)

			// Collect cards via the same paging logic as search.
			collected := make([]motohunt.Listing, 0, limit)
			curStart := 0
			for pages := 0; pages < maxPages && len(collected) < limit; pages++ {
				url, _, _ := site.BuildSearchURL(motohunt.SearchParams{
					Location: location, Make: mk, Model: model, Style: style, State: state, Sort: effSort, Start: curStart,
				})
				doc, ferr := client.Fetch(ctx, url)
				if ferr != nil {
					if len(collected) > 0 {
						break
					}
					return apiErr(ferr)
				}
				cards := motohunt.ParseCards(doc, site)
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

			deals := make([]Deal, 0, len(collected))
			for _, l := range collected {
				deals = append(deals, Deal{Listing: l})
			}

			// Rank: deal_rating, then ascending price.
			sort.SliceStable(deals, func(i, j int) bool {
				ri, rj := dealRatingRank(deals[i].DealRating), dealRatingRank(deals[j].DealRating)
				if ri != rj {
					return ri < rj
				}
				pi, pj := parsePrice(deals[i].Price), parsePrice(deals[j].Price)
				// Known prices sort ascending; unknown prices sink.
				if (pi == 0) != (pj == 0) {
					return pi != 0
				}
				return pi < pj
			})

			// Enrich the top N by pulling ALP/MSRP from the detail page.
			enrichN := enrich
			if enrichN > len(deals) {
				enrichN = len(deals)
			}
			if cliutil.IsDogfoodEnv() && enrichN > 3 {
				enrichN = 3
				fmt.Fprintln(os.Stderr, "note: dogfood env — enrich capped at 3")
			}
			for i := 0; i < enrichN; i++ {
				if deals[i].ID == "" {
					continue
				}
				doc, ferr := client.Fetch(ctx, site.DetailURL(deals[i].ID))
				if ferr != nil {
					fmt.Fprintf(os.Stderr, "warning: enrich %s failed: %v\n", deals[i].ID, ferr)
					continue
				}
				d := motohunt.ParseDetail(doc, site, deals[i].ID)
				deals[i].BaseMSRP = d.BaseMSRP
				deals[i].ALP = d.ALP
				// Only mark enriched when price-research data was actually present;
				// MotoHunt omits the block on some listings. A truthful flag keeps
				// `enriched == true` consumers from getting false positives and keeps
				// data-less rows out of the gap-based re-rank.
				deals[i].Enriched = d.ALP != "" || d.BaseMSRP != ""
				if deals[i].Price == "" && d.Price != "" {
					deals[i].Price = d.Price
				}
				if d.DealRating != "" && deals[i].DealRating == "" {
					deals[i].DealRating = d.DealRating
				}
				alp := parsePrice(d.ALP)
				ask := parsePrice(deals[i].Price)
				if alp > 0 && ask > 0 {
					deals[i].GapPct = roundTo((alp-ask)/alp*100, 2)
				}
			}

			// Re-rank enriched rows by gap so the biggest under-ALP deals float up,
			// while keeping unenriched rows in deal-rating order below them.
			sort.SliceStable(deals, func(i, j int) bool {
				if deals[i].Enriched != deals[j].Enriched {
					return deals[i].Enriched
				}
				if deals[i].Enriched && deals[j].Enriched && deals[i].GapPct != deals[j].GapPct {
					return deals[i].GapPct > deals[j].GapPct
				}
				ri, rj := dealRatingRank(deals[i].DealRating), dealRatingRank(deals[j].DealRating)
				return ri < rj
			})

			fmt.Fprintf(os.Stderr, "ranked %d listings (%d enriched with ALP gap)\n", len(deals), enrichN)
			return printDomainJSON(cmd.OutOrStdout(), deals, flags)
		},
	}

	cmd.Flags().StringVar(&location, "location", "", "US ZIP code")
	cmd.Flags().StringVar(&mk, "make", "", "Make facet")
	cmd.Flags().StringVar(&style, "style", "", "Style facet")
	cmd.Flags().StringVar(&model, "model", "", "Model facet")
	cmd.Flags().StringVar(&state, "state", "", "State facet")
	cmd.Flags().StringVar(&sortFlag, "sort", "", "Sort: t|p|a|c (default c=best-deal)")
	cmd.Flags().IntVar(&limit, "limit", cardsPerPage, "Max listings to rank")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Max pages to fetch")
	cmd.Flags().IntVar(&enrich, "enrich", 5, "Enrich the top N with ALP/MSRP via per-id detail fetch (0 to skip)")

	return cmd
}

// parsePrice turns "$24,999" into 24999.0; returns 0 when unparseable/empty.
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == '.' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0
	}
	return v
}

func roundTo(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	// math.Round rounds half away from zero, so negative gaps (ask above ALP)
	// round correctly; the old int64(v*p+0.5) truncated negatives toward zero.
	return math.Round(v*p) / p
}
