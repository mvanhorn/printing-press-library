// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: projects command.

package cli

import (
	"github.com/spf13/cobra"
)

func newProjectsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Short:   "List your API projects",
		Long:    "List your API projects. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli projects",
		Annotations: map[string]string{"pp:endpoint": "projects.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{}
			data, err := gqlExec(cmd, flags, "getEntityProjects", variables, "data.projects")
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
