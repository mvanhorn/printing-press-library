package cli

import (
	"github.com/spf13/cobra"
)

// newDealsCmd is the parent for transcendence-layer deal commands.
// (The endpoint-mirror DealsList lives under `graphql deals-list`.)
func newDealsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Optimize deals against the active cart",
		Long: `Browse and optimize Domino's coupons.

  list       List all coupons available at a store (no cart required).
  best       Find coupons that auto-apply to the active cart.
  eligible   Show which coupons apply and which don't (with reasons).

'list' fetches the public store menu (no auth) and dumps the embedded
Coupons section. 'best' and 'eligible' POST the active cart to the
auto-couponing-service to get cart-specific eligibility.`,
	}
	cmd.AddCommand(newDealsListCmd(flags))
	cmd.AddCommand(newDealsBestCmd(flags))
	cmd.AddCommand(newDealsEligibleCmd(flags))
	return cmd
}
