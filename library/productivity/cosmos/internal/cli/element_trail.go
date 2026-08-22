// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelElementTrailCmd(flags *rootFlags) *cobra.Command {
	var flagId string
	var flagDepth string
	var flagLimit string

	cmd := &cobra.Command{
		Use:         "trail",
		Short:       "Walk visual similarity results to a bounded depth and emit a deduplicated, source-aware graph.",
		Example:     "  cosmos-pp-cli element trail --id 2113061259 --depth 2 --limit 12 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "walk Cosmos similarity trail")
			}
			return runCosmosElementTrail(cmd, flags, flagId, flagDepth, flagLimit)
		},
	}
	cmd.Flags().StringVar(&flagId, "id", "", "Starting element ID")
	cmd.Flags().StringVar(&flagDepth, "depth", "2", "Maximum traversal depth (1-3)")
	cmd.Flags().StringVar(&flagLimit, "limit", "12", "Neighbors per element (1-40)")
	return cmd
}
