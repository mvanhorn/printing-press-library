// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

// newProductsSearchCmd is the seeded product search — the endpoint EverBee's own
// Product Analytics search box calls (GET /product_analytics?search_term=).
//
// It is hand-authored rather than spec-emitted on purpose. The generator elects a
// resource's shortest endpoint path as its canonical bulk-sync target, and this
// endpoint sits at the collection root (/product_analytics) while REQUIRING a
// search_term. Declaring it in the spec therefore made a bare `sync` call it with
// no term, which EverBee answers with 400. A seeded search is query-scoped, not an
// enumerable collection: `sync` mirrors the browse feed (`products list`), and the
// search lives here.
func newProductsSearchCmd(flags *rootFlags) *cobra.Command {
	var searchTerm string
	var orderBy string
	var orderDirection string
	var timeRange string
	var page int
	var perPage int

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search EverBee's Etsy listing database by search term, with sales, revenue, tags, and listing type.",
		Long: strings.Trim(`
Search EverBee's Etsy listing database for a term.

This is the endpoint EverBee's Product Analytics search box calls. It is the
answer to "what listings exist for this niche" — unlike 'products list', which
browses EverBee's default feed and ignores any term you give it.

Each listing carries estimated monthly sales and revenue, price, tags, listing
type (physical vs digital download), listing age, conversion rate, and its shop.
Rows are returned as EverBee ranked them; for relevance-annotated, evidence-scored
output use 'research niche' instead.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli products search --search-term "dad shirt" --json
  everbee-pp-cli products search --search-term "dad shirt" --agent --select results.title,results.price,results.listing_type
  everbee-pp-cli products search --search-term "wedding sign" --order-by est_mo_sales --per-page 50 --csv
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "products.search",
			"pp:method":     "GET",
			"pp:path":       research.PathProductSearch,
			"pp:happy-args": "--search-term=dad shirt",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s (search_term=%q)\n", research.PathProductSearch, searchTerm)
				return nil
			}
			// EverBee answers a term-less request with 400. Catch it here so the
			// user gets a usage error naming the flag, not an opaque API error.
			if strings.TrimSpace(searchTerm) == "" && len(args) > 0 {
				searchTerm = strings.Join(args, " ")
			}
			if strings.TrimSpace(searchTerm) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--search-term is required, e.g. --search-term \"dad shirt\""))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"search_term":     searchTerm,
				"type_of_search":  "listings",
				"time_range":      timeRange,
				"order_by":        orderBy,
				"order_direction": orderDirection,
				"page":            strconv.Itoa(page),
				"per_page":        strconv.Itoa(perPage),
			}
			data, err := c.Get(ctx, research.PathProductSearch, params)
			if err != nil {
				return fmt.Errorf("searching products for %q: %w", searchTerm, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&searchTerm, "search-term", "", "Product search term, e.g. \"dad shirt\" (required)")
	cmd.Flags().StringVar(&orderBy, "order-by", "est_mo_revenue", "Sort field: est_mo_revenue, est_mo_sales, price, growth_rate, or review_count")
	cmd.Flags().StringVar(&orderDirection, "order-direction", "desc", "Sort direction: desc or asc")
	cmd.Flags().StringVar(&timeRange, "time-range", "last_1_month", "Metric window for sales/revenue estimates")
	cmd.Flags().IntVar(&page, "page", 1, "Result page")
	cmd.Flags().IntVar(&perPage, "per-page", 20, "Results per page")
	return cmd
}
