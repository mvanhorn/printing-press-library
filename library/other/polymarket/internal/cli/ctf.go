// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: ConditionalTokens (CTF) on-chain primitives (documented stubs).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCtfCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ctf",
		Short: "ConditionalTokens (CTF) on-chain primitives: split, merge, redeem.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCtfSplitCmd(flags))
	cmd.AddCommand(newCtfMergeCmd(flags))
	cmd.AddCommand(newCtfRedeemCmd(flags))
	return cmd
}

func newCtfSplitCmd(flags *rootFlags) *cobra.Command {
	var market string
	var amount float64
	cmd := &cobra.Command{
		Use:         "split <market>",
		Short:       "Split USDC collateral into YES + NO outcome tokens for a market (documented stub).",
		Example:     `  polymarket-pp-cli ctf split 0xCONDITION_ID --amount 100`,
		Annotations: map[string]string{"pp:onchain": "ctf.split"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				market = args[0]
			}
			if (market == "" || amount == 0) && !flags.dryRun {
				return usageErr(fmt.Errorf("required: <market> arg + --amount FLAG"))
			}
			if dryRunOK(flags) {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), ctfStubPayload("split", market, amount), flags)
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount of USDC to split (required)")
	return cmd
}

func newCtfMergeCmd(flags *rootFlags) *cobra.Command {
	var market string
	var amount float64
	cmd := &cobra.Command{
		Use:         "merge <market>",
		Short:       "Merge equal YES + NO outcome tokens back into USDC collateral (documented stub).",
		Example:     `  polymarket-pp-cli ctf merge 0xCONDITION_ID --amount 100`,
		Annotations: map[string]string{"pp:onchain": "ctf.merge"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				market = args[0]
			}
			if (market == "" || amount == 0) && !flags.dryRun {
				return usageErr(fmt.Errorf("required: <market> arg + --amount FLAG"))
			}
			if dryRunOK(flags) {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), ctfStubPayload("merge", market, amount), flags)
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount of merged tokens to convert back to USDC (required)")
	return cmd
}

func newCtfRedeemCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "redeem <market>",
		Short:       "Redeem winning outcome tokens for USDC after a market resolves (documented stub).",
		Example:     `  polymarket-pp-cli ctf redeem 0xCONDITION_ID`,
		Annotations: map[string]string{"pp:onchain": "ctf.redeem"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				market = args[0]
			}
			if market == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("required: <market> arg (CTF condition ID)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), ctfStubPayload("redeem", market, 0), flags)
		},
	}
	return cmd
}

// ctfStubPayload returns the documented calldata for a CTF on-chain op.
// Real broadcast deferred to v0.2; this build's stub exposes the exact
// payload an operator would send via the official Rust CLI or ethers.js.
func ctfStubPayload(op, market string, amount float64) map[string]any {
	addresses := map[string]string{
		"ConditionalTokens": "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
		"NegRiskAdapter":    "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		"Exchange":          "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E",
	}
	functionSelectors := map[string]string{
		"split":  "0xa312ba18", // splitPosition(IERC20,bytes32,bytes32,uint256[],uint256)
		"merge":  "0xfae0d048", // mergePositions(IERC20,bytes32,bytes32,uint256[],uint256)
		"redeem": "0x2eb2c2d6", // redeemPositions(IERC20,bytes32,bytes32,uint256[])
	}
	return map[string]any{
		"op":                op,
		"market":            market,
		"amount_usdc":       amount,
		"contract_address":  addresses["ConditionalTokens"],
		"function_selector": functionSelectors[op],
		"expected_event":    fmt.Sprintf("PayoutRedemption(redeemer, collateralToken, parentCollectionId, conditionId, indexSets, payout) — %s", op),
		"status":            "NOT_BROADCAST",
		"broadcast_note":    "Live on-chain broadcast deferred to v0.2 (needs go-ethereum + Polygon RPC). Send this calldata via the official Polymarket Rust CLI's `polymarket ctf " + op + "` or via ethers.js. Exit code 0 — pipelines can plan against this payload safely.",
	}
}
