// Hand-authored: networth top-level command. Hosts the `explain` novel
// feature plus a `history` shortcut that wraps accounts.networth_snapshots.

package cli

import "github.com/spf13/cobra"

func newNetworthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Net-worth analytics: history, snapshots, and delta attribution",
	}
	cmd.AddCommand(newNetworthExplainCmd(flags))
	cmd.AddCommand(newNetworthHistoryCmd(flags))
	return cmd
}
