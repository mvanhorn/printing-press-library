// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored clients command group for revenue intelligence.

import "github.com/spf13/cobra"

func newClientsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Client revenue intelligence",
		Long:  "Analyze synced customer revenue, concentration, and AdE risk signals.",
		Example: `  partitaiva24-pp-cli clients top --year 2026 --limit 10
  partitaiva24-pp-cli clients top --json --select customers.customer,customers.total`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newClientsTopCmd(flags))
	return cmd
}
