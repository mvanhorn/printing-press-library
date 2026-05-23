// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// On-chain approval primitives for Polymarket trading.
//
// `approve status` reads current ERC-20 allowance + ERC-1155 setApprovalForAll
// from Polygon mainnet via the configured RPC.
//
// `approve set` broadcasts the 6 approvals Polymarket requires:
//   - USDC.e  approve(CTFExchange,        MaxUint256)
//   - USDC.e  approve(NegRiskCTFExchange, MaxUint256)
//   - USDC.e  approve(NegRiskAdapter,     MaxUint256)
//   - CTF     setApprovalForAll(CTFExchange,        true)
//   - CTF     setApprovalForAll(NegRiskCTFExchange, true)
//   - CTF     setApprovalForAll(NegRiskAdapter,     true)
//
// Skips approvals that are already set (idempotent — re-runs safely cost
// zero gas). Default --dry-run prints what would be sent; --broadcast
// actually broadcasts; --yes or --agent bypasses the confirmation prompt.

package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/onchain"
)

type approvalRow struct {
	Token       string `json:"token"`
	TokenName   string `json:"token_name"`
	Spender     string `json:"spender"`
	SpenderName string `json:"spender_name"`
	Kind        string `json:"kind"` // "erc20" or "erc1155"
	AlreadySet  bool   `json:"already_set"`
	TxHash      string `json:"tx_hash,omitempty"`
	TxLink      string `json:"polygonscan,omitempty"`
	Receipt     string `json:"receipt,omitempty"`
	Error       string `json:"error,omitempty"`
}

// approvals returns the 6 (asset, spender) pairs the CLOB requires.
// Kind tags whether the asset is ERC-20 (USDC.e) or ERC-1155 (CTF).
func approvals() []approvalRow {
	return []approvalRow{
		{Token: onchain.USDCe, TokenName: "USDC.e", Spender: onchain.CTFExchangeAddr, SpenderName: "CTFExchange", Kind: "erc20"},
		{Token: onchain.USDCe, TokenName: "USDC.e", Spender: onchain.NegRiskCTFExchangeAddr, SpenderName: "NegRiskCTFExchange", Kind: "erc20"},
		{Token: onchain.USDCe, TokenName: "USDC.e", Spender: onchain.NegRiskAdapterAddr, SpenderName: "NegRiskAdapter", Kind: "erc20"},
		{Token: onchain.CTFAddr, TokenName: "CTF", Spender: onchain.CTFExchangeAddr, SpenderName: "CTFExchange", Kind: "erc1155"},
		{Token: onchain.CTFAddr, TokenName: "CTF", Spender: onchain.NegRiskCTFExchangeAddr, SpenderName: "NegRiskCTFExchange", Kind: "erc1155"},
		{Token: onchain.CTFAddr, TokenName: "CTF", Spender: onchain.NegRiskAdapterAddr, SpenderName: "NegRiskAdapter", Kind: "erc1155"},
	}
}

func newApproveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Set / check the 6 on-chain ERC-20 + ERC-1155 approvals required to trade on Polymarket.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newApproveSetCmd(flags))
	cmd.AddCommand(newApproveStatusCmd(flags))
	return cmd
}

