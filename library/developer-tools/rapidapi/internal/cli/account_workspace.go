// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic account workspace command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountWorkspaceCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:         "workspace",
		Short:       "Show your workspace: owned APIs, subscribed APIs, metrics",
		Long:        "Show the logged-in user's workspace: owned APIs with metrics (error rate, latency, subscribers, requests, sales) and subscribed APIs with request totals and subscription statuses.",
		Example:     "  rapidapi-pp-cli account workspace --from 2026-08-01 --to 2026-08-28",
		Annotations: map[string]string{"pp:endpoint": "account.workspace", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "--from=2026-08-01;--to=2026-08-28"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				from = "2026-01-01"
			}
			if to == "" {
				to = "2026-12-31"
			}
			variables := map[string]any{"fromDate": from, "toDate": to}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getWorkspaceData", variables, gqlResponsePaths["getWorkspaceData"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"ownedApis": true, "subscribedApis": true, "invitedApis": true})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD, default 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD, default 2026-12-31)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
