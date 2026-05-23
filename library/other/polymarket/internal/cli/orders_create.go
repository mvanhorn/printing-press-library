// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: orders create (sibling under orders parent).

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newOrdersCreateCmd(flags *rootFlags) *cobra.Command {
	var token, side, orderType, expiration string
	var price, size float64
	var preflight, broadcast bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Compose & (optionally preflight) a new CLOB order.",
		Long: `Compose a CLOB order: produce the EIP-712 hash and canonical request body.
Default mode does not broadcast — pass --broadcast to actually submit (documented stub).
Pass --preflight to additionally probe spec endpoints for tick-size / min-size
issues before signing.`,
		Example: `  polymarket-pp-cli orders create --token 0xTOKEN --side buy --price 0.55 --size 100 --type GTC
  polymarket-pp-cli orders create --token 0xTOKEN --side sell --price 0.6 --size 50 --type GTD --expiration 1750000000 --preflight`,
		Annotations: map[string]string{"pp:novel": "orders.create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if (!cmd.Flags().Changed("token") || !cmd.Flags().Changed("side") ||
				!cmd.Flags().Changed("price") || !cmd.Flags().Changed("size")) && !flags.dryRun {
				return usageErr(fmt.Errorf("required flags: --token, --side, --price, --size"))
			}
			if dryRunOK(flags) {
				return nil
			}
			side = strings.ToUpper(side)
			if side != "BUY" && side != "SELL" {
				return usageErr(fmt.Errorf("--side must be buy or sell"))
			}
			orderType = strings.ToUpper(orderType)
			validTypes := map[string]bool{"GTC": true, "GTD": true, "FOK": true, "FAK": true}
			if !validTypes[orderType] {
				return usageErr(fmt.Errorf("--type must be one of GTC, GTD, FOK, FAK"))
			}

			body := map[string]any{
				"tokenId":   token,
				"side":      side,
				"price":     price,
				"size":      size,
				"orderType": orderType,
			}
			if orderType == "GTD" && expiration != "" {
				body["expiration"] = expiration
			}

			// Compute a deterministic hash preview of the canonical body.
			bodyBytes, _ := json.Marshal(body)
			h := sha256.Sum256(bodyBytes)
			canonHash := hex.EncodeToString(h[:])

			out := map[string]any{
				"order":              body,
				"canonical_sha256":   canonHash,
				"eip712_hash_status": "NOT_IMPLEMENTED",
				"eip712_note":        "EIP-712 typed-data hashing + signing requires go-ethereum (~250 LOC). Honest stub: the sha256 above identifies the canonical body for replay protection but is NOT a valid Polymarket order signature.",
			}

			if preflight {
				preflightResults := map[string]any{}
				c, err := flags.newClient()
				if err == nil {
					// Pull market metadata (tick size, min size) via spec endpoint.
					marketRaw, merr := c.GetWithHeaders(cmd.Context(),
						"https://clob.polymarket.com/markets/"+token, nil, nil)
					if merr == nil {
						var m map[string]any
						_ = json.Unmarshal(marketRaw, &m)
						if v, ok := m["min_order_size"]; ok {
							preflightResults["min_order_size"] = v
						}
						if v, ok := m["minimum_tick_size"]; ok {
							preflightResults["minimum_tick_size"] = v
						}
						if v, ok := m["accepting_orders"]; ok {
							preflightResults["accepting_orders"] = v
						}
					} else {
						preflightResults["market_lookup_error"] = merr.Error()
					}
				}
				out["preflight"] = preflightResults
			}

			if broadcast {
				out["broadcast_status"] = "NOT_IMPLEMENTED"
				out["broadcast_note"] = "Live order submission requires EIP-712 signing — same dep as `auth derive`. Use the official Polymarket Rust CLI's `polymarket clob create-order` to broadcast this body, or wait for v0.2 native signing."
				out["broadcast"] = true
			}
			out["created_at"] = time.Now().UTC().Format(time.RFC3339)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Token ID (CLOB token ID for the outcome)")
	cmd.Flags().StringVar(&side, "side", "", "buy or sell (required)")
	cmd.Flags().Float64Var(&price, "price", 0, "Limit price in implied probability (0–1)")
	cmd.Flags().Float64Var(&size, "size", 0, "Order size in outcome tokens")
	cmd.Flags().StringVar(&orderType, "type", "GTC", "Order type: GTC, GTD, FOK, FAK")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Unix-seconds expiration timestamp (required for GTD)")
	cmd.Flags().BoolVar(&preflight, "preflight", false, "Probe spec endpoints for tick-size / min-size compliance before signing")
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Actually submit the order on-chain (not implemented — see broadcast_note)")
	return cmd
}
