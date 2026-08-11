// Copyright 2026 dev-abhirup-sc and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented novel command. generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelIndicesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Live prices for Nasdaq, S&P 500, Dow Jones, and other US indices with market status.",
		Example:     "  indmoney-pp-cli indices list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "indices list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/catalog/indices", map[string]string{
				"segment":              "cash",
				"mkt-status-required":  "true",
				"require_live_feed":    "true",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var resp struct {
				Index []struct {
					IndKey      string  `json:"ind_key"`
					Name        string  `json:"name"`
					LivePrice   float64 `json:"live_price"`
					OneDChange  float64 `json:"oneD_change"`
					AbsChange   float64 `json:"absolute_change"`
				} `json:"index"`
				MarketStatusDetails struct {
					MarketStatus       string `json:"MarketStatus"`
					IsTradingDay       bool   `json:"IsTradingDay"`
					WhenItWillOpen     string `json:"WhenItWillOpen"`
					ExtendedHourStatus string `json:"ExtendedHourStatus"`
				} `json:"market_status_details"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing indices response: %w", err)
			}

			type index struct {
				Name       string  `json:"name"`
				Symbol     string  `json:"symbol"`
				LivePrice  float64 `json:"live_price"`
				DayChange  float64 `json:"day_change"`
				AbsChange  float64 `json:"absolute_change"`
			}

			indices := make([]index, 0, len(resp.Index))
			for _, idx := range resp.Index {
				indices = append(indices, index{
					Name: idx.Name, Symbol: idx.IndKey,
					LivePrice: idx.LivePrice, DayChange: idx.OneDChange,
					AbsChange: idx.AbsChange,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), indices, flags)
			}

			ms := resp.MarketStatusDetails
			fmt.Fprintf(cmd.OutOrStdout(), "Market: %s", ms.MarketStatus)
			if ms.ExtendedHourStatus != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", ms.ExtendedHourStatus)
			}
			if ms.MarketStatus == "close" && ms.WhenItWillOpen != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " — opens %s", ms.WhenItWillOpen)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout())

			if len(indices) == 0 {
				return nil
			}
			items := make([]map[string]any, len(indices))
			for i, idx := range indices {
				items[i] = map[string]any{
					"name": idx.Name, "price": idx.LivePrice,
					"change": fmt.Sprintf("%.2f (%.2f%%)", idx.AbsChange, idx.DayChange),
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	return cmd
}
