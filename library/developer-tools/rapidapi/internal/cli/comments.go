// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: comments command.

package cli

import (
	"github.com/spf13/cobra"
)

func newCommentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "comments",
		Short:   "List message threads",
		Long:    "List message threads. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli comments",
		Annotations: map[string]string{"pp:endpoint": "comments.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"where": map[string]any{}, "pagination": map[string]any{}}
			data, err := gqlExec(cmd, flags, "messageThreadsPaginated", variables, "data.messageThreadsPaginated.nodes")
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
