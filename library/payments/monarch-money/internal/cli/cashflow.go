package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

const cashflowQuery = `query Web_GetCashFlowPage($filters: TransactionFilterInput) {
  summary: aggregates(filters: $filters, fillEmptyValues: true) {
    summary {
      sumIncome
      sumExpense
      savings
      savingsRate
    }
  }
}`

func newCashflowCmd() *cobra.Command {
	var jsonOut bool
	var startDate, endDate string
	cmd := &cobra.Command{
		Use:   "cashflow",
		Short: "Summarize income, expenses, net savings, and savings rate for a date range.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if startDate == "" && endDate == "" {
				now := time.Now()
				startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
				endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
			} else if startDate == "" || endDate == "" {
				return fmt.Errorf("--start and --end must be provided together")
			}
			filters := map[string]any{"search": "", "categories": []string{}, "accounts": []string{}, "tags": []string{}, "startDate": startDate, "endDate": endDate}
			data, err := graphql("Web_GetCashFlowPage", cashflowQuery, map[string]any{"filters": filters})
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			aggs := asSlice(data["summary"])
			if len(aggs) == 0 {
				return fmt.Errorf("no cashflow summary returned")
			}
			summary := asMap(asMap(aggs[0])["summary"])
			rows := [][]string{{
				startDate + " → " + endDate,
				money(summary["sumIncome"]),
				money(summary["sumExpense"]),
				money(summary["savings"]),
				fmt.Sprintf("%.1f%%", num(summary["savingsRate"])*100),
			}}
			return table([]string{"Period", "Income", "Expenses", "Savings", "Savings Rate"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	cmd.Flags().StringVar(&startDate, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end", "", "End date YYYY-MM-DD")
	return cmd
}
