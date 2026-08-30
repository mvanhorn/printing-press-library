// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: announcements command.

package cli

import (
	"github.com/spf13/cobra"
)

func newAnnouncementsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "announcements",
		Short:   "List API announcements",
		Long:    "List API announcements. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli announcements",
		Annotations: map[string]string{"pp:endpoint": "announcements.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"where": map[string]any{}, "pagingArgs": map[string]any{}}
			data, err := gqlExec(cmd, flags, "announcements", variables, "data.announcements")
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
