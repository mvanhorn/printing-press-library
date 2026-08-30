// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: audit command.

package cli

import (
	"github.com/spf13/cobra"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Show audit log for your account",
		Long:    "Show audit log for your account. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli audit",
		Annotations: map[string]string{"pp:endpoint": "audit.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"where": map[string]any{"ownerIds": []int{}}, "orderBy": map[string]any{}, "pagination": map[string]any{}}
			data, err := gqlExec(cmd, flags, "getAuditByOwnerId", variables, "data.audits.nodes")
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
