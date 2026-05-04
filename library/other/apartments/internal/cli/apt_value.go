// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// valueEntry is one ranked listing in --budget-aware total-cost order.
type valueEntry struct {
	Rank       int    `json:"rank"`
	URL        string `json:"url"`
	PropertyID string `json:"property_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Beds       int    `json:"beds,omitempty"`
	MaxRent    int    `json:"max_rent,omitempty"`
	TotalCost  int    `json:"total_cost"`
	PetRent    int    `json:"pet_rent,omitempty"`
	PetDeposit int    `json:"pet_deposit,omitempty"`
	PetFee     int    `json:"pet_fee,omitempty"`
}

func newValueCmd(flags *rootFlags) *cobra.Command {
	var budget int
	var pet string
	var months int

	cmd := &cobra.Command{
		Use:         "value",
		Short:       "Rank synced listings by 12-month total cost (rent + pet rent + pet deposit + pet fee).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli value --budget 2500 --pet dog --months 12 --json
  apartments-pp-cli value --budget 1800 --months 18
  apartments-pp-cli value --pet none
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch pet {
			case "", "none", "cat", "dog", "both":
			default:
				return usageErr(fmt.Errorf("invalid --pet %q: must be none|cat|dog|both", pet))
			}
			if months <= 0 {
				months = 12
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
			hasPet := pet == "cat" || pet == "dog" || pet == "both"

			var out []valueEntry
			for _, r := range rows {
				li := r.Data
				if li.MaxRent <= 0 {
					continue
				}
				total := li.MaxRent * months
				if hasPet {
					total += li.PetPolicy.PetRent * months
					total += li.PetPolicy.PetDeposit
					total += li.PetPolicy.PetFee
				}
				if budget > 0 && total > budget*months {
					continue
				}
				out = append(out, valueEntry{
					URL:        li.URL,
					PropertyID: li.PropertyID,
					Title:      li.Title,
					Beds:       li.Beds,
					MaxRent:    li.MaxRent,
					TotalCost:  total,
					PetRent:    li.PetPolicy.PetRent,
					PetDeposit: li.PetPolicy.PetDeposit,
					PetFee:     li.PetPolicy.PetFee,
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].TotalCost < out[j].TotalCost
			})
			for i := range out {
				out[i].Rank = i + 1
			}
			if out == nil {
				out = []valueEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&budget, "budget", 0, "Hard monthly budget in USD; total cost / months must not exceed this.")
	cmd.Flags().StringVar(&pet, "pet", "none", "Pet status: none|cat|dog|both.")
	cmd.Flags().IntVar(&months, "months", 12, "Lease length in months.")
	return cmd
}
