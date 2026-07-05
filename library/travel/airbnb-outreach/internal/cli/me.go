// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "me",
		Short:   "Show the signed-in account (name, id, host status)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			acct, err := c.Me()
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, acct)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s (id %s)\n", bold("Account:"), acct.FirstName, acct.ID)
			role := "guest"
			if acct.IsHomeHost {
				role = "host"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Role:    %s\n", role)
			return nil
		},
	}
}
