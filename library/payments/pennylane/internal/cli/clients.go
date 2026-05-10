// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newClientsCmd returns the "clients" parent command.
func newClientsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Clients — classement, analyse, rentabilité",
	}
	cmd.AddCommand(newClientsRankCmd(flags))
	return cmd
}

// ─── clients rank ──────────────────────────────────────────────────────────

type clientRankRow struct {
	Rank         int     `json:"rank"`
	Client       string  `json:"client"`
	Total        float64 `json:"total"`
	InvoiceCount int     `json:"invoice_count"`
	PctOfTotal   float64 `json:"pct_of_total"`
}

func newClientsRankCmd(flags *rootFlags) *cobra.Command {
	var by string
	var ytd bool
	var period string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "rank",
		Short:       "Classement des clients par chiffre d'affaires ou nombre de factures",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli clients rank --by revenue --ytd
  accounting-pp-cli clients rank --by invoice_count --period 2026-Q1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			// Resolve period
			var start, end, label string
			if ytd {
				start = fmt.Sprintf("%d-01-01", time.Now().Year())
				end = time.Now().Format("2006-01-02")
				label = "YTD " + fmt.Sprintf("%d", time.Now().Year())
			} else if period != "" {
				var err error
				start, end, label, err = parsePeriod(period)
				if err != nil {
					return fmt.Errorf("invalid period: %w", err)
				}
			} else {
				// Default: YTD
				start = fmt.Sprintf("%d-01-01", time.Now().Year())
				end = time.Now().Format("2006-01-02")
				label = "YTD"
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.customer.name'), json_extract(data,'$.customer_name'), 'Unknown') AS client,
					COALESCE(CAST(json_extract(data,'$.amount_with_tax') AS REAL),
					         CAST(json_extract(data,'$.currency_amount') AS REAL),
					         0) AS amount
				FROM resources
				WHERE json_extract(data,'$.date') BETWEEN ? AND ?
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
			`, start, end)
			if err != nil {
				return fmt.Errorf("querying invoices: %w", err)
			}
			defer rows.Close()

			type agg struct {
				total float64
				count int
			}
			m := make(map[string]*agg)
			for rows.Next() {
				var client string
				var amount float64
				if err := rows.Scan(&client, &amount); err != nil {
					continue
				}
				if m[client] == nil {
					m[client] = &agg{}
				}
				m[client].total += amount
				m[client].count++
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			type entry struct {
				client string
				total  float64
				count  int
			}
			var entries []entry
			var grandTotal float64
			for c, a := range m {
				entries = append(entries, entry{c, math.Round(a.total*100) / 100, a.count})
				grandTotal += a.total
			}

			switch strings.ToLower(by) {
			case "invoice_count", "count":
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].count > entries[j].count
				})
			default: // revenue
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].total > entries[j].total
				})
			}

			var result []clientRankRow
			for i, e := range entries {
				pct := 0.0
				if grandTotal > 0 {
					pct = math.Round((e.total/grandTotal)*10000) / 100
				}
				result = append(result, clientRankRow{
					Rank:         i + 1,
					Client:       e.client,
					Total:        e.total,
					InvoiceCount: e.count,
					PctOfTotal:   pct,
				})
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("Période : %s\n\n", label)
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "RANG\tCLIENT\tTOTAL\tFACTURES\t% DU CA")
			for _, r := range result {
				fmt.Fprintf(tw, "%d\t%s\t%.2f\t%d\t%.1f%%\n",
					r.Rank, r.Client, r.Total, r.InvoiceCount, r.PctOfTotal)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&by, "by", "revenue", "Sort by: revenue or invoice_count")
	cmd.Flags().BoolVar(&ytd, "ytd", false, "Year to date")
	cmd.Flags().StringVar(&period, "period", "", "Period: YYYY-QN, YYYY-MM or YYYY")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
