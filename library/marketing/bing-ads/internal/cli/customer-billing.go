// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newCustomerBillingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "customer-billing",
		Short:       "Get, search, create, add, and update customer billing",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:api-resource": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newCustomerBillingAddInsertionOrderCmd(flags))
	cmd.AddCommand(newCustomerBillingCheckFeatureAdoptionCouponEligibilityCmd(flags))
	cmd.AddCommand(newCustomerBillingClaimFeatureAdoptionCouponsCmd(flags))
	cmd.AddCommand(newCustomerBillingDispatchCouponsCmd(flags))
	cmd.AddCommand(newCustomerBillingDistributeCouponsCmd(flags))
	cmd.AddCommand(newCustomerBillingGetAccountMonthlySpendCmd(flags))
	cmd.AddCommand(newCustomerBillingGetBillingDocumentsCmd(flags))
	cmd.AddCommand(newCustomerBillingGetBillingDocumentsInfoCmd(flags))
	cmd.AddCommand(newCustomerBillingGetBillingGroupsCmd(flags))
	cmd.AddCommand(newCustomerBillingGetCouponInfoCmd(flags))
	cmd.AddCommand(newCustomerBillingGetUngroupedAccountsCmd(flags))
	cmd.AddCommand(newCustomerBillingRedeemCouponCmd(flags))
	cmd.AddCommand(newCustomerBillingSearchCouponsCmd(flags))
	cmd.AddCommand(newCustomerBillingSearchInsertionOrdersCmd(flags))
	cmd.AddCommand(newCustomerBillingUpdateBillingGroupAccountsCmd(flags))
	cmd.AddCommand(newCustomerBillingUpdateInsertionOrderCmd(flags))
	return cmd
}
