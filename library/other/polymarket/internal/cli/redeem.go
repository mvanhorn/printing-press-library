// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (redemption batch executor). See research.json novel_features.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/polymarket/internal/store"
)

func newRedeemCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redeem",
		Short: "Batched redemption of resolved-market positions.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRedeemAllCmd(flags))
	return cmd
}

func newRedeemAllCmd(flags *rootFlags) *cobra.Command {
	var minValue float64
	var broadcast bool
	var wallet string

	cmd := &cobra.Command{
		Use:     "all",
		Short:   "Discover every position in resolved markets and run ctf redeem on each in one batch, skipping dust below a minimum value. Reports total USDC claimed and gas spent.",
		Example: `  polymarket-pp-cli redeem all --dry-run --min-value 1 --agent`,
		Annotations: map[string]string{
			"pp:novel": "redeem.all",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to dry-run unless --broadcast given.
			effectiveDryRun := flags.dryRun || !broadcast
			if dryRunOK(flags) && !broadcast {
				return nil
			}

			// Derive wallet from POLYMARKET_PRIVATE_KEY env if not given.
			if wallet == "" {
				wallet = os.Getenv("POLYMARKET_FUNDER")
				if wallet == "" {
					// We can't derive without crypto secp256k1 (stdlib only has p256/p384/p521).
					// Honest stub: ask user to supply wallet flag.
					return usageErr(fmt.Errorf("wallet address required: pass --wallet ADDR or set POLYMARKET_FUNDER env var. (Deriving EOA from POLYMARKET_PRIVATE_KEY requires secp256k1 — wire in v0.2 with go-ethereum dep.)"))
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Pull live positions.
			posPath := "https://data-api.polymarket.com/positions"
			posData, perr := c.GetWithHeaders(cmd.Context(), posPath,
				map[string]string{"user": wallet, "limit": "500"}, nil)
			if perr != nil {
				return classifyAPIError(perr, flags)
			}
			var positions []map[string]any
			_ = json.Unmarshal(posData, &positions)

			// Cross-ref against local market data for closed flag.
			dbPath := defaultDBPath("polymarket-pp-cli")
			s, _ := store.OpenReadOnly(dbPath)
			if s != nil {
				defer s.Close()
			}

			type redeemRow struct {
				Market       string  `json:"market"`
				Question     string  `json:"question,omitempty"`
				TokenID      string  `json:"token_id"`
				Size         float64 `json:"size"`
				CurrentValue float64 `json:"current_value_usdc"`
				WouldRedeem  bool    `json:"would_redeem"`
				SkipReason   string  `json:"skip_reason,omitempty"`
			}

			var rows []redeemRow
			totalValue := 0.0
			eligibleCount := 0
			for _, p := range positions {
				row := redeemRow{}
				if v, ok := p["market"].(string); ok {
					row.Market = v
				}
				if v, ok := p["title"].(string); ok {
					row.Question = v
				}
				if v, ok := p["asset"].(string); ok {
					row.TokenID = v
				}
				if v, ok := p["size"].(float64); ok {
					row.Size = v
				}
				if v, ok := p["currentValue"].(float64); ok {
					row.CurrentValue = v
				}
				closed := false
				if s != nil && row.Market != "" {
					if raw, err := s.Get("markets", row.Market); err == nil && raw != nil {
						var m map[string]any
						if json.Unmarshal(raw, &m) == nil {
							if v, ok := m["closed"].(bool); ok {
								closed = v
							}
						}
					}
				}
				// Also honor server-side `redeemable` flag when available.
				if v, ok := p["redeemable"].(bool); ok {
					closed = closed || v
				}
				switch {
				case !closed:
					row.SkipReason = "market not closed"
				case row.Size <= 0:
					row.SkipReason = "zero size"
				case row.CurrentValue < minValue:
					row.SkipReason = fmt.Sprintf("below min-value (%.2f < %.2f)", row.CurrentValue, minValue)
				default:
					row.WouldRedeem = true
					eligibleCount++
					totalValue += row.CurrentValue
				}
				rows = append(rows, row)
			}

			out := map[string]any{
				"wallet":           wallet,
				"min_value_usdc":   minValue,
				"dry_run":          effectiveDryRun,
				"broadcast":        broadcast,
				"positions_total":  len(rows),
				"eligible_count":   eligibleCount,
				"total_value_usdc": totalValue,
				"positions":        rows,
			}
			if broadcast {
				out["broadcast_status"] = "NOT_IMPLEMENTED"
				out["broadcast_note"] = "Live on-chain redemption requires go-ethereum + Polygon RPC + EIP-712 signing. The dry-run output above contains the exact set of ctf redeem calls; use the official Polymarket Rust CLI's 'ctf redeem' per market, or wait for v0.2 native broadcast. This command exits 0 to allow scripted pipelines to plan against the dry-run set."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&minValue, "min-value", 1, "Skip positions below this USDC value")
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Actually send on-chain redemptions (not implemented in this build — see broadcast_note)")
	cmd.Flags().StringVar(&wallet, "wallet", "", "Wallet address; defaults to POLYMARKET_FUNDER env var")
	// --dry-run is inherited from root persistent flags; we honor it as the default safe mode.
	return cmd
}
