// Copyright 2026 dev-abhirup-sc and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented novel command. generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelPortfolioHoldingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "holdings",
		Short:       "List every stock you hold with quantity, avg price, current value, unrealised gains, and day P&L.",
		Example:     "  indmoney-pp-cli portfolio holdings --select name,quantity,overall_pnl --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio holdings")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GetWithHeaders(ctx, "/portfolio/equity/summary", map[string]string{"response_format": "json"}, map[string]string{"platform": "web"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var resp struct {
				DematSummary struct {
					AssetSummary struct {
						ScripDetails []struct {
							Metadata struct {
								Name      string  `json:"name"`
								Symbol    string  `json:"symbol"`
								Sector    string  `json:"sector"`
								LivePrice float64 `json:"live_price"`
								Slug      string  `json:"slug"`
							} `json:"metadata"`
							Holdings struct {
								Quantity       float64 `json:"quantity"`
								CurrentValue   float64 `json:"current_value"`
								AvgPrice       float64 `json:"avg_price"`
								DayPnl         float64 `json:"day_pnl"`
								OverallPnl     float64 `json:"overall_pnl"`
								OverallPnlPct  float64 `json:"overall_pnl_percentage"`
								InvestedAmount float64 `json:"invested_amount"`
								UnrealisedGains float64 `json:"unrealised_gains"`
							} `json:"holdings"`
						} `json:"scrip_details"`
					} `json:"asset_summary"`
				} `json:"demat_summary"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing portfolio response: %w", err)
			}

			type holding struct {
				Name          string  `json:"name"`
				Symbol        string  `json:"symbol"`
				Sector        string  `json:"sector"`
				Quantity      float64 `json:"quantity"`
				AvgPrice      float64 `json:"avg_price"`
				LivePrice     float64 `json:"live_price"`
				CurrentValue  float64 `json:"current_value"`
				InvestedAmount float64 `json:"invested_amount"`
				OverallPnl    float64 `json:"overall_pnl"`
				OverallPnlPct float64 `json:"overall_pnl_percentage"`
				DayPnl        float64 `json:"day_pnl"`
			}

			holdings := make([]holding, 0, len(resp.DematSummary.AssetSummary.ScripDetails))
			for _, s := range resp.DematSummary.AssetSummary.ScripDetails {
				if s.Holdings.Quantity == 0 {
					continue
				}
				holdings = append(holdings, holding{
					Name:           s.Metadata.Name,
					Symbol:         s.Metadata.Symbol,
					Sector:         s.Metadata.Sector,
					Quantity:       s.Holdings.Quantity,
					AvgPrice:       s.Holdings.AvgPrice,
					LivePrice:      s.Metadata.LivePrice,
					CurrentValue:   s.Holdings.CurrentValue,
					InvestedAmount: s.Holdings.InvestedAmount,
					OverallPnl:     s.Holdings.OverallPnl,
					OverallPnlPct:  s.Holdings.OverallPnlPct,
					DayPnl:         s.Holdings.DayPnl,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), holdings, flags)
			}

			if len(holdings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active holdings found.")
				return nil
			}
			items := make([]map[string]any, len(holdings))
			for i, h := range holdings {
				items[i] = map[string]any{
					"name": h.Name, "symbol": h.Symbol, "qty": h.Quantity,
					"avg_price": h.AvgPrice, "current_value": h.CurrentValue,
					"pnl": h.OverallPnl, "pnl_pct": fmt.Sprintf("%.1f%%", h.OverallPnlPct),
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	return cmd
}
