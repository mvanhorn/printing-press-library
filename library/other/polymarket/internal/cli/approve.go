// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: on-chain ERC-20 approval primitives (documented stubs).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newApproveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Set / check the 6 on-chain ERC-20 approvals required to trade on Polymarket.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newApproveSetCmd(flags))
	cmd.AddCommand(newApproveStatusCmd(flags))
	return cmd
}

func newApproveSetCmd(flags *rootFlags) *cobra.Command {
	var broadcast bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the 6 on-chain ERC-20 approvals required to trade on Polymarket (USDC→CTF/NRA/NRX + CT→Exchange/NRA/NRX).",
		Example: `  polymarket-pp-cli approve set --dry-run
  polymarket-pp-cli approve set --broadcast   # honest stub — see broadcast_note`,
		Annotations: map[string]string{"pp:onchain": "approve.set"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := map[string]any{
				"required_approvals": []map[string]string{
					{"asset": "USDC", "spender": "ConditionalTokens", "purpose": "Stake collateral into CTF positions"},
					{"asset": "USDC", "spender": "NegRiskAdapter", "purpose": "Stake into negative-risk multi-outcome markets"},
					{"asset": "USDC", "spender": "NegRiskExchange", "purpose": "Settle negative-risk trades"},
					{"asset": "ConditionalTokens", "spender": "Exchange", "purpose": "Sell YES/NO positions on the CLOB"},
					{"asset": "ConditionalTokens", "spender": "NegRiskAdapter", "purpose": "Split/merge negative-risk positions"},
					{"asset": "ConditionalTokens", "spender": "NegRiskExchange", "purpose": "Trade negative-risk positions"},
				},
				"broadcast_status": "NOT_IMPLEMENTED",
				"broadcast_note":   "Live approval transactions require go-ethereum + Polygon RPC + EIP-155 signing. Run the official Polymarket Rust CLI's `polymarket approve set` once per wallet (sends ~6 Polygon tx, costs ~$0.02 MATIC). Alternatively wire go-ethereum here in v0.2. To check current approval state without sending tx, run: polymarket-pp-cli balance get --asset-type COLLATERAL",
				"broadcast":        broadcast,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Send approval transactions on-chain (not implemented — see broadcast_note)")
	return cmd
}

func newApproveStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Inspect current ERC-20 allowances for the configured wallet.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:onchain": "approve.status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := map[string]any{
				"status":         "NOT_IMPLEMENTED",
				"note":           fmt.Sprintf("Reading on-chain allowances requires a Polygon RPC client. Use the Polymarket spec endpoint as a proxy: `polymarket-pp-cli balance get --asset-type COLLATERAL` (and `--asset-type CONDITIONAL`) — the balance endpoint reflects whether allowance is set."),
				"workaround_cmd": "polymarket-pp-cli balance get --asset-type COLLATERAL",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
