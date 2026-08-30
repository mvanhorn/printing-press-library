// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic account notifications command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountNotificationsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:         "notifications",
		Short:       "List your notifications",
		Long:        "List the logged-in user's notifications: type, title, body, read state, and created time.",
		Example:     "  rapidapi-pp-cli account notifications --limit 10",
		Annotations: map[string]string{"pp:endpoint": "account.notifications", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			variables := map[string]any{"userId": 0, "limit": limit, "offset": offset}
			// userId=0 lets the gateway resolve the caller's own notifications.
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getNotifications", variables, gqlResponsePaths["getNotifications"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "type": true, "createdAt": true, "isRead": true, "title": true, "body": true, "callToAction": true, "image": true})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of notifications")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
