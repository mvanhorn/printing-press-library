// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: alerts command.

package cli

import (
	"github.com/spf13/cobra"
)

func newAlertsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "alerts",
		Short:   "Show notification alerts",
		Long:    "Show notification alerts. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli alerts --limit 5",
		Annotations: map[string]string{"pp:endpoint": "alerts.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"userId": 0, "limit": 5, "offset": 0}
			data, err := gqlExec(cmd, flags, "getNotifications", variables, "data.newNotificationsByUserId")
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true})
		},
	}
	cmd.Flags().Int("limit", 10, "Max alerts to show")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")
	return cmd
}
