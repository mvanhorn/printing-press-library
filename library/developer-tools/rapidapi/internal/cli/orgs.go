// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: orgs command.

package cli

import (
	"github.com/spf13/cobra"
)

func newOrgsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orgs",
		Short:   "List organizations you belong to",
		Long:    "List organizations you belong to. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli orgs",
		Annotations: map[string]string{"pp:endpoint": "orgs.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{}
			data, err := gqlExec(cmd, flags, "organizations", variables, "data.organizations")
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
