// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic collections list command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newCollectionsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var page int

	cmd := &cobra.Command{
		Use:         "list [flags]",
		Short:       "List curated collections (Recommended, Popular, Free, AI-based)",
		Example:     "  rapidapi-pp-cli collections list --limit 10",
		Annotations: map[string]string{"pp:endpoint": "collections.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			variables := map[string]any{"page": page, "limit": limit}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "GetCollectionsCollapsed", variables, gqlResponsePaths["GetCollectionsCollapsed"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "title": true, "slugifiedKey": true, "weight": true, "thumbnail": true, "shortDescription": true})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of collections")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
