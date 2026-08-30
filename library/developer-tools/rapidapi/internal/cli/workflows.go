// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: workflows command.

package cli

import (
	"github.com/spf13/cobra"
)

func newWorkflowsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workflows",
		Short:   "List approval workflows",
		Long:    "List approval workflows. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli workflows",
		Annotations: map[string]string{"pp:endpoint": "workflows.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"options": map[string]any{}, "pagingArgs": map[string]any{}}
			data, err := gqlExec(cmd, flags, "getWorkflowsByRequestee", variables, "data.getWorkflowsByRequestee.data")
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
