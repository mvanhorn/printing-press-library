// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Bambu history command family.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "history",
		Short:       "history subcommands: failure-correlations",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE:        newBambuHistoryRunE(flags),
	}
	cmd.Flags().String("since", "30d", "History window such as 24h, 7d, or 30d")
	cmd.Flags().Int("limit", 100, "Maximum persisted jobs to return (1-1000)")
	cmd.AddCommand(newNovelHistoryFailureCorrelationsCmd(flags))
	return cmd
}
