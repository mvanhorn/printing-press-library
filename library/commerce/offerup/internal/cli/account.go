// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
func newAccountCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "account",
		Short:       "Show your own OfferUp profile and reputation (requires login)",
		Example:     "  offerup-pp-cli account --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthRead(cmd, flags, map[string]any{}, func() (any, error) {
				return newOfferupClient(flags).Account(cmd.Context())
			})
		},
	}
}
