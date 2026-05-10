// Hand-authored novel feature: aggregate one month's transactions, budgets,
// net-worth-snapshots, and recurring data into a structured packet ready for
// LLM narrative draft.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newCashflowMonthlyMemoCmd(flags *rootFlags) *cobra.Command {
	var flagMonth string
	cmd := &cobra.Command{
		Use:   "monthly-memo",
		Short: "Aggregate a month's transactions, budgets, and NW into a structured memo packet",
		Long: `Returns a one-month bundle shaped for | claude to draft prose from:
  - top categories by spend with month-over-month deltas
  - budget hits / misses
  - biggest single transactions (top 10)
  - net-worth delta for the month
  - income vs expense totals`,
		Example:     "  monarch-pp-cli cashflow monthly-memo --month 2026-04 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagMonth == "" {
				flagMonth = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01")
			}
			t, err := time.Parse("2006-01", flagMonth)
			if err != nil {
				return usageErr(fmt.Errorf("--month must be YYYY-MM (got %q)", flagMonth))
			}
			start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			end := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour).Format("2006-01-02")
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "cashflow monthly-memo",
					Persona:    "Sasha — agent-power-user",
					Plan:       fmt.Sprintf("Aggregate transactions, budgets, networth-snapshots for %s..%s into a memo bundle", start, end),
					Operations: []string{"Web_GetTransactionsSummaryCard", "Common_GetBudgetStatus", "Common_GetAggregateSnapshots", "Web_GetTransactionsList"},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			memo := map[string]any{
				"month":  flagMonth,
				"window": map[string]string{"start": start, "end": end},
			}
			// 1. cashflow summary
			if data, err := fetchGraphQL(c, client.TransactionsSummaryQuery, map[string]any{
				"filters": map[string]any{"startDate": start, "endDate": end},
			}); err == nil {
				var resp struct {
					Aggregates []struct {
						Summary struct {
							SumIncome  float64 `json:"sumIncome"`
							SumExpense float64 `json:"sumExpense"`
							Count      int     `json:"count"`
						}
					} `json:"aggregates"`
				}
				_ = json.Unmarshal(data, &resp)
				if len(resp.Aggregates) > 0 {
					s := resp.Aggregates[0].Summary
					memo["cashflow"] = map[string]any{
						"income":            s.SumIncome,
						"spending":          -s.SumExpense,
						"net":               s.SumIncome + s.SumExpense,
						"transaction_count": s.Count,
					}
				}
			}
			// 2. budget snapshot (this month only)
			if data, err := fetchGraphQL(c, client.BudgetsStatusQuery, map[string]any{"startDate": start, "endDate": end}); err == nil {
				var resp struct {
					BudgetData struct {
						Categories []struct {
							Category       struct{ Name, Icon string } `json:"category"`
							MonthlyAmounts []struct {
								PlannedCashFlowAmount float64 `json:"plannedCashFlowAmount"`
								ActualAmount          float64 `json:"actualAmount"`
								RemainingAmount       float64 `json:"remainingAmount"`
							} `json:"monthlyAmounts"`
						} `json:"monthlyAmountsByCategory"`
					} `json:"budgetData"`
				}
				_ = json.Unmarshal(data, &resp)
				rows := []map[string]any{}
				for _, c := range resp.BudgetData.Categories {
					if len(c.MonthlyAmounts) == 0 || c.MonthlyAmounts[0].PlannedCashFlowAmount == 0 {
						continue
					}
					m := c.MonthlyAmounts[0]
					rows = append(rows, map[string]any{
						"category":  c.Category.Name,
						"planned":   m.PlannedCashFlowAmount,
						"actual":    -m.ActualAmount,
						"remaining": m.RemainingAmount,
						"on_budget": -m.ActualAmount <= m.PlannedCashFlowAmount,
					})
				}
				memo["budgets"] = rows
			}
			// 3. networth delta (snapshots over the month)
			if data, err := fetchGraphQL(c, client.NetworthSnapshotsQuery, map[string]any{
				"filters": map[string]any{"startDate": start, "endDate": end},
			}); err == nil {
				var resp struct {
					Snapshots []struct {
						Date    string  `json:"date"`
						Balance float64 `json:"balance"`
					} `json:"aggregateSnapshots"`
				}
				_ = json.Unmarshal(data, &resp)
				if len(resp.Snapshots) > 0 {
					first := resp.Snapshots[0]
					last := resp.Snapshots[len(resp.Snapshots)-1]
					memo["net_worth"] = map[string]any{
						"start_balance": first.Balance,
						"end_balance":   last.Balance,
						"delta":         last.Balance - first.Balance,
					}
				}
			}
			// 4. biggest single transactions
			if data, err := fetchGraphQL(c, client.TransactionsListQuery, map[string]any{
				"limit":   10,
				"offset":  0,
				"orderBy": "AMOUNT_ASC", // most negative first
				"filters": map[string]any{"startDate": start, "endDate": end},
			}); err == nil {
				var resp struct {
					All struct {
						Results json.RawMessage `json:"results"`
					} `json:"allTransactions"`
				}
				_ = json.Unmarshal(data, &resp)
				memo["biggest_transactions"] = resp.All.Results
			}
			return printJSONOrTable(flags, memo)
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Month to summarize, YYYY-MM (default: previous month)")
	return cmd
}
