// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: billing command.

package cli

import (
	"github.com/spf13/cobra"
)

func newBillingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "billing",
		Short:   "Show billing plans for an API",
		Long:    "Show billing plans for an API. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli billing --api-id api_x",
		Annotations: map[string]string{"pp:endpoint": "billing.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"apiId": "api_x", "entityId": "0"}
			data, err := gqlExec(cmd, flags, "apiBillingPlans", variables, "data.api")
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
