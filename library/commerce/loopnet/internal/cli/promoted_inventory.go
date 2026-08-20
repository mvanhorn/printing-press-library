// LoopNet live inventory search. Hand-written: replaces the generated
// generic HTML-page command with real schema.org JSON-LD listing
// extraction. `inventory` is a live look — it does not write the store;
// use `sync` to persist a submarket.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

func newInventoryPromotedCmd(flags *rootFlags) *cobra.Command {
	var (
		market   lnMarketFlags
		limit    int
		minPrice int
		maxPrice int
		minSize  int
		maxSize  int
	)

	cmd := &cobra.Command{
		Use:   "inventory <location>",
		Short: "Search LoopNet inventory live for a location, property type, and sale/lease",
		Long: `Search LoopNet commercial real estate inventory for a location.

inventory is a live look at LoopNet — it fetches and parses search-result
pages and prints the listings. It does not write the local store; run 'sync'
to persist a submarket and unlock the history-based intelligence commands.

Prerequisite: this live fetch needs Akamai clearance cookies. Run
'loopnet-pp-cli auth refresh' first (or 'auth set'); check 'doctor' or
'auth status' to confirm they are present.`,
		Example: `  loopnet-pp-cli inventory worcester-ma --type industrial --listing for-sale
  loopnet-pp-cli inventory "Los Angeles, CA" --type office --limit 50 --json
  loopnet-pp-cli inventory boston-ma --type retail --max-price 2000000`,
		Annotations: map[string]string{
			"pp:endpoint": "inventory.listings", "pp:method": "GET",
			"pp:path": "/search/{property_type}/{location}/{listing_type}/", "mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			location := loopnet.SlugLocation(args[0])
			pt := loopnet.NormalizeType(market.propertyType)
			lt := loopnet.NormalizeListingType(market.listingType)
			filters := loopnet.SearchFilters{
				MinPrice: minPrice, MaxPrice: maxPrice, MinSize: minSize, MaxSize: maxSize,
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch: %s\n",
					loopnet.BuildSearchURL(pt, location, lt, 1, filters))
				return nil
			}
			if location == "" {
				return usageErr(fmt.Errorf("location is required (e.g. 'worcester-ma' or \"Los Angeles, CA\")"))
			}

			if limit <= 0 {
				limit = 25
			}

			var listings []loopnet.Listing
			total := 0
			for page := 1; page <= 20 && len(listings) < limit; page++ {
				res, err := lnFetchSearch(flags, pt, location, lt, page, filters)
				if err != nil {
					if page == 1 {
						return lnClassifyFetchError(fmt.Errorf("searching LoopNet: %w", err), flags)
					}
					break
				}
				if res.TotalResults > total {
					total = res.TotalResults
				}
				for _, l := range res.Listings {
					l.PropertyType, l.ListingType, l.Location = pt, lt, location
					listings = append(listings, l)
				}
				if !res.HasNextPage || dogfoodCurtail() {
					break
				}
			}
			if len(listings) > limit {
				listings = listings[:limit]
			}
			if len(listings) == 0 {
				return notFoundErr(fmt.Errorf("no listings found for %s (%s, %s)", location, pt, lt))
			}

			result := map[string]any{
				"location":      location,
				"property_type": pt,
				"listing_type":  lt,
				"total_results": total,
				"returned":      len(listings),
				"listings":      listings,
			}
			return flags.printJSON(cmd, result)
		},
	}

	addMarketFlags(cmd, &market)
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum listings to return")
	cmd.Flags().IntVar(&minPrice, "min-price", 0, "Filter: minimum asking price")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Filter: maximum asking price")
	cmd.Flags().IntVar(&minSize, "min-size", 0, "Filter: minimum building size (SF)")
	cmd.Flags().IntVar(&maxSize, "max-size", 0, "Filter: maximum building size (SF)")
	return cmd
}
