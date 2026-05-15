// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import "github.com/spf13/cobra"

func newAuctionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auction",
		Short: "Auction inspection backed by the local placement log (explain).",
	}
	cmd.AddCommand(newAuctionExplainCmd(flags))
	return cmd
}
