// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored bollo due summary.

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type stampQuarter struct {
	Quarter string  `json:"quarter"`
	Total   float64 `json:"total"`
}

type stampDueReport struct {
	Year        int            `json:"year"`
	ByQuarter   []stampQuarter `json:"by_quarter"`
	AnnualTotal float64        `json:"annual_total"`
}

func newStampDueCmd(flags *rootFlags) *cobra.Command {
	year := currentYear()
	cmd := &cobra.Command{
		Use:   "stamp-due",
		Short: "Sum bollo due by quarter",
		Long:  "Read synced invoices with stamp data and sum bollo amounts by quarter for AdE cross-checks.",
		Example: `  partitaiva24-pp-cli stamp-due --year 2026
  partitaiva24-pp-cli stamp-due --year 2026 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT strftime('%m', date), COALESCE(json_extract(data, '$.stamp.amount'), 0) FROM invoices WHERE strftime('%Y', date) = ? AND json_extract(data, '$.stamp') IS NOT NULL`, fmt.Sprintf("%04d", year))
			if err != nil {
				return err
			}
			defer rows.Close()
			totals := []float64{0, 0, 0, 0}
			for rows.Next() {
				var month string
				var amount sql.NullFloat64
				if err := rows.Scan(&month, &amount); err != nil {
					return err
				}
				m := 0
				fmt.Sscanf(month, "%d", &m)
				if m >= 1 && m <= 12 {
					totals[(m-1)/3] += nullableFloat(amount)
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			var annual float64
			var by []stampQuarter
			for i, total := range totals {
				annual += total
				by = append(by, stampQuarter{Quarter: fmt.Sprintf("Q%d", i+1), Total: money2(total)})
			}
			return printJSONFiltered(cmd.OutOrStdout(), stampDueReport{Year: year, ByQuarter: by, AnnualTotal: money2(annual)}, flags)
		},
	}
	cmd.Flags().IntVar(&year, "year", year, "Fiscal year")
	return cmd
}