func newApproveSetCmd(flags *rootFlags) *cobra.Command {
	var broadcast, wait bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the 6 on-chain approvals required to trade on Polymarket (USDC.e×3 + CTF×3).",
		Long: `Broadcast Polymarket trading approvals. Reads PK from env (or config),
checks current allowance/operator-status for each of the 6 required
spenders, and broadcasts approve()/setApprovalForAll() ONLY where
missing.

  --dry-run         print what would be sent (no tx)
  --broadcast       actually send transactions
  --yes / --agent   bypass confirmation prompt
  --wait            block until each tx receipt confirms (status=1)

Polygon mainnet gas cost: ~0.001 MATIC per approve (~$0.001), so the
full 6-tx setup costs well under $0.10 even at peak. POLYGON_RPC_URL
must be set (see config).`,
		Example: `  polymarket-pp-cli approve set --dry-run
  polymarket-pp-cli approve set --broadcast --yes --wait`,
		Annotations: map[string]string{"pp:onchain": "approve.set"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			rpcURL := os.Getenv("POLYGON_RPC_URL")
			if rpcURL == "" {
				return fmt.Errorf("POLYGON_RPC_URL not set in env. Required for on-chain broadcast")
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

			owner := signer.Address()
			rows := approvals()

			// Pass 1: read current state for every pair.
			for i := range rows {
				token := common.HexToAddress(rows[i].Token)
				spender := common.HexToAddress(rows[i].Spender)
				if rows[i].Kind == "erc20" {
					allow, aerr := onchain.AllowanceERC20(ctx, client, token, owner, spender)
					if aerr != nil {
						rows[i].Error = aerr.Error()
						continue
					}
					// Treat any non-zero allowance as already-set. Polymarket
					// approves with MaxUint256 so the check is unambiguous.
					rows[i].AlreadySet = allow.Sign() > 0
				} else {
					ok, oerr := onchain.IsApprovedForAllERC1155(ctx, client, token, owner, spender)
					if oerr != nil {
						rows[i].Error = oerr.Error()
						continue
					}
					rows[i].AlreadySet = ok
				}
			}

			if !broadcast {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"owner":            owner.Hex(),
					"chain_id":         onchain.PolygonChainID,
					"broadcast_status": "DRY_RUN (no --broadcast flag)",
					"approvals":        rows,
				}, flags)
			}

			if !flags.yes && !flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"owner":            owner.Hex(),
					"chain_id":         onchain.PolygonChainID,
					"broadcast_status": "AWAITING_CONFIRMATION",
					"confirm_hint":     "Re-run with --yes (or --agent) to actually broadcast.",
					"approvals":        rows,
				}, flags)
			}

			// Pass 2: broadcast missing approvals.
			pk := signer.PrivateKey()
			for i := range rows {
				if rows[i].AlreadySet || rows[i].Error != "" {
					continue
				}
				token := common.HexToAddress(rows[i].Token)
				spender := common.HexToAddress(rows[i].Spender)
				var hash common.Hash
				var ferr error
				if rows[i].Kind == "erc20" {
					hash, ferr = onchain.ApproveERC20(ctx, client, pk, token, spender, onchain.MaxUint256)
				} else {
					hash, ferr = onchain.SetApprovalForAllERC1155(ctx, client, pk, token, spender, true)
				}
				if ferr != nil {
					rows[i].Error = ferr.Error()
					continue
				}
				rows[i].TxHash = hash.Hex()
				rows[i].TxLink = onchain.PolygonscanLink(hash)

				if wait {
					rcpt, werr := onchain.WaitMinedByHash(ctx, client, hash)
					if werr != nil {
						rows[i].Error = "wait receipt: " + werr.Error()
						continue
					}
					if rcpt.Status == 1 {
						rows[i].Receipt = "success"
						rows[i].AlreadySet = true
					} else {
						rows[i].Receipt = "reverted"
					}
				}
			}

			anyBroadcast := false
			for _, r := range rows {
				if r.TxHash != "" {
					anyBroadcast = true
					break
				}
			}
			status := "ALL_APPROVALS_ALREADY_SET"
			if anyBroadcast {
				status = "APPROVALS_BROADCAST"
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"owner":            owner.Hex(),
				"chain_id":         onchain.PolygonChainID,
				"broadcast_status": status,
				"approvals":        rows,
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Send approval transactions on-chain")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until each tx receipt confirms")
	return cmd
}

func newApproveStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Inspect current on-chain allowances + operator approvals for the configured wallet.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:onchain": "approve.status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rpcURL := os.Getenv("POLYGON_RPC_URL")
			if rpcURL == "" {
				return fmt.Errorf("POLYGON_RPC_URL not set in env")
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
			owner := signer.Address()
			rows := approvals()
			for i := range rows {
				token := common.HexToAddress(rows[i].Token)
				spender := common.HexToAddress(rows[i].Spender)
				if rows[i].Kind == "erc20" {
					allow, aerr := onchain.AllowanceERC20(ctx, client, token, owner, spender)
					if aerr != nil {
						rows[i].Error = aerr.Error()
						continue
					}
					rows[i].AlreadySet = allow.Sign() > 0
					if rows[i].AlreadySet && allow.Cmp(onchain.MaxUint256) != 0 {
						// Surface partial allowance so users know they may need to re-approve before larger trades.
						rows[i].Receipt = "partial allowance: " + allow.String()
					}
				} else {
					ok, oerr := onchain.IsApprovedForAllERC1155(ctx, client, token, owner, spender)
					if oerr != nil {
						rows[i].Error = oerr.Error()
						continue
					}
					rows[i].AlreadySet = ok
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"owner":     owner.Hex(),
				"chain_id":  onchain.PolygonChainID,
				"approvals": rows,
			}, flags)
		},
	}
	return cmd
}

// Quiet unused-import warnings in case the file gets pared back later.
var _ = strings.HasPrefix
var _ = big.NewInt
var _ = context.TODO
var _ = hex.EncodeToString
