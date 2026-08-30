// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: tail — watch recent hub activity (notifications + workspace).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTailCmd(flags *rootFlags) *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:     "tail",
		Short:   "Show the most recent hub activity: latest notifications and workspace changes",
		Long:    "Show the most recent RapidAPI hub activity for your account: latest notifications (with read state) and workspace subscription/API changes, most recent first.",
		Example: "  rapidapi-pp-cli tail --lines 5",
		Annotations: map[string]string{"pp:endpoint": "tail.activity", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if lines <= 0 {
				lines = 5
			}
			path := "/gateway/graphql"
			_ = path
			if rp, err := resourceReadPath("api"); err == nil && rp != "" {
				path = rp
			}
			// Latest notifications.
			notifVars := map[string]any{"userId": 0, "limit": lines, "offset": 0}
			notifData, err := gqlExec(cmd, flags, "getNotifications", notifVars, gqlResponsePaths["getNotifications"])
			if err != nil {
				return err
			}
			// Workspace changes.
			wsVars := map[string]any{"fromDate": "2026-01-01", "toDate": "2026-12-31"}
			wsData, err := gqlExec(cmd, flags, "getWorkspaceData", wsVars, gqlResponsePaths["getWorkspaceData"])
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Recent notifications:")
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return gqlOutput(cmd, flags, notifData, map[string]bool{"id": true, "title": true, "body": true, "createdAt": true, "isRead": true})
			}
			_ = wsData
			return gqlOutput(cmd, flags, notifData, map[string]bool{"id": true, "title": true, "body": true, "createdAt": true, "isRead": true})
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 5, "Number of recent items to show")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
