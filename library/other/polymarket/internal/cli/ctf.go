// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// ConditionalTokens (CTF) on-chain primitives. `ctf redeem` ships a live
// broadcast path via go-ethereum + Polygon RPC. `ctf split` and `ctf merge`
// remain documented stubs in v0.2 (live broadcast in v0.3 — same go-ethereum
// pattern as redeem, just different ABI selector + amount parameter).

package cli

import (
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/onchain"
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
	var broadcast, wait, negRisk bool
	var indexSetsCSV string

	cmd := &cobra.Command{
		Use:   "redeem <market>",
		Short: "Redeem winning outcome tokens for USDC after a market resolves.",
		Long: `Call CTF.redeemPositions() to convert winning tokens back into USDC.
For binary markets the default --index-sets "1,2" tries both YES and NO
positions; the CTF contract auto-skips outcomes the caller holds zero of.

  --broadcast       actually send the redeem tx
  --yes / --agent   bypass confirmation prompt
  --wait            block until tx receipt confirms
  --neg-risk        target NegRiskAdapter instead of CTF for neg-risk markets
  --index-sets      override the [1,2] default (comma-separated uint256 values)

If the market is not resolved or the caller holds zero winning tokens,
the tx mines but transfers nothing — gas spent regardless. Check market
status via 'polymarket-pp-cli markets get <condition_id>' first.`,
		Example: `  polymarket-pp-cli ctf redeem 0xCONDITION_ID --broadcast --yes --wait
  polymarket-pp-cli ctf redeem 0xCONDITION_ID --neg-risk --broadcast --yes`,
		Annotations: map[string]string{"pp:onchain": "ctf.redeem"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				market = args[0]
			}
			if market == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("required: <market> arg (CTF condition ID, 0x-prefixed bytes32)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			conditionId := common.HexToHash(market)
			if conditionId == (common.Hash{}) {
				return usageErr(fmt.Errorf("--market must be a valid bytes32 hex (got %q)", market))
			}

			// Parse index sets — default [1,2] for binary markets.
			indexSets := []*big.Int{big.NewInt(1), big.NewInt(2)}
			if indexSetsCSV != "" {
				parts := strings.Split(indexSetsCSV, ",")
				indexSets = make([]*big.Int, 0, len(parts))
				for _, p := range parts {
					n, ok := new(big.Int).SetString(strings.TrimSpace(p), 10)
					if !ok {
						return usageErr(fmt.Errorf("--index-sets value %q is not a valid decimal uint256", p))
					}
					indexSets = append(indexSets, n)
				}
			}

			contractAddr := onchain.CTFAddr
			contractName := "ConditionalTokens"
			if negRisk {
				contractAddr = onchain.NegRiskAdapterAddr
				contractName = "NegRiskAdapter"
			}

			if !broadcast {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"market":            market,
					"contract":          contractAddr,
					"contract_name":     contractName,
					"collateral_token":  onchain.USDCe,
					"index_sets":        indexSetsCSVOrDefault(indexSets),
					"function_selector": "0x2eb2c2d6",
					"broadcast_status":  "DRY_RUN (no --broadcast flag)",
				}, flags)
			}

			if !flags.yes && !flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"market":           market,
					"broadcast_status": "AWAITING_CONFIRMATION",
					"confirm_hint":     "Re-run with --yes (or --agent) to actually broadcast.",
				}, flags)
			}

			rpcURL := os.Getenv("POLYGON_RPC_URL")
			if rpcURL == "" {
				return fmt.Errorf("POLYGON_RPC_URL not set in env (required for broadcast)")
			}
			signer, err := loadSigner(flags)
			if err != nil {
				return authErr(err)
			}
			ctx := cmd.Context()
			client, err := onchain.Dial(ctx, rpcURL)
			if err != nil {
				return fmt.Errorf("dial Polygon RPC: %w", err)
			}
			defer client.Close()

			hash, err := onchain.RedeemCTF(ctx, client, signer.PrivateKey(),
				common.HexToAddress(contractAddr),
				common.HexToAddress(onchain.USDCe),
				conditionId, indexSets)
			if err != nil {
				return fmt.Errorf("broadcast redeemPositions: %w", err)
			}

			out := map[string]any{
				"market":           market,
				"contract":         contractAddr,
				"contract_name":    contractName,
				"collateral_token": onchain.USDCe,
				"index_sets":       indexSetsCSVOrDefault(indexSets),
				"tx_hash":          hash.Hex(),
				"polygonscan":      onchain.PolygonscanLink(hash),
				"broadcast_status": "SUBMITTED",
			}
			if wait {
				rcpt, werr := onchain.WaitMinedByHash(ctx, client, hash)
				if werr != nil {
					out["wait_error"] = werr.Error()
				} else if rcpt.Status == 1 {
					out["receipt"] = "success"
					out["gas_used"] = rcpt.GasUsed
				} else {
					out["receipt"] = "reverted"
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Send the redeemPositions transaction on-chain")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until tx receipt confirms")
	cmd.Flags().BoolVar(&negRisk, "neg-risk", false, "Target NegRiskAdapter instead of CTF (neg-risk markets)")
	cmd.Flags().StringVar(&indexSetsCSV, "index-sets", "1,2", "Comma-separated uint256 outcome index sets to redeem")
	return cmd
}

// indexSetsCSVOrDefault renders the index-sets list as a comma-separated
// string for JSON output. Pulled out so the dry-run / broadcast envelopes
// share rendering and the caller's --index-sets flag round-trips exactly.
func indexSetsCSVOrDefault(s []*big.Int) string {
	parts := make([]string, 0, len(s))
	for _, n := range s {
		parts = append(parts, n.String())
	}
	return strings.Join(parts, ",")
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
