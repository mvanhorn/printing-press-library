// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newCallsCmd registers the top-level `calls` parent and its children:
// grep (FTS over transcripts) and worst (lowest-scored calls join).
func newCallsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calls",
		Short: "Search and rank synced call records",
	}
	cmd.AddCommand(newCallsGrepCmd(flags))
	cmd.AddCommand(newCallsWorstCmd(flags))
	return cmd
}
