// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

type dealItem struct {
	offerup.StoredListing
	BelowMedianPercent float64 `json:"belowMedianPercent"`
}

type dealsResult struct {
	Query                 string     `json:"query"`
	Location              string     `json:"location,omitempty"`
	Median                float64    `json:"median"`
	BelowThresholdPercent float64    `json:"belowThresholdPercent"`
	MaxDealPrice          float64    `json:"maxDealPrice"`
	Count                 int        `json:"count"`
	Deals                 []dealItem `json:"deals"`
}

// pp:data-source live
func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	var below float64
	cmd := &cobra.Command{
		Use:   "deals <query>",
		Short: "Surface listings priced a chosen percentage below the local median for that item",
		Long: strings.Trim(`
Flag underpriced listings. Searches OfferUp live, computes the local median
asking price for the query, then returns the listings priced at least --below
percent under that median — sorted with the steepest discounts first.`, "\n"),
		Example:     "  offerup-pp-cli deals \"dewalt drill\" --zip 98101 --below 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			result := dealsResult{Query: query, Location: lf.label(), BelowThresholdPercent: below, Deals: []dealItem{}}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			listings, err := searchAndRecord(cmd, flags, lf, query, lf.searchOpts(0))
			if err != nil {
				return err
			}
			stored := offerup.ListingsToStored(listings)
			median := offerup.Median(stored)
			result.Median = median
			if median > 0 {
				result.MaxDealPrice = median * (1 - below/100)
			}
			result.Deals = dealsBelowMedian(stored, median, below)
			result.Count = len(result.Deals)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	cmd.Flags().Float64Var(&below, "below", 20, "Percent below the local median a listing must be priced to count as a deal")
	return cmd
}

// dealsBelowMedian returns listings priced at least `below` percent under the
// median, steepest discount first. Shared by `deals` and `digest`.
func dealsBelowMedian(stored []offerup.StoredListing, median, below float64) []dealItem {
	out := []dealItem{}
	if median <= 0 {
		return out
	}
	threshold := median * (1 - below/100)
	for _, l := range stored {
		if l.Price <= 0 || l.Price > threshold {
			continue
		}
		out = append(out, dealItem{
			StoredListing:      l,
			BelowMedianPercent: round1((median - l.Price) / median * 100),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BelowMedianPercent > out[j].BelowMedianPercent })
	return out
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }
