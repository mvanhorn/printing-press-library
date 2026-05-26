// Copyright 2026 kothari-nikunj. Licensed under Apache-2.0. See LICENSE.

// hotels <location> <checkin> <checkout> — the headline command.
// Hand-written: lives outside the generator's emit set so re-running
// /printing-press leaves this file alone.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newHotelsCmd(flags *rootFlags) *cobra.Command {
	var opts hotelSearchOpts
	var childAgesCSV, hotelClassCSV string
	var noSnapshots bool

	cmd := &cobra.Command{
		Use:   "hotels <location> <checkin> <checkout>",
		Short: "Search Google Hotels by location and date range",
		Long: `Search Google Hotels by location and date range.

Returns per-result OTA price breakdown, deep booking links, and standard
hotel metadata (name, brand, address, lat/lng, hotel_class, rating,
reviews, amenities, images, description).

All filter flags are applied client-side after the SSR fetch; brand and
amenity filters in particular run against parsed records since Google's
URL surface doesn't accept them.`,
		Example: strings.Trim(`
  hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17
  hotel-goat-pp-cli hotels "Paris" 2026-07-15 2026-07-20 --sort cheapest --max-price 300
  hotel-goat-pp-cli hotels "Tokyo" 2026-09-01 2026-09-05 --brand Hyatt,Marriott --min-rating 4.0
  hotel-goat-pp-cli hotels "Lisbon" 2026-08-20 2026-08-23 --agent --select results.name,results.rating,results.price_per_night,results.booking_urls.primary
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 3 {
				return cmd.Help()
			}
			location, checkin, checkout := args[0], args[1], args[2]
			if err := validateYYYYMMDD("checkin", checkin); err != nil {
				return err
			}
			if err := validateYYYYMMDD("checkout", checkout); err != nil {
				return err
			}
			opts.ChildAges = parseChildAges(childAgesCSV)
			opts.HotelClass = parseIntCSV(hotelClassCSV)

			results, wouldURL, err := fetchAndParseHotels(cmd.Context(), location, checkin, checkout, opts)
			if err != nil {
				return apiErr(err)
			}
			env := newEnvelope(wouldURL, results)
			env.Meta["location"] = location
			env.Meta["checkin"] = checkin
			env.Meta["checkout"] = checkout
			// Append snapshots best-effort. Skipped when --no-snapshots
			// or when running under verify (no real data).
			if !noSnapshots {
				recordSnapshotsForResults(cmd.Context(), results, checkin, checkout, cmd.ErrOrStderr())
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}

	cmd.Flags().IntVar(&opts.Adults, "adults", 2, "Number of adult guests")
	cmd.Flags().IntVar(&opts.Children, "children", 0, "Number of children")
	cmd.Flags().StringVar(&childAgesCSV, "child-ages", "", "Comma-separated child ages (e.g. 5,8,12)")
	cmd.Flags().IntVar(&opts.Rooms, "rooms", 1, "Number of rooms")
	cmd.Flags().StringVar(&opts.Currency, "currency", "", "ISO 4217 currency code (e.g. USD, EUR)")
	cmd.Flags().StringVar(&opts.Sort, "sort", "best", "Sort order: cheapest, best, rating, reviews")
	cmd.Flags().Float64Var(&opts.MinPrice, "min-price", 0, "Minimum nightly price")
	cmd.Flags().Float64Var(&opts.MaxPrice, "max-price", 0, "Maximum nightly price")
	cmd.Flags().Float64Var(&opts.MinRating, "min-rating", 0, "Minimum guest rating (0-5)")
	cmd.Flags().StringVar(&hotelClassCSV, "hotel-class", "", "Star ratings to include, e.g. 4,5")
	cmd.Flags().StringSliceVar(&opts.Brand, "brand", nil, "Hotel chains/brands to match (Hyatt, Westin, ...)")
	cmd.Flags().StringSliceVar(&opts.Amenities, "amenities", nil, "Required amenities (pool, breakfast, parking)")
	cmd.Flags().BoolVar(&opts.FreeCancellation, "free-cancellation", false, "Only show free-cancellation rates")
	cmd.Flags().BoolVar(&opts.SpecialOffers, "special-offers", false, "Only show special-offer rates")
	cmd.Flags().BoolVar(&opts.EcoCertified, "eco-certified", false, "Only show eco-certified properties")
	cmd.Flags().StringVar(&opts.Type, "type", "", "Property type: hotel or rental")
	cmd.Flags().IntVar(&opts.MinBedrooms, "min-bedrooms", 0, "Minimum bedrooms (vacation rentals)")
	cmd.Flags().IntVar(&opts.MinBathrooms, "min-bathrooms", 0, "Minimum bathrooms (vacation rentals)")
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "Page number for pagination")
	cmd.Flags().StringVar(&opts.Locale, "locale", "en", "Locale (en, de, fr, ...)")
	cmd.Flags().BoolVar(&noSnapshots, "no-snapshots", false, "Don't append to local price_snapshots history")
	return cmd
}

func parseIntCSV(s string) []int {
	var out []int
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		fmt.Sscanf(p, "%d", &n)
		if n > 0 {
			out = append(out, n)
		}
	}
	return out
}
