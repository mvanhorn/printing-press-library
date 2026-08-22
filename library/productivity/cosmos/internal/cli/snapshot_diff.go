// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Compare historical collection membership and show added, removed, and moved elements.",
		Example:     "  cosmos-pp-cli snapshot diff --from 7d --to now --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare Cosmos snapshots")
			}
			return runCosmosSnapshotDiff(cmd, flags, flagFrom, flagTo)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "7d", "Earlier snapshot time: 7d, RFC3339, or now")
	cmd.Flags().StringVar(&flagTo, "to", "now", "Later snapshot time: duration, RFC3339, or now")
	return cmd
}
