// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newNovelLedgerDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	var account, month, from, to string
	var minConfidence float64

	cmd := &cobra.Command{
		Use:   "duplicates",
		Short: "Catch transactions that were imported or entered twice, ranked by confidence.",
		Long: `Use this command to find transactions that were imported or entered twice. Do NOT use this command to find duplicate payee names; use 'audit payees' instead.

A pair is flagged when both transactions are in the same account, have the same
amount, are dated within --days of each other, and share a payee (same payee
after merges, the same normalized bank text, or payee names at least 80%
similar). Deleted rows, split parents and children, transfers and starting
balances are never paired. Confidence (0..1) rises with payee evidence and date
closeness; output is sorted by confidence, highest first. Amounts are integer
minor units (cents).`,
		Example: strings.Trim(`
  actual-budget-pp-cli ledger duplicates --days 3 --agent
  actual-budget-pp-cli ledger duplicates --account "Visa Card" --month 2026-09
  actual-budget-pp-cli ledger duplicates --days 0 --min-confidence 0.9 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "ledger duplicates"); done {
				return err
			}
			if days < 0 {
				return usageErr(errors.New("--days must be >= 0"))
			}
			if math.IsNaN(minConfidence) || minConfidence < 0 || minConfidence > 1 {
				return usageErr(errors.New("--min-confidence must be between 0 and 1"))
			}
			if err := checkLimit(limit); err != nil {
				return err
			}
			f, t, err := ledgerDateRange(month, from, to)
			if err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.DuplicatePair, 0), func(ctx context.Context, h *mirrorHandle) error {
				acctID, err := ledgerResolveAccount(ctx, h, account)
				if err != nil {
					return err
				}
				pairs, err := h.Ledger.FindDuplicates(ctx, actual.DuplicateOptions{Days: days, From: f, To: t, AccountID: acctID, MinConfidence: minConfidence})
				if err != nil {
					return err
				}
				if limit > 0 && len(pairs) > limit {
					pairs = pairs[:limit]
				}
				return emitRows(cmd, flags, pairs, func() error {
					if len(pairs) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No likely duplicates found.")
						return nil
					}
					items := make([]map[string]any, 0, len(pairs))
					for _, p := range pairs {
						items = append(items, map[string]any{"confidence": fmt.Sprintf("%.2f", p.Confidence), "account": p.Account, "amount": amt(p.Amount),
							"payee": p.Payee, "a_date": p.ADate, "b_date": p.BDate, "a_id": p.AID, "b_id": p.BID})
					}
					return printAutoTable(cmd.OutOrStdout(), items)
				})
			})
		},
	}
	cmd.Flags().IntVar(&days, "days", 3, "Maximum days between the two transactions")
	cmd.Flags().StringVar(&account, "account", "", "Only this account (name or id)")
	cmd.Flags().StringVar(&month, "month", "", "Only pairs touching this month (YYYY-MM)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest date, inclusive (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Latest date, inclusive (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&minConfidence, "min-confidence", 0.6, "Drop pairs scored below this confidence (0..1)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum pairs to return (0 = no limit)")
	return cmd
}
