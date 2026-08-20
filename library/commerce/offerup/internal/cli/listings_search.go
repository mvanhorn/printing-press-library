// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

func newListingsSearchCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	var query string
	var limit int
	var priceMin, priceMax float64
	var firmOnly, localOnly bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search live OfferUp listings by keyword and location (no login)",
		Long: strings.Trim(`
Search OfferUp listings near a location. Returns cleaned listings (id, title,
price, location, condition, firm flag, image, URL) with ads filtered out. Set
the area with --zip (or --lat/--lon); narrow with --price-min/--price-max,
--firm, --local, and --category. Results are also cached to the local store so
price-check, deals, and new-since have data.`, "\n"),
		Example:     "  offerup-pp-cli listings search \"dewalt drill\" --zip 98101 --limit 20",
		Annotations: map[string]string{"pp:endpoint": "listings.search", "pp:method": "GET", "pp:path": "/search", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				query = args[0]
			}
			if query == "" && cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if query == "" {
				return usageErr(errMissingQuery)
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []offerup.Listing{}, flags)
			}
			opts := lf.searchOpts(limit)
			opts.PriceMin = priceMin
			opts.PriceMax = priceMax
			opts.FirmOnly = firmOnly
			opts.LocalOnly = localOnly
			listings, err := searchAndRecord(cmd, flags, lf, query, opts)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), listings, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	cmd.Flags().StringVar(&query, "query", "", "Keyword to search for (or pass as a positional argument)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum listings to return (0 returns all on the page)")
	cmd.Flags().Float64Var(&priceMin, "price-min", 0, "Only listings at or above this price")
	cmd.Flags().Float64Var(&priceMax, "price-max", 0, "Only listings at or below this price")
	cmd.Flags().BoolVar(&firmOnly, "firm", false, "Only listings with a firm (non-negotiable) price")
	cmd.Flags().BoolVar(&localOnly, "local", false, "Only listings offering local pickup")
	return cmd
}
