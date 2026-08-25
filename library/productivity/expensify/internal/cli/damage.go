// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `damage` — single-glance summary of a month's expense status.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newDamageCmd(flags *rootFlags) *cobra.Command {
	var monthFlag, policyID string
	cmd := &cobra.Command{
		Use:   "damage",
		Short: "Single-glance summary: expensed, pending, approved, paid for a month",
		Example: `  expensify-pp-cli damage
  expensify-pp-cli damage --month previous
  expensify-pp-cli damage --month 2026-02 --policy ABC123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			month := resolveDamageMonth(monthFlag)
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			bd, err := st.Damage(month, policyID)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"month":            month,
					"policy":           policyID,
					"expensed":         bd.Expensed,
					"expensed_count":   bd.ExpensedCount,
					"pending_approval": bd.PendingApproval,
					"pending_count":    bd.PendingCount,
					"approved":         bd.Approved,
					"approved_count":   bd.ApprovedCount,
					"paid":             bd.Paid,
					"paid_count":       bd.PaidCount,
					"missing_receipts": bd.MissingReceipts,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Damage for %s", month)
			if policyID != "" {
				fmt.Fprintf(w, " (policy %s)", policyID)
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  Expensed:          %s (%d items)\n", money(bd.Expensed), bd.ExpensedCount)
			fmt.Fprintf(w, "  Pending approval:  %s (%d items)\n", money(bd.PendingApproval), bd.PendingCount)
			fmt.Fprintf(w, "  Approved:          %s (%d items)\n", money(bd.Approved), bd.ApprovedCount)
			fmt.Fprintf(w, "  Paid:              %s (%d items)\n", money(bd.Paid), bd.PaidCount)
			fmt.Fprintf(w, "  Missing receipts:  %d\n", bd.MissingReceipts)
			return nil
		},
	}
	cmd.Flags().StringVar(&monthFlag, "month", "current", "current | previous | YYYY-MM")
	cmd.Flags().StringVar(&policyID, "policy", "", "Filter to a single policy/workspace ID")
	return cmd
}

func resolveDamageMonth(flag string) string {
	now := time.Now()
	switch strings.ToLower(flag) {
	case "", "current":
		return now.Format("2006-01")
	case "previous", "last":
		return now.AddDate(0, -1, 0).Format("2006-01")
	default:
		// If it looks like YYYY-MM, pass through.
		if len(flag) == 7 && flag[4] == '-' {
			return flag
		}
		return flag
	}
}

func money(cents int64) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100)
}
