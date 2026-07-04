// Hand-authored transcendence command: rank-country. A national rating-per-
// dollar leaderboard the single-map website UI cannot express. Not generated.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelRankCountryCmd(flags *rootFlags) *cobra.Command {
	var minRating, maxPrice float64
	var amenities string
	var sortBy string
	var top int

	cmd := &cobra.Command{
		Use:   "rank-country <country>",
		Short: "Rank a whole country's hotels by Hotelist rating-per-dollar, with compound filters",
		Long: "Rank the best hotels across an entire country by rating-per-dollar (or another sort), " +
			"applying compound amenity, rating, and price filters in one pass. The website's map UI is " +
			"bounded to one viewport; this scans the country in a single call. Use 'corridor' for a " +
			"user-defined multi-city route, or 'chain-compare' to compare brands. Data is scraped from " +
			"hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli rank-country thailand --top 10
  hotelist-pp-cli rank-country portugal --min-rating 8 --max-price 150 --amenities pool,coworking --json
  hotelist-pp-cli rank-country japan --sort score`),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<country>=portugal"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a country is required"))
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
			loc.Label = loc.Label + " — value leaderboard"

			var extra []apiFilter
			for _, a := range splitCSV(amenities) {
				if label := amenityLabel(a); label != "" {
					extra = append(extra, filterAmenity(label))
				}
			}
			if minRating > 0 {
				extra = append(extra, filterMinRating(minRating))
			}
			if maxPrice > 0 {
				extra = append(extra, filterMaxPrice(maxPrice))
			}

			sortF := filterSort("best-value", "desc")
			localResort := "value"
			if sortBy != "" {
				sf, ok := resolveSort(sortBy)
				if !ok {
					return usageErr(fmt.Errorf("unknown --sort %q (use: value, score, price, price-desc, newest, oldest)", sortBy))
				}
				sortF = sf
				localResort = sortBy
			}

			filters := append([]apiFilter{}, loc.Filters...)
			filters = append(filters, extra...)
			filters = append(filters, sortF)

			hotels, err := adaptiveFetch(cmd.Context(), c, filters, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hotels = dedupeHotels(hotels)
			storeHotels(db, hotels)

			// The upstream endpoint ignores the amenities filter param, so
			// enforce --amenities locally against each hotel's pros/cons text.
			hotels = filterByAmenities(hotels, splitCSV(amenities))

			if maxPrice > 0 {
				hotels = dropUnpriced(hotels) // a price ceiling cannot be met by unknown-price hotels
			}
			// Local re-sort guarantees the value ordering even though the
			// upstream sort already approximates it.
			switch localResort {
			case "value", "best-value":
				hotels = dropUnpriced(hotels) // value ranking requires a real price
				sortHotelsByValue(hotels)
			case "score", "rating":
				sortHotelsByRating(hotels)
			}

			view := buildView(loc.Label, hotels, top, loc.Note)
			return printHotelView(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only hotels with a Hotelist Score at or above this (0-10)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only hotels at or below this nightly price")
	cmd.Flags().StringVar(&amenities, "amenities", "", "Comma-separated photo-verified amenities to require")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort: value (default), score, price, price-desc, newest, oldest")
	cmd.Flags().IntVar(&top, "top", 20, "Number of top hotels to return")
	return cmd
}
