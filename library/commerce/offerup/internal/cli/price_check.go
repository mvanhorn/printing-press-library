// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

// pp:data-source live
func newNovelPriceCheckCmd(flags *rootFlags) *cobra.Command {
	lf := &locFlags{}
	cmd := &cobra.Command{
		Use:   "price-check <query>",
		Short: "See the real going rate for an item in your area — median, 25th/75th percentile, min, max",
		Long: strings.Trim(`
See the real going rate for an item in your area. Searches OfferUp live, then
reports the asking-price distribution (median, p25, p75, min, max, mean) plus
the share of listings marked firm — computed across every matching listing.
No OfferUp page shows this; it requires aggregating the whole result set.`, "\n"),
		Example:     "  offerup-pp-cli price-check \"herman miller aeron\" --zip 85001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			label := lf.label()
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), offerup.ComputePriceStats(query, label, nil), flags)
			}
			listings, err := searchAndRecord(cmd, flags, lf, query, lf.searchOpts(0))
			if err != nil {
				return err
			}
			stats := offerup.ComputePriceStats(query, label, offerup.ListingsToStored(listings))
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().StringVar(&lf.zip, "zip", "", "ZIP code to scope the search (e.g. 98101)")
	cmd.Flags().StringVar(&lf.lat, "lat", "", "Latitude for precise location (use with --lon)")
	cmd.Flags().StringVar(&lf.lon, "lon", "", "Longitude for precise location (use with --lat)")
	cmd.Flags().StringVar(&lf.city, "city", "", "City name for the search location")
	cmd.Flags().StringVar(&lf.state, "state", "", "Two-letter state code (e.g. WA)")
	cmd.Flags().StringVar(&lf.category, "category", "", "OfferUp category id (cid) to scope the search")
	return cmd
}
