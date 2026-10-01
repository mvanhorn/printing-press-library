// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newLedgerUncategorizedCmd(flags *rootFlags) *cobra.Command {
	var account, month, from, to string
	var limit int
	cmd := &cobra.Command{
		Use:   "uncategorized",
		Short: "List on-budget transactions that still need a category",
		Long: `Lists live transactions with no category, matching Actual's own "uncategorized"
view: on-budget accounts only, transfers, split parents and starting balances
excluded. Amounts are integer minor units (cents); newest first. Pair with
'categorize suggest' to fill them in.`,
		Example: strings.Trim(`
  actual-budget-pp-cli ledger uncategorized
  actual-budget-pp-cli ledger uncategorized --account "Visa Card" --month 2026-09 --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "ledger uncategorized"); done {
				return err
			}
			if err := checkLimit(limit); err != nil {
				return err
			}
			f, t, err := ledgerDateRange(month, from, to)
			if err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]ledgerTxnRow, 0), func(ctx context.Context, h *mirrorHandle) error {
				acctID, err := ledgerResolveAccount(ctx, h, account)
				if err != nil {
					return err
				}
				txns, err := h.Ledger.Txns(ctx, actual.TxnFilter{From: f, To: t, AccountID: acctID, Uncategorized: true, Limit: limit})
				if err != nil {
					return err
				}
				rows := ledgerRowsFromTxns(txns)
				return emitRows(cmd, flags, rows, func() error {
					if len(rows) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "Every on-budget transaction has a category.")
						return nil
					}
					return printAutoTable(cmd.OutOrStdout(), ledgerTxnTable(rows))
				})
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Only this account (name or id)")
	cmd.Flags().StringVar(&month, "month", "", "Only this month (YYYY-MM)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest date, inclusive (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Latest date, inclusive (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return (0 = no limit)")
	return cmd
}
