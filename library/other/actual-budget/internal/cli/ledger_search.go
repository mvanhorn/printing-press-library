// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newLedgerSearchCmd(flags *rootFlags) *cobra.Command {
	var account, month, from, to string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find transactions by payee, imported text, notes, category or account name",
		Long: `Use this command for keyword lookup of transactions by text. Do NOT use it for arbitrary SQL; use 'ledger sql' instead.

Matches are case-insensitive substrings over the payee name (merged payees
resolved), the bank's imported payee text, notes, category name and account
name. Several words must all match. Deleted transactions and split parents are
skipped (split children are searched). Amounts are integer minor units (cents);
results are newest first.`,
		Example: strings.Trim(`
  actual-budget-pp-cli ledger search kroger
  actual-budget-pp-cli ledger search "deli lunch" --from 2026-08-01 --to 2026-08-31
  actual-budget-pp-cli ledger search amazon --account "Visa Card" --limit 10 --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "query=kroger"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if done, err := mirrorPreflight(cmd, flags, "ledger search"); done {
				return err
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(errors.New("a search query is required, e.g. 'ledger search kroger'"))
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
				txns, err := h.Ledger.SearchTxns(ctx, query, actual.TxnFilter{From: f, To: t, AccountID: acctID}, limit)
				if err != nil {
					return err
				}
				rows := ledgerRowsFromTxns(txns)
				return emitRows(cmd, flags, rows, func() error {
					if len(rows) == 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "No transactions match %q.\n", query)
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

func ledgerRowsFromTxns(txns []actual.Txn) []ledgerTxnRow {
	rows := make([]ledgerTxnRow, 0, len(txns))
	for _, t := range txns {
		rows = append(rows, ledgerTxnRow{ID: t.ID, Date: t.Date, Amount: t.Amount, Account: t.Account, Payee: t.Payee,
			Category: t.Category, Notes: t.Notes, ImportedPayee: t.ImportedDescription})
	}
	return rows
}
