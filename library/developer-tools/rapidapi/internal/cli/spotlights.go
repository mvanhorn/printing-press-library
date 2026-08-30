// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: spotlights command.

package cli

import (
	"github.com/spf13/cobra"
)

func newSpotlightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "spotlights",
		Short:   "Show API spotlights",
		Long:    "Show API spotlights. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli spotlights --owner meteostat --api meteostat",
		Annotations: map[string]string{"pp:endpoint": "spotlights.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"apiOwnerSlug": "meteostat", "apiSlug": "meteostat", "withEndpoints": false}
			data, err := gqlExec(cmd, flags, "getApiBySlugAndOwner", variables, "data.apiBySlugifiedNameAndOwnerName.spotlights")
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
