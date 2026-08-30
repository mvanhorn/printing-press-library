// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelCollectionCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagCollection string
	var flagQuery string
	var flagLimit string

	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Compare live Cosmos search results with live collection membership and return promising references that are not already saved.",
		Example:     "  cosmos-pp-cli collection coverage --collection 101 --query 'brutalist typography' --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "analyze Cosmos collection coverage")
			}
			return runCosmosCollectionCoverage(cmd, flags, flagCollection, flagQuery, flagLimit)
		},
	}
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Collection ID to compare")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Discovery query")
	cmd.Flags().StringVar(&flagLimit, "limit", "20", "Maximum search results (1-40)")
	return cmd
}
