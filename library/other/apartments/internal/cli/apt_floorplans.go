// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type floorPlanRow struct {
	ListingURL   string  `json:"listing_url"`
	PlanName     string  `json:"plan_name,omitempty"`
	Beds         int     `json:"beds,omitempty"`
	Baths        float64 `json:"baths,omitempty"`
	Sqft         int     `json:"sqft,omitempty"`
	RentMin      int     `json:"rent_min,omitempty"`
	RentMax      int     `json:"rent_max,omitempty"`
	PricePerSqft float64 `json:"price_per_sqft,omitempty"`
}

func newFloorplansCmd(flags *rootFlags) *cobra.Command {
	var rank string
	var beds int
	var limit int

	cmd := &cobra.Command{
		Use:         "floorplans",
		Short:       "Rank per-floor-plan rent/sqft across synced listings.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli floorplans --rank price-per-sqft --beds 2 --json
  apartments-pp-cli floorplans --limit 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch rank {
			case "", "price-per-sqft":
			default:
				return usageErr(fmt.Errorf("invalid --rank %q: must be price-per-sqft", rank))
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := loadCachedListings(db.DB())
			if err != nil {
				return err
			}
			var out []floorPlanRow
			for _, r := range rows {
				li := r.Data
				for _, fp := range li.FloorPlans {
					if cmd.Flags().Changed("beds") && fp.Beds != beds {
						continue
					}
					row := floorPlanRow{
						ListingURL: li.URL,
						PlanName:   fp.Name,
						Beds:       fp.Beds,
						Baths:      fp.Baths,
						Sqft:       fp.Sqft,
						RentMin:    fp.RentMin,
						RentMax:    fp.RentMax,
					}
					if fp.RentMin > 0 && fp.Sqft > 0 {
						row.PricePerSqft = float64(fp.RentMin) / float64(fp.Sqft)
					}
					out = append(out, row)
				}
			}
			sort.SliceStable(out, func(i, j int) bool {
				ai := out[i].PricePerSqft
				bj := out[j].PricePerSqft
				if ai == 0 {
					ai = 1e18
				}
				if bj == 0 {
					bj = 1e18
				}
				return ai < bj
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if out == nil {
				out = []floorPlanRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&rank, "rank", "price-per-sqft", "Ranker (currently: price-per-sqft).")
	cmd.Flags().IntVar(&beds, "beds", 0, "Filter to plans with exact bed count.")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return.")
	return cmd
}
