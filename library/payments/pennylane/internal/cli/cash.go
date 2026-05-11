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

// newCashCmd returns the "cash" parent command.
func newCashCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cash",
		Short: "Trésorerie — DSO, runway, projection",
	}
	cmd.AddCommand(newCashDSOCmd(flags))
	cmd.AddCommand(newCashRunwayCmd(flags))
	return cmd
}

// ─── cash dso ──────────────────────────────────────────────────────────────

type dsoRow struct {
	Client       string  `json:"client"`
	AvgDays      float64 `json:"avg_days"`
	InvoiceCount int     `json:"invoice_count"`
	Trend        string  `json:"trend"`
}

type dsoResult struct {
	GlobalDSO float64  `json:"global_dso"`
	Period    string   `json:"period"`
	Clients   []dsoRow `json:"clients"`
}

func newCashDSOCmd(flags *rootFlags) *cobra.Command {
	var rolling int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "dso",
		Short:       "Days Sales Outstanding — délai moyen de paiement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli cash dso
  accounting-pp-cli cash dso --rolling 90 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			periodStart := time.Now().AddDate(0, 0, -rolling).Format("2006-01-02")
			prevStart := time.Now().AddDate(0, 0, -rolling*2).Format("2006-01-02")

			// Current period
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.customer.name'), json_extract(data,'$.customer_name'), 'Unknown') AS client,
					CAST(julianday(COALESCE(json_extract(data,'$.paid_at'), updated_at)) - julianday(COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at'))) AS REAL) AS payment_days
				FROM resources
				WHERE json_extract(data,'$.paid') = 1
				  AND COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at')) >= ?
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				  AND CAST(julianday(COALESCE(json_extract(data,'$.paid_at'), updated_at)) - julianday(COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at'))) AS REAL) > 0
			`, periodStart)
			if err != nil {
				return fmt.Errorf("querying paid invoices: %w", err)
			}
			defer rows.Close()

			type clientAgg struct {
				totalDays float64
				count     int
				totalAmt  float64
			}
			current := make(map[string]*clientAgg)
			var globalDays, globalAmt float64

			for rows.Next() {
				var client string
				var days float64
				if err := rows.Scan(&client, &days); err != nil {
					continue
				}
				if current[client] == nil {
					current[client] = &clientAgg{}
				}
				current[client].totalDays += days
				current[client].count++
				current[client].totalAmt += 1
				globalDays += days
				globalAmt++
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			// Previous period for trend
			prevRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.customer.name'), json_extract(data,'$.customer_name'), 'Unknown') AS client,
					AVG(CAST(julianday(COALESCE(json_extract(data,'$.paid_at'), updated_at)) - julianday(COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at'))) AS REAL)) AS avg_days
				FROM resources
				WHERE json_extract(data,'$.paid') = 1
				  AND COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at')) BETWEEN ? AND ?
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				GROUP BY client
			`, prevStart, periodStart)
			prev := make(map[string]float64)
			if err == nil {
				defer prevRows.Close()
				for prevRows.Next() {
					var client string
					var avgDays float64
					if err := prevRows.Scan(&client, &avgDays); err == nil {
						prev[client] = avgDays
					}
				}
			}
			// Previous-period query failure is non-fatal: trends show "stable" for all clients.

			var result []dsoRow
			for client, agg := range current {
				avg := 0.0
				if agg.count > 0 {
					avg = agg.totalDays / float64(agg.count)
				}
				trend := "stable"
				if p, ok := prev[client]; ok {
					diff := avg - p
					if diff > 2 {
						trend = "↑ pire"
					} else if diff < -2 {
						trend = "↓ meilleur"
					}
				}
				result = append(result, dsoRow{
					Client:       client,
					AvgDays:      math.Round(avg*10) / 10,
					InvoiceCount: agg.count,
					Trend:        trend,
				})
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].AvgDays > result[j].AvgDays
			})

			globalDSO := 0.0
			if globalAmt > 0 {
				globalDSO = math.Round((globalDays/globalAmt)*10) / 10
			}

			res := dsoResult{
				GlobalDSO: globalDSO,
				Period:    fmt.Sprintf("derniers %d jours", rolling),
				Clients:   result,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			fmt.Printf("DSO global : %.1f jours (période : %s)\n\n", globalDSO, res.Period)
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CLIENT\tMOY (j)\tFACTURES\tTENDANCE")
			for _, r := range result {
				fmt.Fprintf(tw, "%s\t%.1f\t%d\t%s\n", r.Client, r.AvgDays, r.InvoiceCount, r.Trend)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&rolling, "rolling", 90, "Rolling window in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ─── cash runway ───────────────────────────────────────────────────────────

type runwayDay struct {
	Date             string  `json:"date"`
	ProjectedBalance float64 `json:"projected_balance"`
	ARExpected       float64 `json:"ar_expected"`
	APDue            float64 `json:"ap_due"`
}

func newCashRunwayCmd(flags *rootFlags) *cobra.Command {
	var horizon int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "runway",
		Short:       "Projection de trésorerie jour par jour",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  accounting-pp-cli cash runway --horizon 90 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			// Get starting balance from transactions if available
			var startBalance float64
			_ = db.DB().QueryRowContext(cmd.Context(), `
				SELECT COALESCE(SUM(CAST(json_extract(data,'$.amount') AS REAL)), 0)
				FROM resources
				WHERE resource_type IN ('external-v2-transactions','external-v2-changelogs-transactions')
			`).Scan(&startBalance)

			// Build day map
			days := make(map[string]*runwayDay)
			today := time.Now().Truncate(24 * time.Hour)
			for i := 0; i <= horizon; i++ {
				d := today.AddDate(0, 0, i).Format("2006-01-02")
				days[d] = &runwayDay{Date: d}
			}

			// Open AR: unpaid customer invoices expected by deadline
			arRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(deadline, json_extract(data,'$.deadline')) AS due_date,
					COALESCE(CAST(json_extract(data,'$.remaining_amount_with_tax') AS REAL),
					         CAST(amount AS REAL), 0) AS amount
				FROM resources
				WHERE json_extract(data,'$.paid') IS NOT 1
				  AND COALESCE(deadline, json_extract(data,'$.deadline')) IS NOT NULL
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
			`)
			if err == nil {
				defer arRows.Close()
				for arRows.Next() {
					var dueDate string
					var amount float64
					if err := arRows.Scan(&dueDate, &amount); err != nil {
						continue
					}
					if d, ok := days[dueDate[:10]]; ok {
						d.ARExpected += amount
					}
				}
			}

			// Open AP: unpaid supplier invoices
			apRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(deadline, json_extract(data,'$.deadline'), due_date, json_extract(data,'$.due_date')) AS due_date,
					COALESCE(CAST(json_extract(data,'$.remaining_amount_with_tax') AS REAL),
					         CAST(amount AS REAL), 0) AS amount
				FROM resources
				WHERE (json_extract(data,'$.payment_status') NOT IN ('paid','settled') OR json_extract(data,'$.payment_status') IS NULL)
				  AND resource_type IN ('external-v2-supplier-invoices','external-v2-changelogs-supplier-invoices')
				  AND COALESCE(deadline, json_extract(data,'$.deadline'), due_date, json_extract(data,'$.due_date')) IS NOT NULL
			`)
			if err == nil {
				defer apRows.Close()
				for apRows.Next() {
					var dueDate string
					var amount float64
					if err := apRows.Scan(&dueDate, &amount); err != nil {
						continue
					}
					dueKey := dueDate
					if len(dueKey) > 10 {
						dueKey = dueKey[:10]
					}
					if d, ok := days[dueKey]; ok {
						d.APDue += amount
					}
				}
			}

			// Build sorted projection
			var result []runwayDay
			var sortedDates []string
			for d := range days {
				sortedDates = append(sortedDates, d)
			}
			sort.Strings(sortedDates)

			balance := startBalance
			for _, d := range sortedDates {
				day := days[d]
				balance += day.ARExpected - day.APDue
				result = append(result, runwayDay{
					Date:             d,
					ProjectedBalance: math.Round(balance*100) / 100,
					ARExpected:       math.Round(day.ARExpected*100) / 100,
					APDue:            math.Round(day.APDue*100) / 100,
				})
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "DATE\tSOLDE PROJETÉ\tAR ATTENDU\tAP DÛ")
			for _, r := range result {
				fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\n",
					r.Date, r.ProjectedBalance, r.ARExpected, r.APDue)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&horizon, "horizon", 90, "Projection horizon in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
