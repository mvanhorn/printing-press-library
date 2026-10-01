// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newNovelLedgerCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Search, audit and query transactions in the local budget mirror",
		Long: `Read-only views over the transactions in your local budget mirror (refresh it
with 'mirror pull'): keyword search, uncategorized transactions, likely duplicates,
and raw read-only SQL. Amounts are integer minor units (cents).`,
		Example: strings.Trim(`
  actual-budget-pp-cli ledger search kroger --month 2026-09
  actual-budget-pp-cli ledger uncategorized --account "Visa Card"
  actual-budget-pp-cli ledger duplicates --days 3 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelLedgerDuplicatesCmd(flags))
	cmd.AddCommand(newNovelLedgerSqlCmd(flags))
	cmd.AddCommand(newLedgerSearchCmd(flags))
	cmd.AddCommand(newLedgerUncategorizedCmd(flags))
	return cmd
}

// ledgerResolveAccount maps an --account value (id or name) to an account id.
// Empty input means "all accounts".
func ledgerResolveAccount(ctx context.Context, h *mirrorHandle, v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", nil
	}
	id, err := h.Ledger.ResolveAccountID(ctx, v)
	return id, ledgerResolveErr(err, "accounts")
}

// ledgerResolveCategory maps a --category value (id or name) to a category id.
// Empty input means "all categories".
func ledgerResolveCategory(ctx context.Context, h *mirrorHandle, v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", nil
	}
	id, err := h.Ledger.ResolveCategoryID(ctx, v)
	return id, ledgerResolveErr(err, "categories")
}

// ledgerResolveErr maps name-resolution failures to typed exit codes: no
// match -> not found (3), several matches -> usage error (2, message lists
// the ids).
func ledgerResolveErr(err error, table string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return notFoundErr(fmt.Errorf("%w (list them with: actual-budget-pp-cli ledger sql \"SELECT id, name FROM %s WHERE tombstone = 0\")", err, table))
	case errors.Is(err, actual.ErrAmbiguousName):
		return usageErr(err)
	}
	return err
}

// ledgerDateRange parses optional --month or --from/--to into YYYYMMDD ints
// (0 = open-ended). Nothing set means no date filter.
func ledgerDateRange(month, from, to string) (int, int, error) {
	if month == "" && from == "" && to == "" {
		return 0, 0, nil
	}
	f, t, _, err := parseRange(month, from, to)
	return f, t, err
}

// ledgerTxnRow is the shared output row for transaction lists.
type ledgerTxnRow struct {
	ID            string `json:"id"`
	Date          string `json:"date"`
	Amount        int64  `json:"amount"`
	Account       string `json:"account"`
	Payee         string `json:"payee"`
	Category      string `json:"category"`
	Notes         string `json:"notes"`
	ImportedPayee string `json:"imported_payee"`
}

func ledgerTxnTable(rows []ledgerTxnRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{"date": r.Date, "amount": amt(r.Amount), "account": r.Account, "payee": r.Payee, "category": r.Category, "notes": r.Notes, "id": r.ID})
	}
	return items
}
