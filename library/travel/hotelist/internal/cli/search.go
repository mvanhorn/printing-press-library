// Hand-authored `search` command: AI-rated hotels in a city, country, or region,
// sorted by Hotelist Score. Not generated.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var minRating float64
	var maxPrice float64
	var name string
	var sortBy string
	var limit int
	var checkin, checkout string
	var exceptionalOnly bool

	cmd := &cobra.Command{
		Use:   "search <city-or-area>",
		Short: "Find AI-rated hotels in a city, country, or region (sorted by Hotelist Score)",
		Long: "Search Hotelist's AI-rated hotels for a city, country, or continent. " +
			"Results are sorted by Hotelist Score by default. Data is scraped from " +
			"hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli search bangkok
  hotelist-pp-cli search lisbon --min-rating 8 --json
  hotelist-pp-cli search portugal --sort value --limit 10
  hotelist-pp-cli search "new york city" --name marriott`),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<city-or-area>=bangkok"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a city, country, or region is required"))
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

			loc, err := resolveLocation(cmd.Context(), c, db, args[0])
			if err != nil {
				return err
			}

			var extra []apiFilter
			if minRating > 0 {
				extra = append(extra, filterMinRating(minRating))
			}
			if maxPrice > 0 {
				extra = append(extra, filterMaxPrice(maxPrice))
			}

			var sortPtr *apiFilter
			if sf, ok := resolveSort(sortBy); ok {
				sortPtr = &sf
			} else if sortBy != "" {
				return usageErr(fmt.Errorf("unknown --sort %q (use: score, price, price-desc, newest, oldest, value)", sortBy))
			} else {
				sf := filterSort("hotellist_rating", "desc")
				sortPtr = &sf
			}

			note := checkinCheckoutNote(checkin, checkout)

			// exceptionalOnly is applied locally after fetch (the list /api has
			// no exceptional flag); handle via a post-filter render.
			if exceptionalOnly {
				return runHotelQueryFiltered(cmd.Context(), c, db, flags, cmd.OutOrStdout(),
					loc, extra, sortPtr, name, limit, note, func(h hlHotel) bool { return isExceptional(h) })
			}
			return runHotelQuery(cmd.Context(), c, db, flags, cmd.OutOrStdout(),
				loc, extra, sortPtr, name, limit, note)
		},
	}
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only hotels with a Hotelist Score at or above this (0-10)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only hotels at or below this nightly price (base currency)")
	cmd.Flags().StringVar(&name, "name", "", "Filter visible hotels by name substring")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort: score (default), price, price-desc, newest, oldest, value")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum hotels to return")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date (recorded as context only; Hotelist has no dated pricing)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date (recorded as context only; Hotelist has no dated pricing)")
	cmd.Flags().BoolVar(&exceptionalOnly, "exceptional", false, "Only hotels that meet the Exceptional bar (score 8+, built within ~10y)")
	return cmd
}

func trimExample(s string) string {
	// preserve the leading 2-space indent of the first example line
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
