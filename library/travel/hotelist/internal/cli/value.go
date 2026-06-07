// Hand-authored `value` command: rating-per-dollar ranking for a location or a
// chain. Not generated.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValueCmd(flags *rootFlags) *cobra.Command {
	var country string
	var minRating, maxPrice float64
	var limit int

	cmd := &cobra.Command{
		Use:   "value <location-or-chain>",
		Short: "Rank hotels by best value (Hotelist rating per dollar) for a place or chain",
		Long: "Rank hotels by rating-per-dollar using Hotelist's own best-value sort. The argument " +
			"is a location (city/country/region) or a chain name/code (e.g. 'Marriott' or 'EM'); narrow " +
			"a chain to a country with --country. Data is scraped from hotelist.com (community/AI-rated, " +
			"not an official API).",
		Example: trimExample(`
  hotelist-pp-cli value tulum
  hotelist-pp-cli value lisbon --max-price 200 --json
  hotelist-pp-cli value marriott --country thailand
  hotelist-pp-cli value "four seasons" --min-rating 8`),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<location-or-chain>=tulum"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a location or chain is required"))
			}

			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			arg := args[0]
			const note = "ranked by Hotelist rating per dollar"

			// If the argument is a recognized chain, rank that chain by value.
			if code, display, ok := normalizeChain(arg); ok {
				loc := &resolvedLocation{Label: display + " (by value)", Kind: "chain"}
				loc.Filters = []apiFilter{filterChain(code)}
				if country != "" {
					cloc, err := resolveLocation(cmd.Context(), c, db, country)
					if err == nil && cloc.Kind == "country" {
						loc.Filters = append(loc.Filters, cloc.Filters...)
						loc.Label = display + " in " + cloc.Label + " (by value)"
					}
				}
				extra := valueExtraFilters(minRating, maxPrice)
				return runValueQuery(cmd.Context(), c, db, flags, cmd.OutOrStdout(), loc, extra, limit, note)
			}

			// Otherwise treat the argument as a location.
			loc, err := resolveLocation(cmd.Context(), c, db, arg)
			if err != nil {
				return err
			}
			loc.Label = loc.Label + " (by value)"
			extra := valueExtraFilters(minRating, maxPrice)
			return runValueQuery(cmd.Context(), c, db, flags, cmd.OutOrStdout(), loc, extra, limit, note)
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "Narrow a chain to a country (only used when the argument is a chain)")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only hotels with a Hotelist Score at or above this (0-10)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only hotels at or below this nightly price")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum hotels to return")
	return cmd
}

func valueExtraFilters(minRating, maxPrice float64) []apiFilter {
	var extra []apiFilter
	if minRating > 0 {
		extra = append(extra, filterMinRating(minRating))
	}
	if maxPrice > 0 {
		extra = append(extra, filterMaxPrice(maxPrice))
	}
	return extra
}
