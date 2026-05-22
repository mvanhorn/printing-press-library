// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newMeCmd is the parent for self-account novel features that don't fit cleanly
// into the spec-derived `account` resource: FTS5 over own history, per-sub
// timing stats, etc.
func newMeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Personal Reddit workflows: search own history, posting stats",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMeSearchCmd(flags))
	cmd.AddCommand(newMePostingStatsCmd(flags))
	return cmd
}
