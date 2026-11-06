// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

const mistakesRetired = "cookie-based find-mistakes is retired; use clips find-retakes for reviewed deterministic repeats, or agent-chats start with explicit --yes for semantic review"

func newVideosClipsFindMistakesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "find-mistakes <video> <clip>", Short: "Retired: use clips find-retakes or agent-chats start", Args: workflowArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return usageErr(fmt.Errorf("%s", mistakesRetired))
	}}
	cmd.Flags().Bool("unofficial", false, "Retired compatibility flag")
	cmd.Flags().Bool("detect-only", false, "Retired compatibility flag")
	_ = cmd.Flags().MarkDeprecated("unofficial", mistakesRetired)
	_ = cmd.Flags().MarkDeprecated("detect-only", mistakesRetired)
	return cmd
}
