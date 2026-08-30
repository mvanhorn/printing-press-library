// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: usages command.

package cli

import (
	"github.com/spf13/cobra"
)

func newUsagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "usages",
		Short:   "Show usage for a subscription",
		Long:    "Show usage for a subscription. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli usages --api-id api_x --subscription-id 1",
		Annotations: map[string]string{"pp:endpoint": "usages.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"apiId": "api_x", "subscriptionId": "1", "resolution": "DAILY", "periods": []string{}}
			data, err := gqlExec(cmd, flags, "getUsagesForSubscription", variables, "data.usages")
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
