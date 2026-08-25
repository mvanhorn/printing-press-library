// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/types"
	"github.com/spf13/cobra"
)

type duplicateExpense struct {
	ExpenseId       string `json:"expense_id"`
	ReportId        string `json:"report_id"`
	TransactionDate string `json:"transaction_date"`
}

// parseDateOnly parses a Concur transaction-date string into a time.Time,
// ignoring any time-of-day component. Reconstructed 2026-08-19 after
// trips_reconcile.go (its original home, sharing this helper across two
// novel commands) was removed as part of dropping the available-expenses-
// dependent commands; this file is now parseDateOnly's only caller. Concur's
// various expense/report date fields have been observed in both a bare
// YYYY-MM-DD form and a full RFC3339 timestamp, so both are tried.
func parseDateOnly(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	if idx := strings.IndexByte(s, 'T'); idx > 0 {
		if t, err := time.Parse("2006-01-02", s[:idx]); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
}

type duplicateGroup struct {
	Vendor   string             `json:"vendor"`
	Amount   float64            `json:"amount"`
	Expenses []duplicateExpense `json:"expenses"`
}

type scanDuplicatesResult struct {
	DuplicateGroups []duplicateGroup `json:"duplicate_groups"`
}

func newNovelExpensesScanDuplicatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scan-duplicates",
		Short:       "Find potential double-entered charges across all of your synced expenses.",
		Example:     "  concur-pp-cli expenses scan-duplicates --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("concur-pp-cli")
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'concur-pp-cli sync --resources expenses' first to populate the local database.", err)
			}
			defer db.Close()

			// hintIfUnsynced already writes a non-fatal "run sync first" hint to
			// stderr when the store is empty; an empty store legitimately means
			// zero duplicates, not an error, so we fall through and return that.
			if !hintIfUnsynced(cmd, db, "expenses") {
				hintIfStale(cmd, db, "expenses", flags.maxAge)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM "expenses"`)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			defer rows.Close()

			var expenses []types.Expense
			rawDatas := make(map[string][]byte)

			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					return err
				}
				var exp types.Expense
				if err := json.Unmarshal(data, &exp); err != nil {
					return err
				}
				expenses = append(expenses, exp)
				rawDatas[exp.ExpenseId] = data
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(expenses) == 0 {
				// Zero synced expenses means zero duplicates -- a valid empty
				// result. hintIfUnsynced above already told the user to sync.
				return printJSONFiltered(cmd.OutOrStdout(), scanDuplicatesResult{DuplicateGroups: nil}, flags)
			}

			// Group by (vendorDescription case-insensitive, transactionAmount exact match)
			type groupKey struct {
				vendor string
				amount string
			}
			groups := make(map[groupKey][]types.Expense)
			for _, exp := range expenses {
				k := groupKey{
					vendor: strings.ToLower(strings.TrimSpace(exp.VendorDescription)),
					amount: fmt.Sprintf("%.2f", exp.TransactionAmount),
				}
				groups[k] = append(groups[k], exp)
			}

			var dupGroups []duplicateGroup
			for k, group := range groups {
				if len(group) < 2 {
					continue
				}

				// Find duplicates within 3 days of each other
				var dupInGroup []duplicateExpense
				for i := 0; i < len(group); i++ {
					isDup := false
					for j := 0; j < len(group); j++ {
						if i == j {
							continue
						}
						d1, err1 := parseDateOnly(group[i].TransactionDate)
						d2, err2 := parseDateOnly(group[j].TransactionDate)
						if err1 == nil && err2 == nil {
							diff := d2.Sub(d1)
							if diff < 0 {
								diff = -diff
							}
							if diff <= 3*24*time.Hour {
								isDup = true
								break
							}
						}
					}
					if isDup {
						expenseId := group[i].ExpenseId
						reportId := getReportIdOfExpense(rawDatas[expenseId])
						dupInGroup = append(dupInGroup, duplicateExpense{
							ExpenseId:       expenseId,
							ReportId:        reportId,
							TransactionDate: group[i].TransactionDate,
						})
					}
				}

				if len(dupInGroup) > 0 {
					var amountVal float64
					if _, err := fmt.Sscanf(k.amount, "%f", &amountVal); err != nil {
						// The grouping key's amount didn't parse as a number --
						// skip rather than report a silently-wrong $0.00, which
						// would misrepresent a real potential duplicate charge
						// as having no amount. This should not happen in
						// practice since k.amount is itself derived from a
						// numeric field upstream, but a defensive skip is
						// safer than fabricated data if it ever does.
						continue
					}
					dupGroups = append(dupGroups, duplicateGroup{
						Vendor:   group[0].VendorDescription, // Original case
						Amount:   amountVal,
						Expenses: dupInGroup,
					})
				}
			}

			result := scanDuplicatesResult{
				DuplicateGroups: dupGroups,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func getReportIdOfExpense(data []byte) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err == nil {
		if val, ok := m["reportId"].(string); ok {
			return val
		}
		if val, ok := m["report_id"].(string); ok {
			return val
		}
	}
	return ""
}
