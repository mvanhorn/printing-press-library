// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// runwaySummary is the computed cash position and burn over a window.
type runwaySummary struct {
	TotalBalance     float64  `json:"total_balance"`
	WindowWeeks      int      `json:"window_weeks"`
	Inflow           float64  `json:"inflow"`
	Outflow          float64  `json:"outflow"`
	NetFlow          float64  `json:"net_flow"`
	AvgWeeklyBurn    float64  `json:"avg_weekly_burn"`
	RunwayWeeks      *float64 `json:"runway_weeks"`
	CashFlowPositive bool     `json:"cash_flow_positive"`
}

func newCashRunwayCmd(flags *rootFlags) *cobra.Command {
	var flagAccountID string
	var flagWeeks int

	cmd := &cobra.Command{
		Use:         "cash-runway",
		Short:       "Estimate weeks of runway from real balance and recent net burn",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  mercury-pp-cli cash-runway
  mercury-pp-cli cash-runway --weeks 12 --account-id 550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagWeeks <= 0 {
				return fmt.Errorf("--weeks must be greater than 0")
			}

			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			accounts, err := fetchAccounts(ctx, c, flags, flagAccountID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			now := time.Now().UTC()
			start := now.AddDate(0, 0, -flagWeeks*7).Format("2006-01-02")
			end := now.Format("2006-01-02")

			var balance float64
			var txns []mercuryTxn
			for _, a := range accounts {
				balance += a.CurrentBalance
				accountTxns, err := fetchAccountTxns(ctx, c, flags, a.ID, start, end)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				txns = append(txns, accountTxns...)
			}

			summary := summarizeRunway(balance, txns, flagWeeks)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				runway := "n/a (cash-flow positive)"
				if summary.RunwayWeeks != nil {
					runway = fmt.Sprintf("%.1f weeks", *summary.RunwayWeeks)
				}
				headers := []string{"BALANCE", "NET/WK", "BURN/WK", "RUNWAY"}
				rows := [][]string{{
					fmt.Sprintf("%.2f", summary.TotalBalance),
					fmt.Sprintf("%.2f", summary.NetFlow/float64(flagWeeks)),
					fmt.Sprintf("%.2f", summary.AvgWeeklyBurn),
					runway,
				}}
				return flags.printTable(cmd, headers, rows)
			}

			payload := map[string]any{
				"accounts": len(accounts),
				"window":   map[string]string{"start": start, "end": end},
				"summary":  summary,
			}
			return flags.printJSON(cmd, payload)
		},
	}

	cmd.Flags().StringVar(&flagAccountID, "account-id", "", "Limit to a single account ID (default: all accounts).")
	cmd.Flags().IntVar(&flagWeeks, "weeks", 8, "Number of weeks of transactions to average burn over.")
	return cmd
}

// summarizeRunway sums signed transaction amounts into inflow/outflow, derives
// average weekly net burn, and projects runway weeks from the current balance.
// A non-negative net over the window is reported as cash-flow positive with no
// finite runway.
func summarizeRunway(balance float64, txns []mercuryTxn, weeks int) runwaySummary {
	var inflow, outflow float64
	for _, t := range txns {
		amt := float64(t.Amount)
		if amt >= 0 {
			inflow += amt
		} else {
			outflow += -amt
		}
	}
	net := inflow - outflow
	summary := runwaySummary{
		TotalBalance: balance,
		WindowWeeks:  weeks,
		Inflow:       inflow,
		Outflow:      outflow,
		NetFlow:      net,
	}
	if weeks <= 0 {
		return summary
	}
	weeklyNet := net / float64(weeks)
	if weeklyNet < 0 {
		// burn is -weeklyNet, so it is strictly positive in this branch.
		burn := -weeklyNet
		summary.AvgWeeklyBurn = burn
		runway := balance / burn
		summary.RunwayWeeks = &runway
	} else {
		summary.CashFlowPositive = true
	}
	return summary
}
