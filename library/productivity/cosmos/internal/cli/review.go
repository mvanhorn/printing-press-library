// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import "github.com/spf13/cobra"

func newNovelReviewCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "review",
		Short:       "Review recent saves for missing sources, duplicate media, AI flags, and unfiled elements",
		Example:     "  cosmos-pp-cli review --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "build Cosmos review queue")
			}
			return runCosmosReview(cmd, flags, flagSince)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Relative duration or RFC3339 timestamp")
	return cmd
}
