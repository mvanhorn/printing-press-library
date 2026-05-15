// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import "github.com/spf13/cobra"

func newCampaignCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaign",
		Short: "Campaign utilities backed by the local mirror (trust-history, group-by-wallet).",
	}
	cmd.AddCommand(newCampaignTrustHistoryCmd(flags))
	cmd.AddCommand(newCampaignGroupByWalletCmd(flags))
	return cmd
}
