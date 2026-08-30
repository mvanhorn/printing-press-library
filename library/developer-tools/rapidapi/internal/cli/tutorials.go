// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: tutorials command.

package cli

import (
	"github.com/spf13/cobra"
)

func newTutorialsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tutorials",
		Short:   "List tutorials for an API",
		Long:    "List tutorials for an API. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli tutorials",
		Annotations: map[string]string{"pp:endpoint": "tutorials.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"id": "api_x", "versionId": "v1"}
			data, err := gqlExec(cmd, flags, "tutorials", variables, "data.tutorials.nodes")
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
