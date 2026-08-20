// LoopNet sync command. Hand-written (replaces the generic generated sync):
// LoopNet has no JSON API, so syncing means walking search-result pages,
// extracting JSON-LD listings, optionally fetching each listing's detail
// page, persisting both grains to the local store, and appending one
// price observation per listing so repeated syncs build a time series.
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		market    lnMarketFlags
		pages     int
		limit     int
		noDetails bool
		minPrice  int
		maxPrice  int
		minSize   int
		maxSize   int
	)

	cmd := &cobra.Command{
		Use:   "sync <location>",
		Short: "Sync a LoopNet submarket into the local store, building price history",
		Long: `Sync pulls a LoopNet submarket into the local SQLite store.

It walks the search-result pages for a location, extracts every listing, and
(unless --no-details) fetches each listing's detail page for the full fact
sheet. Every sync also appends one price observation per listing — run sync
repeatedly over time and the intelligence commands (price-cuts, dom, velocity,
delisted) gain the history LoopNet itself never exposes.

Prerequisite: live fetches need Akamai clearance cookies. Run
'loopnet-pp-cli auth refresh' first (or 'auth set'); check 'doctor' or
'auth status' to confirm they are present.`,
		Example: `  loopnet-pp-cli sync worcester-ma --type industrial --listing for-sale
  loopnet-pp-cli sync "Los Angeles, CA" --type office --pages 5
  loopnet-pp-cli sync boston-ma --no-details --limit 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				location := loopnet.SlugLocation(args[0])
				pt := loopnet.NormalizeType(market.propertyType)
				lt := loopnet.NormalizeListingType(market.listingType)
				fmt.Fprintf(cmd.OutOrStdout(), "would sync %s: %s\n", location,
					loopnet.BuildSearchURL(pt, location, lt, 1, loopnet.SearchFilters{}))
				return nil
			}

			location := loopnet.SlugLocation(args[0])
			if location == "" {
				return usageErr(fmt.Errorf("location is required (e.g. 'worcester-ma' or \"Los Angeles, CA\")"))
			}
			pt := loopnet.NormalizeType(market.propertyType)
			lt := loopnet.NormalizeListingType(market.listingType)
			filters := loopnet.SearchFilters{
				MinPrice: minPrice, MaxPrice: maxPrice, MinSize: minSize, MaxSize: maxSize,
			}

			if pages < 1 {
				pages = 1
			}
			if dogfoodCurtail() {
				// Akamai pacing means each fetch costs ~10s; keep the
				// live-dogfood matrix well under its per-command timeout.
				pages = 1
				limit = 1
			}

			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			observedAt := time.Now().UTC()
			var listings []loopnet.Listing
			seen := map[string]bool{}
			totalResults := 0

			for page := 1; page <= pages; page++ {
				res, err := lnFetchSearch(flags, pt, location, lt, page, filters)
				if err != nil {
					if page == 1 {
						return lnClassifyFetchError(fmt.Errorf("fetching search page 1: %w", err), flags)
					}
					fmt.Fprintf(os.Stderr, "warning: search page %d failed: %v\n", page, err)
					break
				}
				if res.TotalResults > totalResults {
					totalResults = res.TotalResults
				}
				for _, l := range res.Listings {
					if l.ID == "" || seen[l.ID] {
						continue
					}
					seen[l.ID] = true
					l.PropertyType = pt
					l.ListingType = lt
					l.Location = location
					listings = append(listings, l)
				}
				if !res.HasNextPage {
					break
				}
			}

			if len(listings) == 0 {
				return notFoundErr(fmt.Errorf("no listings found for %s (%s, %s) — check the location slug", location, pt, lt))
			}

			obs := make([]lnObservation, 0, len(listings))
			for _, l := range listings {
				if err := lnSaveListing(st, l); err != nil {
					fmt.Fprintf(os.Stderr, "warning: storing listing %s: %v\n", l.ID, err)
				}
				obs = append(obs, lnObservation{
					ListingID: l.ID, Location: location, PropertyType: pt, ListingType: lt,
					Price: l.Price, HasPrice: l.Price > 0, ObservedAt: observedAt,
				})
			}
			if err := recordObservations(st.DB(), obs); err != nil {
				fmt.Fprintf(os.Stderr, "warning: recording observations: %v\n", err)
			}

			propertiesSynced, detailErrors := 0, 0
			if !noDetails {
				detailLimit := limit
				if detailLimit <= 0 || detailLimit > len(listings) {
					detailLimit = len(listings)
				}
				for i := 0; i < detailLimit; i++ {
					l := listings[i]
					prop, err := lnFetchDetail(flags, l.ID)
					if err != nil {
						detailErrors++
						fmt.Fprintf(os.Stderr, "warning: detail fetch %s failed: %v\n", l.ID, err)
						continue
					}
					prop.PropertyType = pt
					prop.ListingType = lt
					prop.Location = location
					if prop.Price == 0 {
						prop.Price = l.Price
					}
					if err := lnSaveProperty(st, *prop); err != nil {
						fmt.Fprintf(os.Stderr, "warning: storing property %s: %v\n", l.ID, err)
						continue
					}
					propertiesSynced++
				}
			}

			summary := map[string]any{
				"location":          location,
				"property_type":     pt,
				"listing_type":      lt,
				"observed_at":       observedAt.Format(time.RFC3339),
				"search_pages":      pages,
				"listings_synced":   len(listings),
				"properties_synced": propertiesSynced,
				"detail_errors":     detailErrors,
				"total_results":     totalResults,
			}
			return flags.printJSON(cmd, summary)
		},
	}

	addMarketFlags(cmd, &market)
	cmd.Flags().IntVar(&pages, "pages", 2, "Number of search-result pages to walk")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max listings to fetch detail pages for (0 = all). Each fetch is paced ~10s for Akamai.")
	cmd.Flags().BoolVar(&noDetails, "no-details", false, "Skip per-listing detail-page fetches (faster, fewer fields)")
	cmd.Flags().IntVar(&minPrice, "min-price", 0, "Filter: minimum asking price")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Filter: maximum asking price")
	cmd.Flags().IntVar(&minSize, "min-size", 0, "Filter: minimum building size (SF)")
	cmd.Flags().IntVar(&maxSize, "max-size", 0, "Filter: maximum building size (SF)")
	return cmd
}
