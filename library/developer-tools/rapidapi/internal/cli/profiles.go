// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: profiles command.

package cli

import (
	"github.com/spf13/cobra"
)

func newProfilesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profiles",
		Short:   "Show entity profile",
		Long:    "Show entity profile. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli profiles",
		Annotations: map[string]string{"pp:endpoint": "profiles.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"where": map[string]any{}, "publishedApisWhere": map[string]any{}}
			data, err := gqlExec(cmd, flags, "getEntityProfile", variables, "data.entity")
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
