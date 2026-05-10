// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newARCmd returns the "ar" parent command.
func newARCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ar",
		Short: "Accounts receivable — aging, reminders, DSO",
	}
	cmd.AddCommand(newARAgingCmd(flags))
	cmd.AddCommand(newARRemindCmd(flags))
	return cmd
}

// ─── ar aging ──────────────────────────────────────────────────────────────

type agingRow struct {
	Client       string  `json:"client"`
	Bucket       string  `json:"bucket"`
	Amount       float64 `json:"amount"`
	InvoiceCount int     `json:"invoice_count"`
}

func newARAgingCmd(flags *rootFlags) *cobra.Command {
	var bucketsStr string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "aging",
		Short:       "Balance âgée — créances clients par tranche de retard",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli ar aging
  accounting-pp-cli ar aging --buckets 0,30,60,90 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			// Parse bucket boundaries
			buckets := []int{0, 30, 60, 90}
			if bucketsStr != "" {
				buckets = nil
				for _, s := range strings.Split(bucketsStr, ",") {
					s = strings.TrimSpace(s)
					n, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("invalid bucket value %q: %w", s, err)
					}
					buckets = append(buckets, n)
				}
			}
			sort.Ints(buckets)

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.customer.name'), json_extract(data,'$.customer_name'), 'Unknown') AS client,
					COALESCE(CAST(json_extract(data,'$.remaining_amount_with_tax') AS REAL),
					         CAST(json_extract(data,'$.amount') AS REAL), 0) AS amount,
					CAST(julianday('now') - julianday(COALESCE(json_extract(data,'$.due_date'), json_extract(data,'$.deadline'))) AS INTEGER) AS days_overdue
				FROM resources
				WHERE resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				  AND (json_extract(data,'$.status') != 'paid' OR json_extract(data,'$.status') IS NULL)
				  AND json_extract(data,'$.paid') IS NOT 1
				  AND (json_extract(data,'$.due_date') IS NOT NULL OR json_extract(data,'$.deadline') IS NOT NULL)
			`)
			if err != nil {
				return fmt.Errorf("querying invoices: %w", err)
			}
			defer rows.Close()

			type key struct{ client, bucket string }
			type agg struct {
				amount float64
				count  int
			}
			m := make(map[key]*agg)

			for rows.Next() {
				var client string
				var amount float64
				var daysOverdue int
				if err := rows.Scan(&client, &amount, &daysOverdue); err != nil {
					continue
				}
				bucket := bucketLabel(daysOverdue, buckets)
				k := key{client, bucket}
				if m[k] == nil {
					m[k] = &agg{}
				}
				m[k].amount += amount
				m[k].count++
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			var result []agingRow
			for k, v := range m {
				result = append(result, agingRow{
					Client:       k.client,
					Bucket:       k.bucket,
					Amount:       math.Round(v.amount*100) / 100,
					InvoiceCount: v.count,
				})
			}
			sort.Slice(result, func(i, j int) bool {
				if result[i].Client != result[j].Client {
					return result[i].Client < result[j].Client
				}
				return result[i].Bucket < result[j].Bucket
			})

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CLIENT\tTRANCHE\tMONTANT\tFACTURES")
			for _, r := range result {
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%d\n", r.Client, r.Bucket, r.Amount, r.InvoiceCount)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&bucketsStr, "buckets", "0,30,60,90", "Comma-separated bucket boundaries in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// bucketLabel returns a human label for a given number of overdue days.
func bucketLabel(days int, buckets []int) string {
	if days <= 0 {
		return "courant"
	}
	for i := 0; i < len(buckets)-1; i++ {
		if days > buckets[i] && days <= buckets[i+1] {
			return fmt.Sprintf("%d-%dj", buckets[i], buckets[i+1])
		}
	}
	last := buckets[len(buckets)-1]
	return fmt.Sprintf(">%dj", last)
}

// ─── ar remind ─────────────────────────────────────────────────────────────

type remindRow struct {
	Client         string  `json:"client"`
	InvoiceNumber  string  `json:"invoice_number"`
	Amount         float64 `json:"amount"`
	DaysOverdue    int     `json:"days_overdue"`
	LastRemindedAt string  `json:"last_reminded_at,omitempty"`
}

func newARRemindCmd(flags *rootFlags) *cobra.Command {
	var overdueDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "remind",
		Short: "Liste des créances à relancer",
		Example: `  accounting-pp-cli ar remind --overdue-days 30
  accounting-pp-cli ar remind --overdue-days 7 --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			// Ensure reminder_log table exists
			if _, err := db.DB().ExecContext(cmd.Context(), `
				CREATE TABLE IF NOT EXISTS reminder_log (
					invoice_id TEXT NOT NULL,
					sent_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					channel    TEXT NOT NULL DEFAULT 'email',
					PRIMARY KEY (invoice_id, sent_at)
				)
			`); err != nil {
				return fmt.Errorf("creating reminder_log: %w", err)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					e.id,
					COALESCE(json_extract(e.data,'$.customer.name'), json_extract(e.data,'$.customer_name'), 'Unknown') AS client,
					COALESCE(json_extract(e.data,'$.invoice_number'), e.id) AS invoice_number,
					COALESCE(CAST(json_extract(e.data,'$.remaining_amount_with_tax') AS REAL),
					         CAST(json_extract(e.data,'$.amount') AS REAL), 0) AS amount,
					CAST(julianday('now') - julianday(COALESCE(json_extract(e.data,'$.due_date'), json_extract(e.data,'$.deadline'))) AS INTEGER) AS days_overdue,
					(SELECT MAX(sent_at) FROM reminder_log rl WHERE rl.invoice_id = e.id) AS last_reminded_at
				FROM resources e
				WHERE e.resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				  AND (json_extract(e.data,'$.status') != 'paid' OR json_extract(e.data,'$.status') IS NULL)
				  AND json_extract(e.data,'$.paid') IS NOT 1
				  AND (json_extract(e.data,'$.due_date') IS NOT NULL OR json_extract(e.data,'$.deadline') IS NOT NULL)
				  AND CAST(julianday('now') - julianday(COALESCE(json_extract(e.data,'$.due_date'), json_extract(e.data,'$.deadline'))) AS INTEGER) >= ?
				  AND (
				      (SELECT MAX(sent_at) FROM reminder_log rl WHERE rl.invoice_id = e.id) IS NULL
				      OR julianday('now') - julianday((SELECT MAX(sent_at) FROM reminder_log rl WHERE rl.invoice_id = e.id)) > 7
				  )
				ORDER BY days_overdue DESC
			`, overdueDays)
			if err != nil {
				return fmt.Errorf("querying invoices: %w", err)
			}
			defer rows.Close()

			var result []remindRow
			var ids []string
			for rows.Next() {
				var id, client, invoiceNum, lastReminded string
				var amount float64
				var daysOverdue int
				var nullReminded sql.NullString
				if err := rows.Scan(&id, &client, &invoiceNum, &amount, &daysOverdue, &nullReminded); err != nil {
					continue
				}
				if nullReminded.Valid {
					lastReminded = nullReminded.String
				}
				result = append(result, remindRow{
					Client:         client,
					InvoiceNumber:  invoiceNum,
					Amount:         math.Round(amount*100) / 100,
					DaysOverdue:    daysOverdue,
					LastRemindedAt: lastReminded,
				})
				ids = append(ids, id)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CLIENT\tFACTURE\tMONTANT\tRETARD (j)\tDERNIÈRE RELANCE")
			for _, r := range result {
				last := r.LastRemindedAt
				if last == "" {
					last = "jamais"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%d\t%s\n",
					r.Client, r.InvoiceNumber, r.Amount, r.DaysOverdue, last)
			}
			if err := tw.Flush(); err != nil {
				return err
			}

			if flags.dryRun || len(ids) == 0 {
				if flags.dryRun {
					fmt.Fprintln(os.Stderr, "[dry-run] aucun enregistrement créé dans reminder_log")
				}
				return nil
			}

			// Log reminders
			now := time.Now().UTC().Format(time.RFC3339)
			for _, id := range ids {
				if _, err := db.DB().ExecContext(cmd.Context(),
					`INSERT OR IGNORE INTO reminder_log (invoice_id, sent_at, channel) VALUES (?, ?, 'cli')`,
					id, now,
				); err != nil {
					fmt.Fprintf(os.Stderr, "warning: logging reminder for %s: %v\n", id, err)
				}
			}
			fmt.Fprintf(os.Stderr, "%d relances enregistrées dans reminder_log\n", len(ids))
			return nil
		},
	}

	cmd.Flags().IntVar(&overdueDays, "overdue-days", 30, "Minimum days overdue")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
