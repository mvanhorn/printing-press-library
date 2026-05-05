// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newSoldCmd(flags *rootFlags) *cobra.Command {
	hf := &homesFlags{status: "sold"}

	cmd := &cobra.Command{
		Use:   "sold",
		Short: "Search sold listings (alias for `homes --status sold`).",
		Long: `Convenience wrapper that runs the gis search with status=sold and
the default sold-time filter (1y/3y/5y window).`,
		Example: `  redfin-pp-cli sold --region-id 30772 --region-type 6 --year-min 2024 --json
  redfin-pp-cli sold --region-slug "city/30772/TX/Austin" --beds-min 4 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			hf.status = "sold"
			opts, oerr := optsFromFlags(hf)
			if oerr != nil {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (status=sold; region required at runtime)")
					return nil
				}
				return oerr
			}
			if dryRunOK(flags) {
				printDryRunGet(cmd, "/stingray/api/gis", redfin.BuildSearchParams(opts))
				return nil
			}
			listings, err := runHomesSearch(cmd, flags, opts, hf.all)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), listings, flags)
		},
	}
	cmd.Flags().Int64Var(&hf.regionID, "region-id", 0, "Numeric Redfin region ID")
	cmd.Flags().IntVar(&hf.regionType, "region-type", 6, "Region type: 1=zip, 2=state, 4=metro, 6=city, 11=neighborhood")
	cmd.Flags().StringVar(&hf.regionSlug, "region-slug", "", "Region slug like 'city/30772/TX/Austin'")
	cmd.Flags().StringVar(&hf.pType, "type", "", "Comma-separated property types: house,condo,townhouse,multi,manufactured,land")
	cmd.Flags().Float64Var(&hf.bedsMin, "beds-min", 0, "Minimum bedrooms")
	cmd.Flags().Float64Var(&hf.bathsMin, "baths-min", 0, "Minimum bathrooms")
	cmd.Flags().IntVar(&hf.priceMin, "price-min", 0, "Minimum price ($)")
	cmd.Flags().IntVar(&hf.priceMax, "price-max", 0, "Maximum price ($)")
	cmd.Flags().IntVar(&hf.sqftMin, "sqft-min", 0, "Minimum sqft")
	cmd.Flags().IntVar(&hf.sqftMax, "sqft-max", 0, "Maximum sqft")
	cmd.Flags().IntVar(&hf.yearMin, "year-min", 0, "Earliest year built")
	cmd.Flags().IntVar(&hf.yearMax, "year-max", 0, "Latest year built")
	cmd.Flags().IntVar(&hf.lotMin, "lot-min", 0, "Minimum lot size (sqft)")
	cmd.Flags().StringVar(&hf.polygon, "polygon", "", "Bounding polygon")
	cmd.Flags().IntVar(&hf.page, "page", 1, "1-indexed page number")
	cmd.Flags().IntVar(&hf.limit, "limit", 50, "Listings per page (max 350)")
	cmd.Flags().BoolVar(&hf.all, "all", false, "Auto-paginate up to 5 pages")
	return cmd
}
