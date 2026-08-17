// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense missing-receipts` — surface expenses with no attached receipt.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newExpenseMissingReceiptsCmd(flags *rootFlags) *cobra.Command {
	var since, policyID string
	cmd := &cobra.Command{
		Use:   "missing-receipts",
		Short: "List expenses in the local store that lack a receipt",
		Example: `  expensify-pp-cli expense missing-receipts
  expensify-pp-cli expense missing-receipts --since 2026-01-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			filters := map[string]string{}
			if since != "" {
				filters["since"] = since
			}
			if policyID != "" {
				filters["policy_id"] = policyID
			}
			rows, err := st.MissingReceipts(filters)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"report":   "missing-receipts",
					"count":    len(rows),
					"expenses": rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "missing-receipts: clean — no expenses lack a receipt.")
				return nil
			}
			tRows := make([][]string, 0, len(rows))
			var total int64
			for _, e := range rows {
				total += e.Amount
				tRows = append(tRows, []string{
					e.TransactionID,
					e.Date,
					truncate(e.Merchant, 30),
					fmt.Sprintf("%.2f", float64(e.Amount)/100),
				})
			}
			if err := flags.printTable(cmd, []string{"TX_ID", "DATE", "MERCHANT", "AMOUNT"}, tRows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nmissing-receipts: %d expenses totaling $%.2f need a receipt attached.\n", len(rows), float64(total)/100)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only check expenses on/after YYYY-MM-DD")
	cmd.Flags().StringVar(&policyID, "policy", "", "Filter to a single policy/workspace ID")
	return cmd
}
