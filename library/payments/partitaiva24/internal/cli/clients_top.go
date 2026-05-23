// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: adapted hand-authored clients top child for the clients command group.

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type clientRow struct {
	Rank         int     `json:"rank"`
	CustomerID   string  `json:"customer_id"`
	Customer     string  `json:"customer"`
	PIVA         string  `json:"p_iva,omitempty"`
	Total        float64 `json:"total"`
	InvoiceCount int     `json:"invoice_count"`
	PctOfYear    float64 `json:"pct_of_year"`
}

type clientsTopReport struct {
	Year          int            `json:"year"`
	YearTotal     float64        `json:"year_total"`
	Customers     []clientRow    `json:"customers"`
	Concentration map[string]any `json:"concentration"`
}

func newClientsTopCmd(flags *rootFlags) *cobra.Command {
	var year int
	var limit int
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank customers by year-to-date revenue (Pareto)",
		Long: `Aggregate the synced invoices table by destination customer and rank by
year-to-date taxable amount. Reports concentration (top 1, top 5 %) so
forfettari can spot the AdE single-client risk threshold (>80% from one
client invites scrutiny).`,
		Example: `  partitaiva24-pp-cli clients top
  partitaiva24-pp-cli clients top --year 2026 --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if year == 0 {
				year = currentYear()
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			start, end := yearRange(year)
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT
					COALESCE(json_extract(data, '$.to.id'), '') AS cid,
					COALESCE(json_extract(data, '$.to.companyname'),
					         TRIM(COALESCE(json_extract(data, '$.to.surname'), '') || ' ' || COALESCE(json_extract(data, '$.to.name'), ''))) AS cname,
					COALESCE(json_extract(data, '$.to.p_iva'), '') AS piva,
					COALESCE(SUM(COALESCE(taxable, total)), 0) AS total,
					COUNT(*) AS n
				FROM invoices
				WHERE date BETWEEN ? AND ? AND COALESCE(status,'') NOT IN ('draft','cancelled','annullata')
				GROUP BY cid, cname, piva
				HAVING total > 0
				ORDER BY total DESC`,
				start, end)
			if err != nil {
				return fmt.Errorf("query invoices: %w", err)
			}
			defer rows.Close()

			var all []clientRow
			yearTotal := 0.0
			for rows.Next() {
				var r clientRow
				var cid sql.NullString
				if err := rows.Scan(&cid, &r.Customer, &r.PIVA, &r.Total, &r.InvoiceCount); err != nil {
					return err
				}
				r.CustomerID = cid.String
				if r.Customer == "" {
					r.Customer = "(unknown)"
				}
				yearTotal += r.Total
				all = append(all, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			top1 := 0.0
			top5 := 0.0
			if yearTotal > 0 {
				if len(all) > 0 {
					top1 = all[0].Total / yearTotal * 100
				}
				for i := 0; i < len(all) && i < 5; i++ {
					top5 += all[i].Total / yearTotal * 100
				}
			}
			for i := range all {
				all[i].Rank = i + 1
				if yearTotal > 0 {
					all[i].PctOfYear = pct1(all[i].Total / yearTotal * 100)
				}
				all[i].Total = money2(all[i].Total)
			}
			if limit > 0 && limit < len(all) {
				all = all[:limit]
			}
			conc := map[string]any{
				"top1_pct": pct1(top1),
				"top5_pct": pct1(top5),
			}
			if top1 > 80 {
				conc["warning"] = "Single-client concentration > 80% — AdE may flag for forfettari review"
			} else if top1 > 50 {
				conc["warning"] = "Single-client concentration > 50% — monitor"
			}
			report := clientsTopReport{
				Year: year, YearTotal: money2(yearTotal),
				Customers: all, Concentration: conc,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Year: %d   Total invoiced: %.2f   (%d customers)\n", year, yearTotal, len(all))
			fmt.Fprintf(w, "Top-1 concentration: %.1f%%   Top-5: %.1f%%\n", top1, top5)
			if v, ok := conc["warning"]; ok {
				fmt.Fprintln(w, red("WARNING: "+v.(string)))
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%-4s  %-50s  %-14s  %5s  %s\n", "#", "CUSTOMER", "P.IVA", "INV", "TOTAL")
			for _, r := range all {
				name := r.Customer
				if len(name) > 50 {
					name = name[:47] + "..."
				}
				fmt.Fprintf(w, "%-4d  %-50s  %-14s  %5d  %12.2f\n", r.Rank, name, r.PIVA, r.InvoiceCount, r.Total)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "Fiscal year (default: current year)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Show top N customers (0 = no limit)")
	return cmd
}
