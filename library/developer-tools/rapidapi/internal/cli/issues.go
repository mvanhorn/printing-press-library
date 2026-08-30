// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: issues command.

package cli

import (
	"github.com/spf13/cobra"
)

func newIssuesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issues",
		Short:   "List issues for an API",
		Long:    "List issues for an API. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli issues --api-id api_x",
		Annotations: map[string]string{"pp:endpoint": "issues.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"apiId": "api_x", "pagingArgs": map[string]any{}}
			data, err := gqlExec(cmd, flags, "getIssuesByApiIdV2", variables, "data.getIssuesByApiIdV2.data")
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
