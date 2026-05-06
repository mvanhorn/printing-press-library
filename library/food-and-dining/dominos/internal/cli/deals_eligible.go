// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// dealEligibility is one classified deal. Eligible = auto-fulfills against
// the cart; NotEligible = won't fit (with a reason from the service when
// available). The previous "unknown" bucket is gone now that we use the
// real auto-couponing-service which classifies definitively.
type dealEligibility struct {
	Code        string `json:"code"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type dealsEligibleResult struct {
	Eligible    []dealEligibility `json:"eligible"`
	NotEligible []dealEligibility `json:"not_eligible"`
}

func newDealsEligibleCmd(flags *rootFlags) *cobra.Command {
	var cartName string
	cmd := &cobra.Command{
		Use:   "eligible",
		Short: "Show which coupons apply to the cart and which don't",
		Long: `Show which coupons apply to the cart and which don't.

Builds a Domino's Order from the active cart (or named template) and POSTs
to the auto-couponing-service. Returns:
  - eligible:     coupons that auto-apply to this exact cart
  - not_eligible: coupons that don't fit; reason field populated when the
                  service surfaces one (often "ItemQualifyingConditionNotMet")

Coupon Name and Description are joined from the store menu's Coupons map
for human-readable output.`,
		Example:     "  dominos-pp-cli deals eligible\n  dominos-pp-cli deals eligible --cart friday --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cart, err := loadCartOrTemplate(cartName)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":     "deals_eligible",
					"dry_run":    true,
					"cart_items": len(cart.Items),
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fulfilled, unfulfilled, err := fetchCouponsForCart(c, cart)
			if err != nil {
				return apiErr(fmt.Errorf("auto-couponing failed: %w", err))
			}
			menu, _ := fetchMenuCoupons(c, cart.StoreID)

			result := dealsEligibleResult{
				Eligible:    []dealEligibility{},
				NotEligible: []dealEligibility{},
			}
			for _, raw := range fulfilled {
				code, _ := rawCouponToCode(raw)
				if code == "" {
					continue
				}
				result.Eligible = append(result.Eligible, buildEligibility(code, "", menu))
			}
			for _, raw := range unfulfilled {
				code, reason := rawCouponToCode(raw)
				if code == "" {
					continue
				}
				result.NotEligible = append(result.NotEligible, buildEligibility(code, reason, menu))
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&cartName, "cart", "", "Use a named template instead of the active cart")
	return cmd
}

// buildEligibility joins a coupon code (and optional reason) with menu
// metadata for human-readable output.
func buildEligibility(code, reason string, menu map[string]menuCoupon) dealEligibility {
	d := dealEligibility{Code: code, Reason: reason}
	if menuEntry, ok := menu[code]; ok {
		d.Name = menuEntry.Name
		d.Description = menuEntry.Description
	}
	return d
}
