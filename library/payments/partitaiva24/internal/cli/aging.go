// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored AR aging report from local invoices.

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type agingCustomer struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

type agingBucket struct {
	Bucket    string          `json:"bucket"`
	Count     int             `json:"count"`
	Total     float64         `json:"total"`
	Customers []agingCustomer `json:"customers"`
}

func newAgingCmd(flags *rootFlags) *cobra.Command {
	asOf := time.Now().Format("2006-01-02")
	cmd := &cobra.Command{
		Use:   "aging",
		Short: "Group unpaid invoices by age bucket",
		Long:  "Read synced unpaid invoices and report AR aging buckets as of a selected date.",
		Example: `  partitaiva24-pp-cli aging --as-of 2026-05-09
  partitaiva24-pp-cli aging --json --select bucket,total,count`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			asOfDate, err := parseYMD(asOf)
			if err != nil {
				return usageErr(fmt.Errorf("--as-of must be YYYY-MM-DD"))
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT date, total, COALESCE(json_extract(data, '$.to.companyname'), ''), COALESCE(json_extract(data, '$.to.surname') || ' ' || json_extract(data, '$.to.name'), '') FROM invoices WHERE paid = 0 AND COALESCE(status, '') NOT IN ('draft','cancelled')`)
			if err != nil {
				return err
			}
			defer rows.Close()
			order := []string{"0-30", "31-60", "61-90", "90+"}
			byBucket := map[string]*agingBucket{}
			customerTotals := map[string]map[string]float64{}
			for _, b := range order {
				byBucket[b] = &agingBucket{Bucket: b}
				customerTotals[b] = map[string]float64{}
			}
			for rows.Next() {
				var dateStr string
				var total sql.NullFloat64
				var company, person sql.NullString
				if err := rows.Scan(&dateStr, &total, &company, &person); err != nil {
					return err
				}
				d, err := parseYMD(dateStr)
				if err != nil {
					continue
				}
				days := int(asOfDate.Sub(d).Hours() / 24)
				bucket := "0-30"
				switch {
				case days > 90:
					bucket = "90+"
				case days > 60:
					bucket = "61-90"
				case days > 30:
					bucket = "31-60"
				}
				name := nullableString(company)
				if name == "" {
					name = nullableString(person)
				}
				if name == "" {
					name = "Unknown"
				}
				amt := nullableFloat(total)
				byBucket[bucket].Count++
				byBucket[bucket].Total = money2(byBucket[bucket].Total + amt)
				customerTotals[bucket][name] += amt
			}
			if err := rows.Err(); err != nil {
				return err
			}
			out := make([]agingBucket, 0, len(order))
			for _, b := range order {
				row := byBucket[b]
				for name, total := range customerTotals[b] {
					row.Customers = append(row.Customers, agingCustomer{Name: name, Total: money2(total)})
				}
				sort.Slice(row.Customers, func(i, j int) bool { return row.Customers[i].Total > row.Customers[j].Total })
				out = append(out, *row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", asOf, "Aging date in YYYY-MM-DD")
	return cmd
}
