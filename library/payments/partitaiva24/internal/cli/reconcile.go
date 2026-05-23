// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored active/passive reconciliation report.

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type reconcileSide struct {
	Taxable float64 `json:"taxable"`
	VAT     float64 `json:"vat"`
	Total   float64 `json:"total"`
	Count   int     `json:"count"`
}

type reconcileReport struct {
	Period     string        `json:"period"`
	Active     reconcileSide `json:"active"`
	Passive    reconcileSide `json:"passive"`
	NetTaxable float64       `json:"net_taxable"`
	NetTotal   float64       `json:"net_total"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Net issued and received invoices for a period",
		Long:  "Compare active invoice totals against passive income records for a year, month, or quarter.",
		Example: `  partitaiva24-pp-cli reconcile --period 2026-Q1
  partitaiva24-pp-cli reconcile --period 2026-04 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if period == "" {
				return cmd.Help()
			}
			where, argsWhere, err := periodWhere(period)
			if err != nil {
				return usageErr(err)
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			side := func(table string) (reconcileSide, error) {
				q := fmt.Sprintf(`SELECT COALESCE(SUM(COALESCE(taxable, total)), 0), COALESCE(SUM(vat), 0), COALESCE(SUM(total), 0), COUNT(*) FROM %s WHERE %s`, table, where)
				var taxable, vat, total sql.NullFloat64
				var count int
				if err := s.DB().QueryRowContext(cmd.Context(), q, argsWhere...).Scan(&taxable, &vat, &total, &count); err != nil {
					return reconcileSide{}, err
				}
				return reconcileSide{Taxable: money2(nullableFloat(taxable)), VAT: money2(nullableFloat(vat)), Total: money2(nullableFloat(total)), Count: count}, nil
			}
			active, err := side("invoices")
			if err != nil {
				return err
			}
			passive, err := side("income")
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), reconcileReport{
				Period: period, Active: active, Passive: passive,
				NetTaxable: money2(active.Taxable - passive.Taxable), NetTotal: money2(active.Total - passive.Total),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "", "Period as YYYY, YYYY-MM, or YYYY-Qn")
	return cmd
}
