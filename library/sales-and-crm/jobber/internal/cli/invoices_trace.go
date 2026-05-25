// Copyright 2026 melanson633. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/internal/store"
)

type invoiceTraceRow struct {
	Event             string  `json:"event"`
	InvoiceID         string  `json:"invoice_id"`
	InvoiceNumber     string  `json:"invoice_number"`
	ClientID          string  `json:"client_id"`
	ClientName        string  `json:"client_name"`
	Status            string  `json:"status"`
	Total             float64 `json:"total"`
	PaymentsTotal     float64 `json:"payments_total"`
	DepositAmount     float64 `json:"deposit_amount"`
	PaymentRecordsSum float64 `json:"payment_records_sum"`
	Balance           float64 `json:"balance"`
	Drift             float64 `json:"drift"`
	IssuedDate        string  `json:"issued_date"`
	DueDate           string  `json:"due_date"`
	JobberWebURI      string  `json:"jobber_web_uri"`
}

func newInvoicesGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "List and analyze invoices",
	}
	list := newInvoicesPromotedCmd(flags)
	list.Use = "list"
	list.Aliases = []string{"ls"}
	list.Short = "List invoices with optional filters"
	cmd.AddCommand(list)
	cmd.AddCommand(newInvoicesTraceSubcmd(flags))
	return cmd
}

func newInvoicesTraceSubcmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var mismatched bool
	var status string
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:         "trace",
		Short:       "Trace invoice totals, payments, deposits, and drift",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("jobber-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'jobber-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `SELECT i.id, COALESCE(i.invoice_number, ''), COALESCE(json_extract(i.data, '$.client.id'), ''), COALESCE(c.name, c.company_name, trim(COALESCE(c.first_name, '') || ' ' || COALESCE(c.last_name, '')), ''), COALESCE(i.invoice_status, ''), i.total, i.payments_total, i.deposit_amount, COALESCE((SELECT SUM(pr.amount) FROM payment_records pr WHERE json_extract(pr.data, '$.invoice.id') = i.id), 0), COALESCE(i.issued_date, ''), COALESCE(i.due_date, ''), COALESCE(i.jobber_web_uri, '')
FROM invoices i
LEFT JOIN clients c ON c.id = json_extract(i.data, '$.client.id')
WHERE (? = '' OR i.invoice_status = ?)
  AND (? = '' OR i.issued_date >= ?)
ORDER BY i.issued_date DESC, i.id`
			if limit > 0 {
				query += " LIMIT " + strconv.Itoa(limit)
			}
			rows, err := db.Query(query, status, status, since, since)
			if err != nil {
				return fmt.Errorf("querying invoice trace: %w", err)
			}
			defer rows.Close()

			var out []invoiceTraceRow
			for rows.Next() {
				var row invoiceTraceRow
				var total, paid, deposit, records sql.NullFloat64
				if err := rows.Scan(&row.InvoiceID, &row.InvoiceNumber, &row.ClientID, &row.ClientName, &row.Status, &total, &paid, &deposit, &records, &row.IssuedDate, &row.DueDate, &row.JobberWebURI); err != nil {
					return fmt.Errorf("scanning invoice trace: %w", err)
				}
				row.Event = "invoice_trace"
				row.Total = nullFloat(total)
				row.PaymentsTotal = nullFloat(paid)
				row.DepositAmount = nullFloat(deposit)
				row.PaymentRecordsSum = nullFloat(records)
				row.Drift = row.Total - row.PaymentsTotal - row.DepositAmount
				row.Balance = math.Max(0, row.Drift)
				if row.ClientName == "" {
					row.ClientName = row.ClientID
				}
				if mismatched && math.Abs(row.PaymentsTotal+row.DepositAmount-row.Total) <= 0.005 {
					continue
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading invoice trace: %w", err)
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for _, row := range out {
					if err := enc.Encode(row); err != nil {
						return err
					}
				}
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "invoice_number\tclient\tstatus\ttotal\tpaid\tdrift\tdue_date")
			for _, row := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%.2f\t%.2f\t%.2f\t%s\n", row.InvoiceNumber, row.ClientName, row.Status, row.Total, row.PaymentsTotal, row.Drift, row.DueDate)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&mismatched, "mismatched", false, "Only show invoices where paid plus deposits differ from total")
	cmd.Flags().StringVar(&status, "status", "", "Filter to invoice status")
	cmd.Flags().StringVar(&since, "since", "", "Filter to issued_date >= YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum invoices to return")
	return cmd
}
