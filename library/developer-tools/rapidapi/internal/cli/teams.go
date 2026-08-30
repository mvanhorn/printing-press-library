// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: teams command.

package cli

import (
	"github.com/spf13/cobra"
)

func newTeamsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "teams",
		Short:   "List teams in an organization",
		Long:    "List teams in an organization. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli teams",
		Annotations: map[string]string{"pp:endpoint": "teams.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"orgId": 0}
			data, err := gqlExec(cmd, flags, "teams", variables, "data.teams")
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
