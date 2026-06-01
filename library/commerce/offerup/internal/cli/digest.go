// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

type digestResult struct {
	Query      string                  `json:"query"`
	Location   string                  `json:"location,omitempty"`
	Since      string                  `json:"since"`
	PriceStats offerup.PriceStats      `json:"priceStats"`
	NewCount   int                     `json:"newCount"`
	New        []offerup.StoredListing `json:"new"`
	DropCount  int                     `json:"dropCount"`
	Drops      []offerup.Drop          `json:"priceDrops"`
	DealCount  int                     `json:"dealCount"`
	Deals      []dealItem              `json:"deals"`
}

// pp:data-source auto
func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	var since string
	var below float64
	cmd := &cobra.Command{
		Use:   "digest <query>",
		Short: "A single composite report for a saved search combining what's new, what dropped in price, and what's underpriced",
		Long: strings.Trim(`
The whole watch in one call. Searches OfferUp live, then returns a composite
report for the query: the price distribution, listings new within --since,
listings whose price dropped within --since, and listings priced below the
local median — everything an agent watching a market needs from one tool call.`, "\n"),
		Example:     "  offerup-pp-cli digest \"snowboard\" --since 24h --zip 98101 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			_, cutoff, err := sinceCutoff(since)
			if err != nil {
				return usageErr(err)
			}
			result := digestResult{
				Query: query, Location: lf.label(), Since: sinceLabel(since, "24h"),
				New: []offerup.StoredListing{}, Drops: []offerup.Drop{}, Deals: []dealItem{},
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			client := newOfferupClient(flags)
			listings, err := client.Search(cmd.Context(), query, lf.searchOpts(0))
			if err != nil {
				return classifyOfferupError(err)
			}
			key := lf.storeKey(query)
			st, err := openOfferupStore()
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if _, err := st.RecordSearch(key, listings); err != nil {
				return apiErr(err)
			}
			stored := offerup.ListingsToStored(listings)
			result.PriceStats = offerup.ComputePriceStats(query, lf.label(), stored)
			result.Deals = dealsBelowMedian(stored, result.PriceStats.Median, below)
			result.DealCount = len(result.Deals)
			if fresh, err := st.NewSince(key, cutoff); err == nil && fresh != nil {
				result.New = fresh
			}
			result.NewCount = len(result.New)
			if drops, err := st.Drops(key, cutoff); err == nil && drops != nil {
				result.Drops = drops
			}
			result.DropCount = len(result.Drops)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	cmd.Flags().StringVar(&since, "since", "24h", "Window for new listings and price drops (e.g. 24h, 7d, 1w)")
	cmd.Flags().Float64Var(&below, "below", 20, "Percent below the local median a listing must be priced to count as a deal")
	return cmd
}
