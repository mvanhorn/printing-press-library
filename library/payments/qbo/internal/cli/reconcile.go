// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/store"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type reconcileCandidate struct {
	Type        string  `json:"type"`
	ID          string  `json:"id"`
	RefNum      string  `json:"ref_number"`
	Party       string  `json:"party"`
	Date        string  `json:"date"`
	TotalAmount float64 `json:"total_amount"`
	Balance     float64 `json:"balance,omitempty"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var amountFlag float64
	var dateFlag string
	var daysFlag int

	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Find ledger candidates matching a bank transaction for reconciliation",
		Example: "  qbo-pp-cli reconcile --amount 1250.00 --date 2026-06-01\n  qbo-pp-cli reconcile --amount 45.90",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var targetDate time.Time
			if dateFlag != "" {
				t, err := time.Parse("2006-01-02", dateFlag)
				if err != nil {
					t, err = time.Parse(time.RFC3339, dateFlag)
					if err != nil {
						return fmt.Errorf("invalid date format: %q. Use YYYY-MM-DD", dateFlag)
					}
				}
				targetDate = t
			}

			var candidates []reconcileCandidate

			// List of entities to search
			searches := []struct {
				entityType string
				tableName  string
				partyPath  string // JSON path for party name
				balPath    string // JSON path for balance
			}{
				{"Invoice", "invoices", "CustomerRef.name", "Balance"},
				{"Bill", "bills", "VendorRef.name", "Balance"},
				{"Payment", "payments", "CustomerRef.name", ""},
				{"Purchase", "purchases", "EntityRef.name", ""},
			}

			for _, src := range searches {
				balCol := "NULL"
				if src.balPath != "" {
					balCol = fmt.Sprintf("CAST(json_extract(raw_json, '$.%s') AS REAL)", src.balPath)
				}

				query := fmt.Sprintf(`
					SELECT 
						id, 
						doc_number, 
						json_extract(raw_json, '$.%s') AS party,
						json_extract(raw_json, '$.TxnDate') AS txn_date,
						CAST(json_extract(raw_json, '$.TotalAmt') AS REAL) AS total_amt,
						%s AS balance
					FROM %s
					WHERE abs(CAST(json_extract(raw_json, '$.TotalAmt') AS REAL) - ?) < 0.01
				`, src.partyPath, balCol, src.tableName)

				var rows, err = s.DB().Query(query, amountFlag)
				if err != nil {
					// A missing table means this entity type has never been
					// synced; skip it gracefully. Any other error is a real
					// problem (WAL conflict, schema mismatch) that must surface.
					if strings.Contains(err.Error(), "no such table") {
						continue
					}
					return fmt.Errorf("querying %s: %w", src.entityType, err)
				}

				for rows.Next() {
					var c reconcileCandidate
					c.Type = src.entityType
					var balance sql.NullFloat64
					var party, date sql.NullString
					if err := rows.Scan(&c.ID, &c.RefNum, &party, &date, &c.TotalAmount, &balance); err != nil {
						rows.Close()
						return fmt.Errorf("scanning %s row: %w", src.entityType, err)
					}
					c.Party = party.String
					c.Date = date.String

					if balance.Valid {
						c.Balance = balance.Float64
					}

					// Date filter if target date is set.
					// Records with an empty or unparseable date are excluded when
					// --date is active; an undated transaction must not be treated
					// as a date-match candidate (would produce false positives).
					if !targetDate.IsZero() {
						if c.Date == "" {
							continue // no date recorded → cannot satisfy date filter
						}
						txnTime, err := time.Parse("2006-01-02", c.Date)
						if err != nil {
							txnTime, err = time.Parse(time.RFC3339, c.Date)
						}
						if err != nil {
							continue // unparseable date → exclude from date-bounded results
						}
						diffDays := math.Abs(txnTime.Sub(targetDate).Hours() / 24)
						if diffDays > float64(daysFlag) {
							continue
						}
					}

					candidates = append(candidates, c)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("reading %s rows: %w", src.entityType, err)
				}
				rows.Close()
			}

			if flags.asJSON {
				return flags.printJSON(cmd, candidates)
			}

			if len(candidates) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching ledger candidates found for $%.2f.\n", amountFlag)
				return nil
			}

			headers := []string{"TYPE", "ID", "DOC NUMBER", "PARTY", "DATE", "TOTAL AMT", "BALANCE"}
			var tableRows [][]string
			for _, c := range candidates {
				balStr := "-"
				if c.Type == "Invoice" || c.Type == "Bill" {
					balStr = fmt.Sprintf("$%.2f", c.Balance)
				}
				tableRows = append(tableRows, []string{
					c.Type,
					c.ID,
					c.RefNum,
					c.Party,
					c.Date,
					fmt.Sprintf("$%.2f", c.TotalAmount),
					balStr,
				})
			}

			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().Float64Var(&amountFlag, "amount", 0.0, "Transaction amount to match in ledger")
	cmd.Flags().StringVar(&dateFlag, "date", "", "Transaction date to match (YYYY-MM-DD)")
	cmd.Flags().IntVar(&daysFlag, "days", 7, "Allowed date window difference in days")
	_ = cmd.MarkFlagRequired("amount")

	return cmd
}
