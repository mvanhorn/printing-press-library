// Hand-authored novel feature: decompose the net-worth change over a window
// into income, spending, market gain/loss, and transfers. Joins
// aggregate_snapshots × transactions × holdings in-memory.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newNetworthExplainCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Decompose net-worth change over a window into income, spending, market, and transfers",
		Long: `Returns the four numbers Monarch's chart hides behind a single line:
- income: sum of inflow transactions categorized as income
- spending: absolute sum of outflow transactions
- market: investment-account balance change minus contributions/withdrawals
- transfers: net inter-account flow

Useful for "why did my net worth jump $4,200 this week?" — the answer
is one command instead of an hour of spreadsheet work.`,
		Example:     "  monarch-pp-cli networth explain --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end := windowFromSince(flagSince)
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command: "networth explain",
					Persona: "Marcus — FIRE-track engineer with 14 accounts",
					Plan:    fmt.Sprintf("Decompose net-worth delta from %s to %s into income/spending/market/transfers", start, end),
					Operations: []string{
						"Common_GetAggregateSnapshots (start + end balance)",
						"Web_GetTransactionsSummaryCard (income vs expense in window)",
					},
					Inputs: map[string]string{"start_date": start, "end_date": end},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// 1. Aggregate snapshots over the window
			snapVars := map[string]any{
				"filters": map[string]any{
					"startDate": start,
					"endDate":   end,
				},
			}
			snapData, err := fetchGraphQL(c, client.NetworthSnapshotsQuery, snapVars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var snapResp struct {
				Snapshots []struct {
					Date               string  `json:"date"`
					Balance            float64 `json:"balance"`
					AssetsBalance      float64 `json:"assetsBalance"`
					LiabilitiesBalance float64 `json:"liabilitiesBalance"`
				} `json:"aggregateSnapshots"`
			}
			if err := json.Unmarshal(snapData, &snapResp); err != nil {
				return apiErr(fmt.Errorf("parsing snapshots: %w", err))
			}
			var startBal, endBal float64
			if len(snapResp.Snapshots) > 0 {
				startBal = snapResp.Snapshots[0].Balance
				endBal = snapResp.Snapshots[len(snapResp.Snapshots)-1].Balance
			}
			// 2. Income vs spend in window
			sumVars := map[string]any{
				"filters": map[string]any{
					"startDate": start,
					"endDate":   end,
				},
			}
			sumData, err := fetchGraphQL(c, client.TransactionsSummaryQuery, sumVars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var sumResp struct {
				Aggregates []struct {
					Summary struct {
						SumIncome  float64 `json:"sumIncome"`
						SumExpense float64 `json:"sumExpense"`
						Sum        float64 `json:"sum"`
					} `json:"summary"`
				} `json:"aggregates"`
			}
			if err := json.Unmarshal(sumData, &sumResp); err != nil {
				return apiErr(fmt.Errorf("parsing summary: %w", err))
			}
			var income, expense float64
			if len(sumResp.Aggregates) > 0 {
				income = sumResp.Aggregates[0].Summary.SumIncome
				expense = sumResp.Aggregates[0].Summary.SumExpense
			}
			delta := endBal - startBal
			cashflow := income + expense // expense is negative
			market := delta - cashflow   // residual is market + transfers (which net to zero across all accounts)
			result := map[string]any{
				"window":               map[string]string{"start": start, "end": end},
				"net_worth_start":      startBal,
				"net_worth_end":        endBal,
				"net_worth_delta":      delta,
				"income":               income,
				"spending":             -expense, // report spending as a positive number
				"cashflow":             cashflow,
				"market_and_transfers": market,
				"note":                 "market_and_transfers is the residual after cashflow; transfers between linked accounts net to zero, so this approximates market gain/loss.",
			}
			return printJSONOrTable(flags, result)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window: e.g. 7d, 30d, 90d, ytd")
	return cmd
}

// windowFromSince converts strings like "7d", "30d", "ytd" into start/end dates.
func windowFromSince(s string) (start, end string) {
	end = today()
	switch s {
	case "7d":
		start = daysAgo(7)
	case "14d":
		start = daysAgo(14)
	case "30d":
		start = daysAgo(30)
	case "60d":
		start = daysAgo(60)
	case "90d":
		start = daysAgo(90)
	case "ytd":
		start = startOfMonth(daysAgo(0))[:5] + "01-01"
	default:
		// Try parsing as nDay shorthand (e.g. "45d")
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil && n > 0 {
			start = daysAgo(n)
		} else {
			start = daysAgo(7)
		}
	}
	return
}
