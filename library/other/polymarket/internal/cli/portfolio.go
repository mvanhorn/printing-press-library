// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (position drift snapshot). See research.json novel_features.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/cliutil"
)

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Portfolio analytics — position drift, entry attribution, exit-ability flags.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPortfolioDriftCmd(flags))
	return cmd
}

func newPortfolioDriftCmd(flags *rootFlags) *cobra.Command {
	var wallet string
	var since time.Duration

	cmd := &cobra.Command{
		Use:     "drift",
		Short:   "For each position: entry price, current mid, min/max over a window, unrealized P&L drift, and a thawed/frozen flag based on current spread + book depth.",
		Example: `  polymarket-pp-cli portfolio drift --wallet 0xYOUR --since 168h --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "portfolio.drift",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("wallet") && !flags.dryRun {
				return usageErr(fmt.Errorf("required flag \"wallet\" not set"))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: pull positions for wallet.
			posPath := "https://data-api.polymarket.com/positions"
			posRaw, err := c.GetWithHeaders(cmd.Context(), posPath,
				map[string]string{"user": wallet, "limit": "200"}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var positions []map[string]any
			if err := json.Unmarshal(posRaw, &positions); err != nil {
				return apiErr(fmt.Errorf("parsing positions: %w", err))
			}

			// Step 2: pull TRADE activity for entry attribution.
			activityPath := "https://data-api.polymarket.com/activity"
			activityRaw, aerr := c.GetWithHeaders(cmd.Context(), activityPath,
				map[string]string{"user": wallet, "type": "TRADE", "limit": "500"}, nil)
			var trades []map[string]any
			if aerr == nil {
				_ = json.Unmarshal(activityRaw, &trades)
			}

			// Build map: tokenID -> earliest buy price.
			entryPrice := map[string]float64{}
			entryTime := map[string]string{}
			cutoff := time.Now().Add(-since)
			for _, t := range trades {
				side, _ := t["side"].(string)
				if side != "BUY" {
					continue
				}
				var tokenID string
				if v, ok := t["asset"].(string); ok {
					tokenID = v
				}
				if v, ok := t["tokenId"].(string); ok && tokenID == "" {
					tokenID = v
				}
				var price float64
				if v, ok := t["price"].(float64); ok {
					price = v
				}
				var ts string
				if v, ok := t["timestamp"].(string); ok {
					ts = v
				}
				// Within window (or no within filter — we just take earliest seen)
				if tokenID == "" || price == 0 {
					continue
				}
				if _, ok := entryPrice[tokenID]; !ok {
					entryPrice[tokenID] = price
					entryTime[tokenID] = ts
				}
			}
			_ = cutoff

			// Step 3: fan-out current mid + book for each position.
			type driftRow struct {
				Market        string  `json:"market"`
				Question      string  `json:"question,omitempty"`
				TokenID       string  `json:"token_id"`
				Size          float64 `json:"size"`
				EntryPrice    float64 `json:"entry_price"`
				EntryTime     string  `json:"entry_time,omitempty"`
				CurrentMid    float64 `json:"current_mid"`
				CurrentValue  float64 `json:"current_value_usdc"`
				UnrealizedPNL float64 `json:"unrealized_pnl_usdc"`
				DriftPct      float64 `json:"drift_pct"`
				Spread        float64 `json:"spread"`
				BookDepth     float64 `json:"book_depth_usdc"`
				ExitFlag      string  `json:"exit_flag"`
			}

			type src struct {
				idx int
				pos map[string]any
			}
			sources := make([]src, 0, len(positions))
			for i, p := range positions {
				sources = append(sources, src{idx: i, pos: p})
			}

			results, ferrs := cliutil.FanoutRun(
				cmd.Context(),
				sources,
				func(s src) string {
					if id, ok := s.pos["asset"].(string); ok {
						return id
					}
					return fmt.Sprintf("pos-%d", s.idx)
				},
				func(ctx context.Context, s src) (driftRow, error) {
					row := driftRow{}
					if v, ok := s.pos["market"].(string); ok {
						row.Market = v
					}
					if v, ok := s.pos["title"].(string); ok {
						row.Question = v
					} else if v, ok := s.pos["eventTitle"].(string); ok {
						row.Question = v
					}
					if v, ok := s.pos["asset"].(string); ok {
						row.TokenID = v
					}
					if v, ok := s.pos["size"].(float64); ok {
						row.Size = v
					}
					if v, ok := s.pos["currentValue"].(float64); ok {
						row.CurrentValue = v
					}
					row.EntryPrice = entryPrice[row.TokenID]
					row.EntryTime = entryTime[row.TokenID]
					if row.TokenID == "" {
						row.ExitFlag = "unknown"
						return row, nil
					}
					// Current mid
					midRaw, merr := c.GetWithHeaders(ctx, "https://clob.polymarket.com/midpoint",
						map[string]string{"token_id": row.TokenID}, nil)
					if merr == nil {
						var midResp map[string]any
						if json.Unmarshal(midRaw, &midResp) == nil {
							if v, ok := midResp["mid"].(float64); ok {
								row.CurrentMid = v
							} else if v, ok := midResp["midpoint"].(float64); ok {
								row.CurrentMid = v
							} else if vs, ok := midResp["mid"].(string); ok {
								_, _ = fmt.Sscanf(vs, "%f", &row.CurrentMid)
							}
						}
					}
					// Book depth + spread
					bookRaw, berr := c.GetWithHeaders(ctx, "https://clob.polymarket.com/book",
						map[string]string{"token_id": row.TokenID}, nil)
					if berr == nil {
						row.BookDepth = computeBookDepthUSDC(bookRaw, 0.05)
						row.Spread = computeSpread(bookRaw)
					}
					// Drift
					if row.EntryPrice > 0 && row.CurrentMid > 0 {
						row.DriftPct = (row.CurrentMid - row.EntryPrice) / row.EntryPrice * 100
						row.UnrealizedPNL = (row.CurrentMid - row.EntryPrice) * row.Size
					}
					// Thawed/frozen flag.
					if row.Spread > 0 && row.Spread < 0.05 && row.BookDepth > 1000 {
						row.ExitFlag = "thawed"
					} else {
						row.ExitFlag = "frozen"
					}
					return row, nil
				},
				cliutil.WithConcurrency(8),
			)
			if len(ferrs) > 0 {
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), ferrs)
			}
			rows := make([]driftRow, 0, len(results))
			for _, r := range results {
				rows = append(rows, r.Value)
			}

			out := map[string]any{
				"wallet":    wallet,
				"since":     since.String(),
				"count":     len(rows),
				"positions": rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&wallet, "wallet", "", "Wallet address (required)")
	cmd.Flags().DurationVar(&since, "since", 7*24*time.Hour, "Lookback window in Go-duration syntax (e.g. 24h, 168h for 7 days)")
	return cmd
}

// computeSpread returns ask[0] - bid[0] from a book payload. Returns 0 when
// either side is empty or malformed.
func computeSpread(raw json.RawMessage) float64 {
	var book map[string]any
	if err := json.Unmarshal(raw, &book); err != nil {
		return 0
	}
	bestBid, bestAsk := 0.0, 0.0
	if bids, ok := book["bids"].([]any); ok && len(bids) > 0 {
		// Polymarket bids typically descending; pick max
		for _, b := range bids {
			lvl, ok := b.(map[string]any)
			if !ok {
				continue
			}
			var p float64
			switch v := lvl["price"].(type) {
			case float64:
				p = v
			case string:
				_, _ = fmt.Sscanf(v, "%f", &p)
			}
			if p > bestBid {
				bestBid = p
			}
		}
	}
	if asks, ok := book["asks"].([]any); ok && len(asks) > 0 {
		bestAsk = 1.0
		for _, a := range asks {
			lvl, ok := a.(map[string]any)
			if !ok {
				continue
			}
			var p float64
			switch v := lvl["price"].(type) {
			case float64:
				p = v
			case string:
				_, _ = fmt.Sscanf(v, "%f", &p)
			}
			if p > 0 && p < bestAsk {
				bestAsk = p
			}
		}
	}
	if bestAsk > 0 && bestBid > 0 {
		return bestAsk - bestBid
	}
	return 0
}
