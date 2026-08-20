package cli

import (
	"github.com/spf13/cobra"
)

func newTrademarkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trademark",
		Short: "Trademark-specific intelligence commands",
		Long: `High-value compound commands that combine multiple TSDR API calls
or add computed logic on top of raw data:

  status    — Full current state of a trademark in one command
  timeline  — Prosecution timeline in chronological order
  deadlines — Section 8, 9, and 15 maintenance deadlines
  watch     — Monitor multiple trademarks for status changes
  batch     — Batch status check for multiple trademarks
  docs      — List all documents in the prosecution file`,
	}

	cmd.AddCommand(newTrademarkStatusCmd(flags))
	cmd.AddCommand(newTrademarkTimelineCmd(flags))
	cmd.AddCommand(newTrademarkDeadlinesCmd(flags))
	cmd.AddCommand(newTrademarkWatchCmd(flags))
	cmd.AddCommand(newTrademarkBatchCmd(flags))
	cmd.AddCommand(newTrademarkDocsCmd(flags))

	return cmd
}
