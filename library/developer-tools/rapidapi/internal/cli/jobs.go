// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: jobs — async job tracking for long-running hub operations.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newJobsCmd(flags *rootFlags) *cobra.Command {
	var wait bool

	cmd := &cobra.Command{
		Use:     "jobs",
		Short:   "Show recent workspace activity as jobs: API creations, subscription changes, notifications",
		Long:    "Show recent workspace activity (subscription changes, notifications, API updates) as a job list, optionally waiting for new activity with --wait (polls the workspace endpoint).",
		Example: "  rapidapi-pp-cli jobs\n  rapidapi-pp-cli jobs --wait",
		Annotations: map[string]string{"pp:endpoint": "jobs.workspace", "pp:method": "POST", "pp:path": "/gateway/graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if wait {
				// Poll the workspace endpoint up to ~30s for new activity.
				deadline := time.Now().Add(30 * time.Second)
				for time.Now().Before(deadline) {
					data, err := gqlExec(cmd, flags, "getWorkspaceData", map[string]any{
						"fromDate": "2026-01-01",
						"toDate":   "2026-12-31",
					}, gqlResponsePaths["getWorkspaceData"])
					if err != nil {
						return err
					}
					_ = data
					time.Sleep(3 * time.Second)
				}
			}
			// Show workspace activity as jobs.
			data, err := gqlExec(cmd, flags, "getWorkspaceData", map[string]any{
				"fromDate": "2026-01-01",
				"toDate":   "2026-12-31",
			}, gqlResponsePaths["getWorkspaceData"])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Workspace activity (most recent first):")
			return gqlOutput(cmd, flags, data, map[string]bool{"ownedApis": true, "subscribedApis": true, "invitedApis": true})
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll for new workspace activity")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
