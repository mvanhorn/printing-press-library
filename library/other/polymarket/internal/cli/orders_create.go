// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// orders create — compose, optionally preflight, optionally broadcast a CLOB order.
//
// Without --broadcast: returns the canonical order body + EIP-712 hash for
// review or replay through another tool.
// With --broadcast: signs the order with the loaded EOA, builds L2 HMAC
// headers, and POSTs to Polymarket CLOB /order. Returns the orderID +
// status on success or the CLOB's structured error on rejection.

package cli

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/clob"
	"polymarket-pp-cli/internal/config"
	"polymarket-pp-cli/internal/eip712"
)

func newOrdersCreateCmd(flags *rootFlags) *cobra.Command {
	var token, side, orderType, expiration, funder string
	var price, size float64
	var preflight, broadcast, negRisk bool
	var sigType int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Compose, sign, and (optionally) broadcast a new CLOB limit order.",
		Long: `Compose a Polymarket CLOB limit order. Without --broadcast the
command prints the canonical order body + EIP-712 hash for review.
With --broadcast the order is signed with your EOA, packaged with
L2 HMAC auth headers, and POSTed to clob.polymarket.com/order.

Amounts are computed in 6-decimal fixed point:
  BUY:  makerAmount = USDC,  takerAmount = outcome tokens
  SELL: makerAmount = tokens, takerAmount = USDC

For neg-risk markets (multi-outcome mutually exclusive) pass --neg-risk
so the EIP-712 domain pins to the NegRisk CTF Exchange contract.

Run 'auth derive' first to mint L2 credentials. Broadcasting orders
moves real USDC on Polygon mainnet — start small.`,
		Example: `  polymarket-pp-cli orders create --token <token_id> --side buy --price 0.55 --size 100 --type GTC
  polymarket-pp-cli orders create --token <token_id> --side sell --price 0.6 --size 50 --preflight
  polymarket-pp-cli orders create --token <token_id> --side buy --price 0.02 --size 50 --broadcast --yes`,
		Annotations: map[string]string{"pp:novel": "orders.create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if (!cmd.Flags().Changed("token") || !cmd.Flags().Changed("side") ||
				!cmd.Flags().Changed("price") || !cmd.Flags().Changed("size")) && !flags.dryRun {
				return usageErr(fmt.Errorf("required flags: --token, --side, --price, --size"))
			}
			if dryRunOK(flags) {
				return nil
			}

			sideUint, err := clob.SideFromString(side)
			if err != nil {
				return usageErr(err)
			}
			orderType = strings.ToUpper(orderType)
			validTypes := map[string]bool{"GTC": true, "GTD": true, "FOK": true, "FAK": true}
			if !validTypes[orderType] {
				return usageErr(fmt.Errorf("--type must be one of GTC, GTD, FOK, FAK"))
			}

			tokenBig, ok := new(big.Int).SetString(strings.TrimPrefix(token, "0x"), 0)
			if !ok || tokenBig.Sign() == 0 {
				// Try decimal as well — Polymarket CLOB returns token IDs as decimal strings.
				tokenBig, ok = new(big.Int).SetString(token, 10)
				if !ok {
					return usageErr(fmt.Errorf("--token must be a valid uint256 (decimal or hex)"))
				}
			}

			// Load signer + config for funder + L2 creds.
			signer, err := loadSigner(flags)
			if err != nil {
				return authErr(err)
			}
			cfg, _ := config.Load(flags.configPath)
			if cfg == nil {
				cfg = &config.Config{}
			}
			if funder == "" {
				funder = cfg.PolymarketFunder
			}
			makerAddr := signer.Address()
			if funder != "" {
				if !common.IsHexAddress(funder) {
					return usageErr(fmt.Errorf("--funder is not a valid hex address: %s", funder))
				}
				makerAddr = common.HexToAddress(funder)
			}

			// Effective signature type: flag wins, fall back to config, then EOA.
			effectiveSigType := uint8(sigType)
			if !cmd.Flags().Changed("signature-type") && cfg.PolymarketSignatureType != "" {
				switch cfg.PolymarketSignatureType {
				case "1":
					effectiveSigType = 1
				case "2":
					effectiveSigType = 2
				}
			}

			salt, err := clob.SaltRandom256()
			if err != nil {
				return err
			}
			makerAmt, takerAmt := clob.AmountsForOrder(sideUint, price, size)
			exp := big.NewInt(0)
			if orderType == "GTD" && expiration != "" {
				if v, ok := new(big.Int).SetString(expiration, 10); ok {
					exp = v
				}
			}

			order := &eip712.Order{
				Salt:          salt,
				Maker:         makerAddr,
				Signer:        signer.Address(),
				Taker:         common.HexToAddress("0x0000000000000000000000000000000000000000"),
				TokenID:       tokenBig,
				MakerAmount:   makerAmt,
				TakerAmount:   takerAmt,
				Expiration:    exp,
				Nonce:         big.NewInt(0),
				FeeRateBps:    big.NewInt(0),
				Side:          sideUint,
				SignatureType: effectiveSigType,
			}

			orderTD := eip712.OrderTypedData(order, negRisk)
			digest, err := eip712.Digest(orderTD)
			if err != nil {
				return fmt.Errorf("EIP-712 order digest: %w", err)
			}
			orderSigHex, err := signer.SignToHex(digest)
			if err != nil {
				return fmt.Errorf("sign order: %w", err)
			}
			orderHashHex := "0x" + common.Bytes2Hex(digest)

			// Wire-format Order JSON object (different shape than struct: side is string, amounts are decimal strings).
			wireOrder := map[string]any{
				"salt":          salt.String(),
				"maker":         makerAddr.Hex(),
				"signer":        signer.AddressHex(),
				"taker":         "0x0000000000000000000000000000000000000000",
				"tokenId":       tokenBig.String(),
				"makerAmount":   makerAmt.String(),
				"takerAmount":   takerAmt.String(),
				"expiration":    exp.String(),
				"nonce":         "0",
				"feeRateBps":    "0",
				"side":          clob.SideString(sideUint),
				"signatureType": int(effectiveSigType),
				"signature":     orderSigHex,
			}

			out := map[string]any{
				"order":           wireOrder,
				"eip712_hash":     orderHashHex,
				"order_type":      orderType,
				"neg_risk":        negRisk,
				"signature_type":  effectiveSigType,
				"maker_address":   makerAddr.Hex(),
				"signer_address":  signer.AddressHex(),
				"price":           price,
				"size":            size,
				"created_at":      time.Now().UTC().Format(time.RFC3339),
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if preflight {
				preflightResults := map[string]any{}
				marketRaw, merr := c.GetWithHeaders(cmd.Context(),
					"https://clob.polymarket.com/markets/"+tokenBig.String(), nil, nil)
				if merr != nil {
					preflightResults["market_lookup_error"] = merr.Error()
				} else {
					var m map[string]any
					_ = json.Unmarshal(marketRaw, &m)
					for _, k := range []string{"min_order_size", "minimum_tick_size", "accepting_orders", "neg_risk", "active"} {
						if v, ok := m[k]; ok {
							preflightResults[k] = v
						}
					}
				}
				out["preflight"] = preflightResults
			}

			if !broadcast {
				out["broadcast_status"] = "DRY_RUN (no --broadcast flag)"
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Live broadcast path.
			if !flags.yes && !flags.agent {
				out["broadcast_status"] = "AWAITING_CONFIRMATION"
				out["confirm_hint"] = "Re-run with --yes (or --agent) to actually submit this order on-chain."
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// L2 creds required for POST /order.
			creds := clob.L2Creds{
				APIKey:     cfg.PolymarketApiKey,
				Secret:     cfg.PolymarketApiSecret,
				Passphrase: cfg.PolymarketApiPassphrase,
			}
			if creds.APIKey == "" {
				return authErr(fmt.Errorf("L2 credentials missing — run `polymarket-pp-cli auth derive` first"))
			}

			postBody := map[string]any{
				"order":     wireOrder,
				"owner":     creds.APIKey,
				"orderType": orderType,
			}
			bodyBytes, err := json.Marshal(postBody)
			if err != nil {
				return fmt.Errorf("marshal /order body: %w", err)
			}
			headers, err := clob.BuildL2Headers(creds, signer.AddressHex(), "POST", "/order", bodyBytes, time.Now().Unix())
			if err != nil {
				return err
			}

			rawResp, status, perr := c.PostWithHeaders(cmd.Context(),
				"https://clob.polymarket.com/order", postBody, headers)
			if perr != nil {
				return classifyAPIError(perr, flags)
			}
			if status >= 400 {
				out["broadcast_status"] = fmt.Sprintf("HTTP %d", status)
				out["broadcast_response"] = string(rawResp)
				return fmt.Errorf("POST /order returned HTTP %d: %s", status, string(rawResp))
			}
			var resp struct {
				Success            bool     `json:"success"`
				ErrorMsg           string   `json:"errorMsg"`
				OrderID            string   `json:"orderID"`
				OrderHashes        []string `json:"orderHashes"`
				TransactionsHashes []string `json:"transactionsHashes"`
				Status             string   `json:"status"`
			}
			if err := json.Unmarshal(rawResp, &resp); err != nil {
				return fmt.Errorf("parse /order response: %w (body=%s)", err, string(rawResp))
			}
			out["broadcast_status"] = "SUBMITTED"
			out["broadcast_response"] = resp
			out["order_id"] = resp.OrderID
			out["clob_status"] = resp.Status
			if !resp.Success && resp.ErrorMsg != "" {
				out["broadcast_status"] = "REJECTED_BY_CLOB"
				out["error_msg"] = resp.ErrorMsg
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Token ID (CLOB token ID for the outcome — decimal or 0x-hex uint256)")
	cmd.Flags().StringVar(&side, "side", "", "buy or sell (required)")
	cmd.Flags().Float64Var(&price, "price", 0, "Limit price in implied probability (0–1)")
	cmd.Flags().Float64Var(&size, "size", 0, "Order size in outcome tokens (whole tokens, not 6-decimal)")
	cmd.Flags().StringVar(&orderType, "type", "GTC", "Order type: GTC, GTD, FOK, FAK")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Unix-seconds expiration timestamp (required for GTD)")
	cmd.Flags().StringVar(&funder, "funder", "", "Funder address (defaults to config; required for signature-type 1 or 2)")
	cmd.Flags().IntVar(&sigType, "signature-type", 0, "Signature type: 0 EOA, 1 Polymarket proxy, 2 Polymarket gnosis-safe")
	cmd.Flags().BoolVar(&preflight, "preflight", false, "Probe /markets/<token> for tick-size / min-size / accepting_orders before signing")
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Actually submit the order to CLOB (requires --yes or --agent to bypass confirmation)")
	cmd.Flags().BoolVar(&negRisk, "neg-risk", false, "Pin EIP-712 domain to the NegRisk CTF Exchange (for neg-risk markets)")
	return cmd
}
