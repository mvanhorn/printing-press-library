// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ratings command.

package cli

import (
	"github.com/spf13/cobra"
)

func newRatingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ratings",
		Short:   "Show API rating",
		Long:    "Show API rating. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli ratings --owner meteostat --api meteostat",
		Annotations: map[string]string{"pp:endpoint": "ratings.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"apiOwnerSlug": "meteostat", "apiSlug": "meteostat", "withEndpoints": false}
			data, err := gqlExec(cmd, flags, "getApiBySlugAndOwner", variables, "data.apiBySlugifiedNameAndOwnerName.rating")
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
