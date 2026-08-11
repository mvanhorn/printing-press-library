// Copyright 2026 dev-abhirup-sc and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented novel command. generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelPortfolioSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "summary",
		Short:       "See your total invested value, current value, overall P&L, and day change across all US stock holdings.",
		Example:     "  indmoney-pp-cli portfolio summary --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio summary")
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
					Summary struct {
						CurrentValue        float64 `json:"current_value"`
						DayChange           float64 `json:"day_change"`
						DayChangePercentage float64 `json:"day_change_percentage"`
						InvestedValue       float64 `json:"invested_value"`
						Pnl                 float64 `json:"pnl"`
						PercentageReturn    float64 `json:"percentage_return"`
						HoldingCount        int     `json:"holding_count"`
						RealisedReturns     float64 `json:"realised_returns"`
					} `json:"summary"`
				} `json:"demat_summary"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing portfolio response: %w", err)
			}
			s := resp.DematSummary.Summary

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), s, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Portfolio Summary\n")
			fmt.Fprintf(cmd.OutOrStdout(), "═══════════════════════════════════════════\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Current Value:    $%-12.2f\n", s.CurrentValue)
			fmt.Fprintf(cmd.OutOrStdout(), "  Invested Value:   $%-12.2f\n", s.InvestedValue)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total P&L:        $%-12.2f  (%.2f%%)\n", s.Pnl, s.PercentageReturn)
			fmt.Fprintf(cmd.OutOrStdout(), "  Day Change:       $%-12.2f  (%.2f%%)\n", s.DayChange, s.DayChangePercentage)
			fmt.Fprintf(cmd.OutOrStdout(), "  Holdings:         %d stocks\n", s.HoldingCount)
			fmt.Fprintf(cmd.OutOrStdout(), "  Realised Returns: $%.2f\n", s.RealisedReturns)
			return nil
		},
	}
	return cmd
}
