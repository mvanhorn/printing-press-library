// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newCustomerManagementCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "customer-management",
		Short:       "Get, search, find, create, add, update, delete, and send customer management",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:api-resource": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newCustomerManagementAddAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementAddClientLinksCmd(flags))
	cmd.AddCommand(newCustomerManagementAddPrepayAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementDeleteAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementDeleteCustomerCmd(flags))
	cmd.AddCommand(newCustomerManagementDeleteUserCmd(flags))
	cmd.AddCommand(newCustomerManagementDismissNotificationsCmd(flags))
	cmd.AddCommand(newCustomerManagementFindAccountsCmd(flags))
	cmd.AddCommand(newCustomerManagementFindAccountsOrCustomersInfoCmd(flags))
	cmd.AddCommand(newCustomerManagementGetAccessibleCustomerCmd(flags))
	cmd.AddCommand(newCustomerManagementGetAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementGetAccountPilotFeaturesCmd(flags))
	cmd.AddCommand(newCustomerManagementGetAccountsInfoCmd(flags))
	cmd.AddCommand(newCustomerManagementGetCurrentUserCmd(flags))
	cmd.AddCommand(newCustomerManagementGetCustomerCmd(flags))
	cmd.AddCommand(newCustomerManagementGetCustomerPilotFeaturesCmd(flags))
	cmd.AddCommand(newCustomerManagementGetCustomersInfoCmd(flags))
	cmd.AddCommand(newCustomerManagementGetLinkedAccountsAndCustomersInfoCmd(flags))
	cmd.AddCommand(newCustomerManagementGetNotificationsCmd(flags))
	cmd.AddCommand(newCustomerManagementGetPilotFeaturesCountriesCmd(flags))
	cmd.AddCommand(newCustomerManagementGetUserCmd(flags))
	cmd.AddCommand(newCustomerManagementGetUserMfaStatusCmd(flags))
	cmd.AddCommand(newCustomerManagementGetUsersInfoCmd(flags))
	cmd.AddCommand(newCustomerManagementMapAccountIdToExternalAccountIdsCmd(flags))
	cmd.AddCommand(newCustomerManagementMapCustomerIdToExternalCustomerIdCmd(flags))
	cmd.AddCommand(newCustomerManagementSearchAccountsCmd(flags))
	cmd.AddCommand(newCustomerManagementSearchClientLinksCmd(flags))
	cmd.AddCommand(newCustomerManagementSearchCustomersCmd(flags))
	cmd.AddCommand(newCustomerManagementSearchUserInvitationsCmd(flags))
	cmd.AddCommand(newCustomerManagementSendUserInvitationCmd(flags))
	cmd.AddCommand(newCustomerManagementSignupCustomerCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdateAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdateClientLinksCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdateCustomerCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdatePrepayAccountCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdateUserCmd(flags))
	cmd.AddCommand(newCustomerManagementUpdateUserRolesCmd(flags))
	cmd.AddCommand(newCustomerManagementUpgradeCustomerToAgencyCmd(flags))
	cmd.AddCommand(newCustomerManagementValidateAddressCmd(flags))
	return cmd
}
