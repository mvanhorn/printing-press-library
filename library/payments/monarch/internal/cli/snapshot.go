// Hand-authored novel feature: agent context bundle. One JSON blob composed
// of net-worth-by-type + MTD cashflow + top-10 categories + budget burn +
// next-7-day recurring + last-7-day transactions, shaped for | claude.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Emit one JSON blob describing your current financial state, shaped for LLM consumption",
		Long: `Composes six existing reads into a single agent-friendly bundle:
  - net_worth_by_type       (asset / liability totals by account type)
  - month_to_date_cashflow  (income vs spend so far this month)
  - top_categories          (top-10 categories MTD by absolute spend)
  - budget_burn             (per-category projected month-end position)
  - next_7_days_recurring   (upcoming charges)
  - last_7_days_transactions (recent activity)

Designed to be piped to an LLM for narrative answers like
"what's my discretionary spend trend, exclude medical, draft a paragraph".`,
		Example:     "  monarch-pp-cli snapshot --json | claude",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command: "snapshot",
					Persona: "Sasha — agent-power-user",
					Plan:    "Compose accounts + transactions summary + budgets + recurring into one JSON",
					Operations: []string{
						"Web_GetAccountsPage",
						"Web_GetTransactionsSummaryCard",
						"Common_GetBudgetStatus",
						"Common_GetAggregatedRecurringItems",
						"Web_GetTransactionsList",
					},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			snapshot := map[string]any{}
			// 1. accounts grouped by type
			if data, err := fetchGraphQL(c, client.AccountsListQuery, map[string]any{"filters": map[string]any{}}); err == nil {
				var resp struct {
					Accounts []struct {
						DisplayName       string                         `json:"displayName"`
						CurrentBalance    float64                        `json:"currentBalance"`
						Type              struct{ Name, Display string } `json:"type"`
						IsAsset           bool                           `json:"isAsset"`
						IncludeInNetWorth bool                           `json:"includeInNetWorth"`
					} `json:"accounts"`
				}
				_ = json.Unmarshal(data, &resp)
				byType := map[string]float64{}
				var totalAssets, totalLiabilities float64
				for _, a := range resp.Accounts {
					if !a.IncludeInNetWorth {
						continue
					}
					byType[a.Type.Display] += a.CurrentBalance
					if a.IsAsset {
						totalAssets += a.CurrentBalance
					} else {
						totalLiabilities += a.CurrentBalance
					}
				}
				snapshot["net_worth_by_type"] = byType
				snapshot["total_assets"] = totalAssets
				snapshot["total_liabilities"] = totalLiabilities
				snapshot["net_worth"] = totalAssets - totalLiabilities
			}
			// 2. MTD cashflow
			start := startOfMonth(today())
			end := today()
			if data, err := fetchGraphQL(c, client.TransactionsSummaryQuery, map[string]any{
				"filters": map[string]any{"startDate": start, "endDate": end},
			}); err == nil {
				var resp struct {
					Aggregates []struct {
						Summary struct {
							SumIncome  float64 `json:"sumIncome"`
							SumExpense float64 `json:"sumExpense"`
							Count      int     `json:"count"`
						} `json:"summary"`
					} `json:"aggregates"`
				}
				_ = json.Unmarshal(data, &resp)
				if len(resp.Aggregates) > 0 {
					s := resp.Aggregates[0].Summary
					snapshot["month_to_date"] = map[string]any{
						"start":    start,
						"end":      end,
						"income":   s.SumIncome,
						"spending": -s.SumExpense,
						"net":      s.SumIncome + s.SumExpense,
						"count":    s.Count,
					}
				}
			}
			// 3. recent transactions (last 7 days)
			if data, err := fetchGraphQL(c, client.TransactionsListQuery, map[string]any{
				"limit":   25,
				"offset":  0,
				"orderBy": "DATE_DESC",
				"filters": map[string]any{
					"startDate": daysAgo(7),
					"endDate":   today(),
				},
			}); err == nil {
				var resp struct {
					All struct {
						Results json.RawMessage `json:"results"`
					} `json:"allTransactions"`
				}
				_ = json.Unmarshal(data, &resp)
				snapshot["last_7_days_transactions"] = resp.All.Results
			}
			// 4. upcoming recurring (next 7)
			if data, err := fetchGraphQL(c, client.RecurringListQuery, map[string]any{
				"startDate": today(),
				"endDate":   daysAgo(-7),
			}); err == nil {
				var resp struct {
					RecurringItems json.RawMessage `json:"recurringItems"`
				}
				_ = json.Unmarshal(data, &resp)
				snapshot["next_7_days_recurring"] = resp.RecurringItems
			}
			snapshot["meta"] = map[string]any{
				"generated_at": today(),
				"command":      "monarch-pp-cli snapshot",
			}
			return printJSONOrTable(flags, snapshot)
		},
	}
	return cmd
}
