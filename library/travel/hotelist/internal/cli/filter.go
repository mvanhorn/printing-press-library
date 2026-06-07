// Hand-authored `filter` command: photo-verified amenity filtering in a
// location. Not generated.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newFilterCmd(flags *rootFlags) *cobra.Command {
	var gymWeights, pool, tennis bool
	var amenityList string
	var minRating, maxPrice, minPrice float64
	var chain string
	var boutique, collection bool
	var builtAfter int
	var sortBy string
	var limit int

	cmd := &cobra.Command{
		Use:   "filter <location>",
		Short: "Filter hotels by photo-verified amenities (real gym, pool, tennis, and more)",
		Long: "Filter a location's hotels on Hotelist's photo-verified amenities — the AI " +
			"cross-checks claimed amenities against actual room photos, so --gym-weights means a " +
			"real weightlifting gym, not one treadmill in a closet. Multiple amenities are ANDed. " +
			"Data is scraped from hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli filter lisbon --gym-weights --pool
  hotelist-pp-cli filter bangkok --amenity coworking,sauna --min-rating 8 --json
  hotelist-pp-cli filter tokyo --tennis --max-price 300
  hotelist-pp-cli filter italy --chain marriott --built-after 2018`),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<location>=lisbon"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a location (city, country, or region) is required"))
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
			var applied []string
			var wantedAmenities []string
			if gymWeights {
				extra = append(extra, filterAmenity("Weightlifting gym"))
				applied = append(applied, "Weightlifting gym")
				wantedAmenities = append(wantedAmenities, "weightlifting")
			}
			if pool {
				extra = append(extra, filterAmenity("Pool"))
				applied = append(applied, "Pool")
				wantedAmenities = append(wantedAmenities, "pool")
			}
			if tennis {
				extra = append(extra, filterAmenity("Tennis court"))
				applied = append(applied, "Tennis court")
				wantedAmenities = append(wantedAmenities, "tennis")
			}
			for _, a := range splitCSV(amenityList) {
				label := amenityLabel(a)
				if label != "" {
					extra = append(extra, filterAmenity(label))
					applied = append(applied, label)
					wantedAmenities = append(wantedAmenities, a)
				}
			}
			if minRating > 0 {
				extra = append(extra, filterMinRating(minRating))
			}
			if minPrice > 0 {
				extra = append(extra, filterMinPrice(minPrice))
			}
			if maxPrice > 0 {
				extra = append(extra, filterMaxPrice(maxPrice))
			}
			if builtAfter > 0 {
				extra = append(extra, filterBuiltAfter(builtAfter))
			}
			if boutique {
				extra = append(extra, apiFilter{Target: "boutique", Value: "1", Type: "exact-match"})
			}
			if collection {
				extra = append(extra, apiFilter{Target: "collection", Value: "1", Type: "exact-match"})
			}
			if chain != "" {
				code, _, ok := normalizeChain(chain)
				if !ok {
					return usageErr(fmt.Errorf("unknown chain %q (try a name like 'Marriott' or a code like 'EM')", chain))
				}
				extra = append(extra, filterChain(code))
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

			note := ""
			var keep func(hlHotel) bool
			if len(applied) > 0 {
				note = "amenities matched against AI review text (best-effort; Hotelist's /api does not expose its photo-verified amenity filter): " + strings.Join(applied, ", ")
				// Upstream ignores the amenities filter param, so enforce the
				// requested amenities locally against each hotel's pros/cons.
				want := wantedAmenities
				keep = func(h hlHotel) bool { return matchesAmenities(h, want) }
			}
			return runHotelQueryFiltered(cmd.Context(), c, db, flags, cmd.OutOrStdout(),
				loc, extra, sortPtr, "", limit, note, keep)
		},
	}
	cmd.Flags().BoolVar(&gymWeights, "gym-weights", false, "Require a real weightlifting gym (verified against photos)")
	cmd.Flags().BoolVar(&pool, "pool", false, "Require a pool (verified against photos)")
	cmd.Flags().BoolVar(&tennis, "tennis", false, "Require a tennis court (verified against photos)")
	cmd.Flags().StringVar(&amenityList, "amenity", "", "Comma-separated amenities to require, e.g. coworking,sauna,bath")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only hotels with a Hotelist Score at or above this (0-10)")
	cmd.Flags().Float64Var(&minPrice, "min-price", 0, "Only hotels at or above this nightly price")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only hotels at or below this nightly price")
	cmd.Flags().StringVar(&chain, "chain", "", "Restrict to a hotel chain (name like 'Hilton' or code like 'EH')")
	cmd.Flags().BoolVar(&boutique, "boutique", false, "Only independent / boutique hotels")
	cmd.Flags().BoolVar(&collection, "collection", false, "Only small-luxury collection hotels")
	cmd.Flags().IntVar(&builtAfter, "built-after", 0, "Only hotels built after this year")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort: score (default), price, price-desc, newest, oldest, value")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum hotels to return")
	return cmd
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
