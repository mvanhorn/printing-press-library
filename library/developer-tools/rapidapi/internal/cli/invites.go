// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: invites command.

package cli

import (
	"github.com/spf13/cobra"
)

func newInvitesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invites",
		Short:   "Show organization invite by token",
		Long:    "Show organization invite by token. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli invites --token abc",
		Annotations: map[string]string{"pp:endpoint": "invites.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"token": "abc"}
			data, err := gqlExec(cmd, flags, "getUserInviteByToken", variables, "data.getUserInviteByToken")
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true})
		},
	}
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")
	return cmd
}
