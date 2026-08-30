// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: followers command.

package cli

import (
	"github.com/spf13/cobra"
)

func newFollowersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "followers",
		Short:   "Show APIs you follow",
		Long:    "Show APIs you follow. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli followers",
		Annotations: map[string]string{"pp:endpoint": "followers.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{}
			data, err := gqlExec(cmd, flags, "getUserSavedApis", variables, "data.userSavedApis")
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
