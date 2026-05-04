// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type marketStats struct {
	CityState         string  `json:"city_state"`
	Count             int     `json:"count"`
	MedianRent        float64 `json:"median_rent"`
	P10Rent           float64 `json:"p10_rent"`
	P90Rent           float64 `json:"p90_rent"`
	MedianRentPerSqft float64 `json:"median_rent_per_sqft"`
	PetFriendlyShare  float64 `json:"pet_friendly_share"`
	Beds              *int    `json:"beds,omitempty"`
}

func newMarketCmd(flags *rootFlags) *cobra.Command {
	var beds int

	cmd := &cobra.Command{
		Use:         "market <city-state>",
		Short:       "Median, p10/p90 rent, rent/sqft, pet-friendly share for one city/state.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli market austin-tx --json
  apartments-pp-cli market new-york-ny --beds 2
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.ToLower(strings.TrimSpace(args[0]))
			city, state := splitCityState(slug)

			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := loadCachedListings(db.DB())
			if err != nil {
				return err
			}

			stats := marketStats{CityState: slug}
			if cmd.Flags().Changed("beds") {
				b := beds
				stats.Beds = &b
			}

			var rents, ratios []float64
			petFriendly := 0
			matched := 0
			for _, r := range rows {
				li := r.Data
				// Address fields are populated only by successful listing
				// detail fetches; placards (sync-search) populate URL but
				// not address. Fall back to parsing city-state from the
				// URL slug — listing URLs always end with -{city}-{state}.
				liCity := strings.ToLower(strings.ReplaceAll(li.Address.City, " ", "-"))
				liState := strings.ToLower(li.Address.State)
				if liCity == "" || liState == "" {
					urlCity, urlState := cityStateFromListingURL(li.URL)
					if liCity == "" {
						liCity = urlCity
					}
					if liState == "" {
						liState = urlState
					}
				}
				if city != "" && !strings.HasPrefix(liCity, city) {
					continue
				}
				if state != "" && !strings.EqualFold(liState, state) {
					continue
				}
				if cmd.Flags().Changed("beds") && li.Beds != beds {
					continue
				}
				matched++
				if li.MaxRent > 0 {
					rents = append(rents, float64(li.MaxRent))
					if li.Sqft > 0 {
						ratios = append(ratios, float64(li.MaxRent)/float64(li.Sqft))
					}
				}
				if li.PetPolicy.AllowsCats || li.PetPolicy.AllowsDogs {
					petFriendly++
				}
			}
			stats.Count = matched
			stats.MedianRent = median(rents)
			stats.P10Rent = percentile(rents, 10)
			stats.P90Rent = percentile(rents, 90)
			stats.MedianRentPerSqft = median(ratios)
			if matched > 0 {
				stats.PetFriendlyShare = float64(petFriendly) / float64(matched)
			}
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().IntVar(&beds, "beds", 0, "Filter by exact bed count.")
	return cmd
}
