// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
func newSavedCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "saved",
		Short:       "List your saved/favorited OfferUp lists (requires login)",
		Example:     "  offerup-pp-cli saved --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthRead(cmd, flags, []any{}, func() (any, error) {
				return newOfferupClient(flags).SavedLists(cmd.Context())
			})
		},
	}
}
