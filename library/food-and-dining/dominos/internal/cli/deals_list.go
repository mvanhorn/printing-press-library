// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// dealsListEntry is the per-coupon row returned to the caller. Trimmed to
// the fields users actually want; the full menu.Coupons[code] entry has
// more fields (PriceInfo, Tags, Bundle) available via --json --select.
type dealsListEntry struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
	Local       bool   `json:"local"`
}

type dealsListResult struct {
	Coupons     []dealsListEntry `json:"coupons"`
	StoreID     string           `json:"store_id"`
	Total       int              `json:"total"`
	LocalOnly   bool             `json:"local_only,omitempty"`
}

func newDealsListCmd(flags *rootFlags) *cobra.Command {
	var storeID string
	var localOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all coupons available at a store (no cart required)",
		Long: `List all coupons available at a store.

Fetches the store menu (no auth required) and extracts the Coupons section,
which holds 50+ coupons keyed by code. Each entry has Code, Name,
Description, Price, and a Local flag (Local=true means the coupon is store-
specific; Local=false means it's a national promotion).

For cart-aware coupon application, see 'deals eligible' (which only
returns coupons that auto-apply to your active cart).`,
		Example: "  dominos-pp-cli deals list --store-id 7144\n  dominos-pp-cli deals list --store-id 7144 --local-only --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if storeID == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--store-id is required"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), dealsListResult{StoreID: storeID, LocalOnly: localOnly}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			menu, err := fetchMenuCoupons(c, storeID)
			if err != nil {
				return apiErr(fmt.Errorf("fetching menu coupons: %w", err))
			}
			result := dealsListResult{StoreID: storeID, LocalOnly: localOnly}
			for code, mc := range menu {
				if localOnly && !mc.Local {
					continue
				}
				result.Coupons = append(result.Coupons, dealsListEntry{
					Code:        code,
					Name:        mc.Name,
					Description: mc.Description,
					Price:       mc.Price,
					Local:       mc.Local,
				})
			}
			// Stable order: by Code lexically (so output is deterministic for
			// dogfood / acceptance tests).
			sort.Slice(result.Coupons, func(i, j int) bool {
				return result.Coupons[i].Code < result.Coupons[j].Code
			})
			result.Total = len(result.Coupons)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&storeID, "store-id", "", "Store ID (required; find via 'stores find-stores')")
	cmd.Flags().BoolVar(&localOnly, "local-only", false, "Filter to local (store-specific) coupons only")
	return cmd
}
