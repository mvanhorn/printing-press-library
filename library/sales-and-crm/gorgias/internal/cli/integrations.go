// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newIntegrationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "integrations",
		Short:  "Install and configure third-party integrations (Shopify, SMS, social)",
		Hidden: true,
	}

	cmd.AddCommand(newIntegrationsCreateCmd(flags))
	cmd.AddCommand(newIntegrationsDeleteCmd(flags))
	cmd.AddCommand(newIntegrationsGetCmd(flags))
	cmd.AddCommand(newIntegrationsListCmd(flags))
	cmd.AddCommand(newIntegrationsUpdateCmd(flags))
	return cmd
}
