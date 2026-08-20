// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/internal/store"
	"github.com/spf13/cobra"
)

type arAgingRow struct {
	Event        string  `json:"event"`
	ClientID     string  `json:"client_id"`
	ClientName   string  `json:"client_name"`
	Bucket030    float64 `json:"bucket_0_30"`
	Bucket3160   float64 `json:"bucket_31_60"`
	Bucket6190   float64 `json:"bucket_61_90"`
	BucketOver90 float64 `json:"bucket_over_90"`
	Total        float64 `json:"total"`
	InvoiceCount int     `json:"invoice_count"`
}

type arAgingTotal struct {
	Event        string  `json:"event"`
	ClientCount  int     `json:"client_count"`
	InvoiceCount int     `json:"invoice_count"`
	Bucket030    float64 `json:"bucket_0_30"`
	Bucket3160   float64 `json:"bucket_31_60"`
	Bucket6190   float64 `json:"bucket_61_90"`
	BucketOver90 float64 `json:"bucket_over_90"`
	Total        float64 `json:"total"`
	AsOf         string  `json:"as_of"`
}

func newARCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "ar",
		Short:       "Analyze accounts receivable from local synced data",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newARAgingCmd(flags))
	return cmd
}

func newARAgingCmd(flags *rootFlags) *cobra.Command {
	var asOfText string
	var dbPath string
	var includePaid bool
	var clientFilter string
	var top int
	var selectFields string

	cmd := &cobra.Command{
		Use:         "aging",
		Short:       "Bucket outstanding invoices by client and due-date age",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("jobber-pp-cli")
			}
			asOf, err := time.Parse("2006-01-02", asOfText)
			if err != nil {
				return fmt.Errorf("parsing --as-of as YYYY-MM-DD: %w", err)
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database read-only: %w\nRun 'jobber-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.Query(`SELECT i.id, COALESCE(json_extract(i.data, '$.client.id'), ''), COALESCE(c.name, c.company_name, trim(COALESCE(c.first_name, '') || ' ' || COALESCE(c.last_name, '')), ''), i.invoice_status, i.total, i.payments_total, i.due_date
FROM invoices i
LEFT JOIN clients c ON c.id = json_extract(i.data, '$.client.id')`)
			if err != nil {
				return fmt.Errorf("querying invoices: %w", err)
			}
			defer rows.Close()

			byClient := map[string]*arAgingRow{}
			for rows.Next() {
				var invoiceID, clientID, clientName string
				var status, due sql.NullString
				var total, paid sql.NullFloat64
				if err := rows.Scan(&invoiceID, &clientID, &clientName, &status, &total, &paid, &due); err != nil {
					return fmt.Errorf("scanning invoice: %w", err)
				}
				if clientID == "" {
					clientID = "unknown"
					clientName = "(unattributed)"
				}
				if clientName == "" {
					clientName = clientID
				}
				if clientFilter != "" && !strings.EqualFold(clientFilter, clientID) && !strings.Contains(strings.ToLower(clientName), strings.ToLower(clientFilter)) {
					continue
				}
				totalValue, paidValue := nullFloat(total), nullFloat(paid)
				if !includePaid && strings.EqualFold(status.String, "paid") && paidValue >= totalValue {
					continue
				}
				balance := math.Max(0, totalValue-paidValue)
				row := byClient[clientID]
				if row == nil {
					row = &arAgingRow{Event: "ar_aging_row", ClientID: clientID, ClientName: clientName}
					byClient[clientID] = row
				}
				if !addARAgingBucket(row, due.String, asOf, balance) {
					continue
				}
				row.Total += balance
				row.InvoiceCount++
				_ = invoiceID
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading invoices: %w", err)
			}

			out := make([]arAgingRow, 0, len(byClient))
			totalRow := arAgingTotal{Event: "ar_aging_total", AsOf: asOfText}
			for _, row := range byClient {
				out = append(out, *row)
				totalRow.InvoiceCount += row.InvoiceCount
				totalRow.Bucket030 += row.Bucket030
				totalRow.Bucket3160 += row.Bucket3160
				totalRow.Bucket6190 += row.Bucket6190
				totalRow.BucketOver90 += row.BucketOver90
				totalRow.Total += row.Total
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			totalRow.ClientCount = len(out)

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for _, row := range out {
					if err := encodeMaybeSelected(cmd.OutOrStdout(), enc, row, selectFields); err != nil {
						return err
					}
				}
				return encodeMaybeSelected(cmd.OutOrStdout(), enc, totalRow, selectFields)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Client\t0-30\t31-60\t61-90\t90+\tTotal\tInvoices")
			for _, row := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t%d\n", row.ClientName, row.Bucket030, row.Bucket3160, row.Bucket6190, row.BucketOver90, row.Total, row.InvoiceCount)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "TOTAL\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t%d\n", totalRow.Bucket030, totalRow.Bucket3160, totalRow.Bucket6190, totalRow.BucketOver90, totalRow.Total, totalRow.InvoiceCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&asOfText, "as-of", time.Now().UTC().Format("2006-01-02"), "Date for aging buckets (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&includePaid, "include-paid", false, "Include fully paid invoices")
	cmd.Flags().StringVar(&clientFilter, "client", "", "Filter to one client by name or id")
	cmd.Flags().IntVar(&top, "top", 0, "Return top N clients by outstanding total")
	cmd.Flags().StringVar(&selectFields, "select", "", "Comma-separated top-level JSON fields to include")
	return cmd
}

func addARAgingBucket(row *arAgingRow, dueText string, asOf time.Time, balance float64) bool {
	due, ok := parseJobberDate(dueText)
	if !ok {
		return false
	}
	ageDays := int(asOf.Sub(due).Hours() / 24)
	switch {
	case ageDays <= 30:
		row.Bucket030 += balance
	case ageDays <= 60:
		row.Bucket3160 += balance
	case ageDays <= 90:
		row.Bucket6190 += balance
	default:
		row.BucketOver90 += balance
	}
	return true
}

func parseJobberDate(s string) (time.Time, bool) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func nullFloat(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}

func encodeMaybeSelected(w io.Writer, enc *json.Encoder, v any, fields string) error {
	if strings.TrimSpace(fields) == "" {
		return enc.Encode(v)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(filterFields(raw, fields), '\n'))
	return err
}
