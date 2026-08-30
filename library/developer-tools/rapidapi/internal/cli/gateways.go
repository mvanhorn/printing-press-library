// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: gateways command.

package cli

import (
	"github.com/spf13/cobra"
)

func newGatewaysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gateways",
		Short:   "List available API gateways",
		Long:    "List available API gateways. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli gateways",
		Annotations: map[string]string{"pp:endpoint": "gateways.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{}
			data, err := gqlExec(cmd, flags, "GetGateways", variables, "data.getGateways")
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
