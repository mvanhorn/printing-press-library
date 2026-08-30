// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic metrics command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMetricsPromotedCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:     "metrics",
		Short:   "Show public marketplace metrics: total APIs, users, consumers, API traffic",
		Long:    "Show RapidAPI hub public metrics: total public APIs, registered users, active API consumers, and total API traffic, with current/previous period deltas.",
		Example: "  rapidapi-pp-cli metrics --from 2026-08-01 --to 2026-08-28",
		Annotations: map[string]string{"pp:endpoint": "metrics.show", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "--from=2026-08-01;--to=2026-08-28"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			if from == "" {
				from = "2026-01-01"
			}
			if to == "" {
				to = "2026-12-31"
			}
			variables := map[string]any{"where": map[string]any{"fromDate": from, "toDate": to}}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getHubMetrics", variables, gqlResponsePaths["getHubMetrics"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"publicApis": true, "users": true, "activeApiConsumers": true, "totalApiTraffic": true})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD, default 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD, default 2026-12-31)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
