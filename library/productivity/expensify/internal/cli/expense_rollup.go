// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense rollup` — local SQL aggregation by category / tag / merchant.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newExpenseRollupCmd(flags *rootFlags) *cobra.Command {
	var month, by, policyID string
	cmd := &cobra.Command{
		Use:   "rollup",
		Short: "Aggregate expenses by category, tag, or merchant for a month",
		Example: `  expensify-pp-cli expense rollup
  expensify-pp-cli expense rollup --month 2026-03 --by merchant`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			rows, err := st.Rollup(month, by, policyID)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"report": "rollup",
					"month":  month,
					"by":     by,
					"rows":   rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "rollup for %s by %s: no expenses.\n", month, by)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rollup for %s by %s:\n", month, by)
			tableRows := make([][]string, 0, len(rows))
			var grand int64
			for _, r := range rows {
				grand += r.Total
				tableRows = append(tableRows, []string{
					r.Key,
					fmt.Sprintf("%d", r.Count),
					fmt.Sprintf("%.2f", float64(r.Total)/100),
				})
			}
			header := []string{"CATEGORY", "COUNT", "TOTAL"}
			if by == "tag" {
				header[0] = "TAG"
			} else if by == "merchant" {
				header[0] = "MERCHANT"
			}
			if err := flags.printTable(cmd, header, tableRows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %.2f across %d rows (month %s)\n", float64(grand)/100, len(rows), month)
			return nil
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "YYYY-MM (default: current month)")
	cmd.Flags().StringVar(&by, "by", "category", "Group by: category | tag | merchant")
	cmd.Flags().StringVar(&policyID, "policy", "", "Filter to a single policy/workspace ID")
	return cmd
}
