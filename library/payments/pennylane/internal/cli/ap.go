// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newAPCmd returns the "ap" parent command.
func newAPCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ap",
		Short: "Comptes fournisseurs — optimisation des paiements",
	}
	cmd.AddCommand(newAPScheduleCmd(flags))
	return cmd
}

// ─── ap schedule ───────────────────────────────────────────────────────────

type apPaymentRow struct {
	Rank         int     `json:"rank"`
	Supplier     string  `json:"supplier"`
	Amount       float64 `json:"amount"`
	Deadline     string  `json:"deadline"`
	DaysUntilDue int     `json:"days_until_due"`
	Overdue      bool    `json:"overdue"`
}

func newAPScheduleCmd(flags *rootFlags) *cobra.Command {
	var horizon int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "schedule",
		Short:       "Optimisation des paiements fournisseurs — ordre suggéré par échéance",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli ap schedule
  accounting-pp-cli ap schedule --horizon 60 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, horizon).Format("2006-01-02")
			today := time.Now().Format("2006-01-02")

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					id,
					COALESCE(json_extract(data,'$.supplier.name'), json_extract(data,'$.supplier_name'), name, 'Unknown') AS supplier,
					COALESCE(CAST(json_extract(data,'$.remaining_amount_with_tax') AS REAL),
					         CAST(json_extract(data,'$.amount_with_tax') AS REAL),
					         CAST(amount AS REAL), 0) AS amount,
					COALESCE(deadline, json_extract(data,'$.deadline'), due_date, json_extract(data,'$.due_date')) AS due_date
				FROM resources
				WHERE resource_type IN ('external-v2-supplier-invoices','external-v2-changelogs-supplier-invoices')
				  AND (json_extract(data,'$.payment_status') NOT IN ('paid','settled') OR json_extract(data,'$.payment_status') IS NULL)
				  AND COALESCE(deadline, json_extract(data,'$.deadline'), due_date, json_extract(data,'$.due_date')) <= ?
				ORDER BY due_date ASC, amount ASC
			`, cutoff)
			if err != nil {
				return fmt.Errorf("querying supplier invoices: %w", err)
			}
			defer rows.Close()

			var result []apPaymentRow
			rank := 0
			for rows.Next() {
				var id, supplier, dueDate string
				var amount float64
				if err := rows.Scan(&id, &supplier, &amount, &dueDate); err != nil {
					continue
				}
				if len(dueDate) > 10 {
					dueDate = dueDate[:10]
				}
				daysUntil := 0
				overdue := false
				if dueDate != "" {
					t, err := time.Parse("2006-01-02", dueDate)
					if err == nil {
						diff := int(math.Round(t.Sub(time.Now()).Hours() / 24))
						daysUntil = diff
						overdue = dueDate < today
					}
				}
				rank++
				result = append(result, apPaymentRow{
					Rank:         rank,
					Supplier:     supplier,
					Amount:       math.Round(amount*100) / 100,
					Deadline:     dueDate,
					DaysUntilDue: daysUntil,
					Overdue:      overdue,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			// Sort: overdue first (by deadline ASC), then upcoming by deadline ASC, amount ASC
			sort.SliceStable(result, func(i, j int) bool {
				if result[i].Overdue != result[j].Overdue {
					return result[i].Overdue
				}
				if result[i].Deadline != result[j].Deadline {
					return result[i].Deadline < result[j].Deadline
				}
				return result[i].Amount < result[j].Amount
			})
			// Re-rank after sort
			for i := range result {
				result[i].Rank = i + 1
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "RANG\tFOURNISSEUR\tMONTANT\tÉCHÉANCE\tJ RESTANTS\tEN RETARD")
			for _, r := range result {
				overdueStr := ""
				if r.Overdue {
					overdueStr = "⚠ OUI"
				}
				fmt.Fprintf(tw, "%d\t%s\t%.2f\t%s\t%d\t%s\n",
					r.Rank, r.Supplier, r.Amount, r.Deadline, r.DaysUntilDue, overdueStr)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&horizon, "horizon", 60, "Horizon in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
