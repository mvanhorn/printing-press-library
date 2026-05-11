// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newInvoiceNovelCmd groups the novel invoice sub-commands.
func newInvoiceNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoice",
		Short: "Factures — import CSV, récurrences, suivi",
	}
	cmd.AddCommand(newInvoiceBulkCreateCmd(flags))
	cmd.AddCommand(newInvoiceCheckRecurringCmd(flags))
	return cmd
}

func newInvoiceBulkCreateCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bulk-create",
		Short: "Import CSV de factures en lot — validation + création séquentielle",
		Example: `  accounting-pp-cli invoice bulk-create --file invoices.csv --dry-run
  accounting-pp-cli invoice bulk-create --file invoices.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file est requis")
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run: 0 lignes validees (verify mode)")
				return nil
			}

			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("ouverture du CSV : %w", err)
			}
			defer f.Close()

			r := csv.NewReader(f)
			r.TrimLeadingSpace = true

			header, err := r.Read()
			if err != nil {
				return fmt.Errorf("lecture en-tête CSV : %w", err)
			}
			colIdx := make(map[string]int)
			for i, h := range header {
				colIdx[strings.ToLower(strings.TrimSpace(h))] = i
			}

			required := []string{"customer_name", "amount", "label", "date"}
			for _, req := range required {
				if _, ok := colIdx[req]; !ok {
					return fmt.Errorf("colonne manquante dans le CSV : %s (colonnes trouvées : %s)", req, strings.Join(header, ", "))
				}
			}

			// Load customer cache for fuzzy matching
			var customers []string
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, dbErr := store.OpenWithContext(cmd.Context(), dbPath)
			if dbErr == nil {
				defer db.Close()
				cRows, _ := db.DB().QueryContext(cmd.Context(), `
					SELECT DISTINCT COALESCE(name, json_extract(data,'$.name'), '') FROM resources
					WHERE resource_type IN ('external-v2-customers','external-v2-changelogs-customers')
					  AND COALESCE(name, json_extract(data,'$.name'), '') != ''
					LIMIT 500
				`)
				if cRows != nil {
					defer cRows.Close()
					for cRows.Next() {
						var n string
						if err := cRows.Scan(&n); err == nil && n != "" {
							customers = append(customers, n)
						}
					}
				}
			}

			type csvRow struct {
				Row             int
				CustomerName    string
				CustomerMatched string
				Amount          float64
				Label           string
				Date            string
				DueDate         string
				Status          string
				err             error
			}

			var results []csvRow
			rowNum := 1
			for {
				record, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("lecture CSV ligne %d : %w", rowNum+1, err)
				}
				rowNum++

				getCol := func(name string) string {
					if idx, ok := colIdx[name]; ok && idx < len(record) {
						return strings.TrimSpace(record[idx])
					}
					return ""
				}

				row := csvRow{
					Row:          rowNum,
					CustomerName: getCol("customer_name"),
					Label:        getCol("label"),
					Date:         getCol("date"),
					DueDate:      getCol("due_date"),
				}

				amtStr := getCol("amount")
				amt, parseErr := strconv.ParseFloat(strings.ReplaceAll(amtStr, ",", "."), 64)
				if parseErr != nil {
					row.Status = "error"
					row.err = fmt.Errorf("montant invalide : %q", amtStr)
				} else {
					row.Amount = math.Round(amt*100) / 100
				}

				// Fuzzy match customer
				row.CustomerMatched = fuzzyMatchCustomer(row.CustomerName, customers)

				if row.err != nil {
					row.Status = "error"
				} else {
					row.Status = "ok"
				}

				results = append(results, row)
			}

			if flags.asJSON {
				type jsonRow struct {
					Row             int     `json:"row"`
					CustomerName    string  `json:"customer_name"`
					CustomerMatched string  `json:"customer_matched"`
					Amount          float64 `json:"amount"`
					Label           string  `json:"label"`
					Date            string  `json:"date"`
					Status          string  `json:"status"`
					Error           string  `json:"error,omitempty"`
				}
				var out []jsonRow
				for _, r := range results {
					jr := jsonRow{
						Row: r.Row, CustomerName: r.CustomerName,
						CustomerMatched: r.CustomerMatched, Amount: r.Amount,
						Label: r.Label, Date: r.Date, Status: r.Status,
					}
					if r.err != nil {
						jr.Error = r.err.Error()
					}
					out = append(out, jr)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "LIGNE\tCLIENT\tCLIENT TROUVÉ\tMONTANT\tSTATUT")
			for _, r := range results {
				errStr := ""
				if r.err != nil {
					errStr = " (" + r.err.Error() + ")"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%.2f\t%s%s\n",
					r.Row, r.CustomerName, r.CustomerMatched, r.Amount, r.Status, errStr)
			}
			if err := tw.Flush(); err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(os.Stderr, "\n[dry-run] %d facture(s) validée(s), aucun appel API\n", len(results))
				return nil
			}

			// Real mode: create invoices sequentially via the API.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			created := 0
			failed := 0
			for _, r := range results {
				if r.Status != "ok" {
					failed++
					continue
				}
				body := map[string]any{
					"customer_name": r.CustomerMatched,
					"amount":        r.Amount,
					"label":         r.Label,
					"date":          r.Date,
				}
				if r.DueDate != "" {
					body["due_date"] = r.DueDate
				}
				_, statusCode, apiErr := c.Post("/api/external/v2/customer_invoices", body)
				if apiErr != nil || statusCode >= 400 {
					failed++
					if apiErr != nil {
						fmt.Fprintf(os.Stderr, "ligne %d : erreur API — %v\n", r.Row, apiErr)
					} else {
						fmt.Fprintf(os.Stderr, "ligne %d : erreur API HTTP %d\n", r.Row, statusCode)
					}
				} else {
					created++
				}
			}
			fmt.Fprintf(os.Stderr, "\n%d facture(s) créée(s), %d erreur(s)\n", created, failed)
			if failed > 0 {
				return fmt.Errorf("%d facture(s) n'ont pas pu être créées", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Chemin vers le fichier CSV")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// fuzzyMatchCustomer finds the best matching customer name using Levenshtein distance.
func fuzzyMatchCustomer(name string, customers []string) string {
	if len(customers) == 0 {
		return name
	}
	lower := strings.ToLower(name)
	best := name
	bestDist := len(name)/2 + 2 // threshold
	for _, c := range customers {
		d := levenshteinDistance(lower, strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

// ─── invoice check-recurring ───────────────────────────────────────────────

type recurringRow struct {
	Customer        string    `json:"customer"`
	ExpectedMonthly float64   `json:"expected_monthly"`
	Last3Months     []float64 `json:"last_3_months"`
	Status          string    `json:"status"`
	DriftPct        float64   `json:"drift_pct"`
}

func newInvoiceCheckRecurringCmd(flags *rootFlags) *cobra.Command {
	var tolerance float64
	var months int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "check-recurring",
		Short:       "Détection de dérive sur factures récurrentes mensuelles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli invoice check-recurring --tolerance 5 --months 6
  accounting-pp-cli invoice check-recurring --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			since := time.Now().AddDate(0, -months, 0).Format("2006-01-02")

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.customer.name'), json_extract(data,'$.customer_name'), 'Unknown') AS customer,
					strftime('%Y-%m', COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at'))) AS month,
					SUM(COALESCE(CAST(json_extract(data,'$.amount_with_tax') AS REAL),
					             CAST(json_extract(data,'$.currency_amount') AS REAL),
					             CAST(amount AS REAL), 0)) AS total
				FROM resources
				WHERE COALESCE(json_extract(data,'$.date'), json_extract(data,'$.created_at')) >= ?
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				GROUP BY customer, month
				ORDER BY customer, month
			`, since)
			if err != nil {
				return fmt.Errorf("querying invoices: %w", err)
			}
			defer rows.Close()

			type monthData struct {
				month string
				total float64
			}
			byClient := make(map[string][]monthData)

			for rows.Next() {
				var customer, month string
				var total float64
				if err := rows.Scan(&customer, &month, &total); err != nil {
					continue
				}
				byClient[customer] = append(byClient[customer], monthData{month, total})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			var result []recurringRow
			for customer, data := range byClient {
				if len(data) < 2 {
					continue // Not enough data to detect pattern
				}

				// Compute expected (average over all months)
				sum := 0.0
				for _, d := range data {
					sum += d.total
				}
				expected := sum / float64(len(data))

				// Last 3 months
				last3 := make([]float64, 0, 3)
				start := len(data) - 3
				if start < 0 {
					start = 0
				}
				for _, d := range data[start:] {
					last3 = append(last3, math.Round(d.total*100)/100)
				}

				// Check for missing months
				allMonths := buildMonthRange(since, time.Now().Format("2006-01"))
				monthsPresent := make(map[string]bool)
				for _, d := range data {
					monthsPresent[d.month] = true
				}
				missing := 0
				for _, m := range allMonths {
					if !monthsPresent[m] {
						missing++
					}
				}

				// Compute drift on last 3 months
				driftPct := 0.0
				if expected > 0 && len(last3) > 0 {
					lastAmt := last3[len(last3)-1]
					driftPct = math.Round(((lastAmt-expected)/expected)*10000) / 100
				}

				status := "ok"
				if missing > 0 {
					status = "missing_months"
				} else if math.Abs(driftPct) > tolerance {
					status = "drift"
				}

				result = append(result, recurringRow{
					Customer:        customer,
					ExpectedMonthly: math.Round(expected*100) / 100,
					Last3Months:     last3,
					Status:          status,
					DriftPct:        driftPct,
				})
			}

			sort.Slice(result, func(i, j int) bool {
				return result[i].Customer < result[j].Customer
			})

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CLIENT\tMENSUEL ATTENDU\tDERNIERS 3 MOIS\tSTATUT\tDÉRIVE %")
			for _, r := range result {
				last3Str := make([]string, len(r.Last3Months))
				for i, v := range r.Last3Months {
					last3Str[i] = fmt.Sprintf("%.0f", v)
				}
				fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%.1f%%\n",
					r.Customer, r.ExpectedMonthly, strings.Join(last3Str, " / "), r.Status, r.DriftPct)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().Float64Var(&tolerance, "tolerance", 5.0, "Drift tolerance percentage")
	cmd.Flags().IntVar(&months, "months", 6, "Number of months to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// buildMonthRange returns a slice of YYYY-MM strings from start to end (inclusive).
func buildMonthRange(start, end string) []string {
	var months []string
	t, err := time.Parse("2006-01", start[:7])
	if err != nil {
		return nil
	}
	endT, err := time.Parse("2006-01", end[:7])
	if err != nil {
		return nil
	}
	for !t.After(endT) {
		months = append(months, t.Format("2006-01"))
		t = t.AddDate(0, 1, 0)
	}
	return months
}
