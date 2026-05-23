// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored quarterly tax projection.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type taxDueReport struct {
	Quarter          string   `json:"quarter"`
	Taxable          float64  `json:"taxable"`
	IRPEFSubstitute  float64  `json:"irpef_substitute"`
	IVA              *float64 `json:"iva"`
	INPSEstimate24   float64  `json:"inps_estimate_24"`
	INPSEstimate2572 float64  `json:"inps_estimate_25_72"`
	INPSNote         string   `json:"inps_note"`
	Regime           string   `json:"regime"`
	FiscalLabel      string   `json:"fiscal_label"`
}

func newTaxDueCmd(flags *rootFlags) *cobra.Command {
	var quarter string
	cmd := &cobra.Command{
		Use:   "tax-due",
		Short: "Project quarter-end taxes from synced invoices",
		Long: `Project IRPEF substitute tax, IVA when applicable, and INPS estimate brackets for a quarter.

The calculation reads synced invoices and fiscal_year metadata. INPS values are estimates because the actual previdential fund is not available in the local schema.`,
		Example: `  partitaiva24-pp-cli tax-due --quarter 2026-Q2
  partitaiva24-pp-cli tax-due --quarter 2026-Q2 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(quarter) == "" {
				return cmd.Help()
			}
			year, start, end, err := quarterBounds(quarter)
			if err != nil {
				return usageErr(err)
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			var taxable sql.NullFloat64
			if err := s.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(SUM(COALESCE(taxable, total)), 0) FROM invoices WHERE date >= ? AND date <= ? AND COALESCE(status, '') != 'draft'`,
				start.Format("2006-01-02"), end.Format("2006-01-02"),
			).Scan(&taxable); err != nil {
				return err
			}
			var label sql.NullString
			var taxRate sql.NullFloat64
			var isVAT sql.NullInt64
			_ = s.DB().QueryRowContext(cmd.Context(), `SELECT label, tax_rate, is_vat FROM fiscal_year WHERE year = ? ORDER BY synced_at DESC LIMIT 1`, year).Scan(&label, &taxRate, &isVAT)
			// Sync may store mock data for fiscal_year (it's a GET-by-year, not a list).
			// Fetch live so IRPEF projection has a real tax_rate.
			if !taxRate.Valid || taxRate.Float64 == 0 {
				if c, ferr := flags.newClient(); ferr == nil {
					if body, gerr := c.Get(fmt.Sprintf("/user/fiscal_year/%d", year), nil); gerr == nil {
						var fy struct {
							Label   string  `json:"label"`
							TaxRate float64 `json:"tax_rate"`
							IsVAT   bool    `json:"is_vat"`
						}
						if json.Unmarshal(body, &fy) == nil {
							if fy.Label != "" {
								label = sql.NullString{String: fy.Label, Valid: true}
							}
							if fy.TaxRate > 0 {
								taxRate = sql.NullFloat64{Float64: fy.TaxRate, Valid: true}
							}
							isVATInt := int64(0)
							if fy.IsVAT {
								isVATInt = 1
							}
							isVAT = sql.NullInt64{Int64: isVATInt, Valid: true}
						}
					}
				}
			}
			amount := nullableFloat(taxable)
			rate := nullableFloat(taxRate)
			var iva *float64
			if isVAT.Valid && isVAT.Int64 == 1 {
				var vat sql.NullFloat64
				_ = s.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(SUM(vat), 0) FROM invoices WHERE date >= ? AND date <= ? AND COALESCE(status, '') != 'draft'`,
					start.Format("2006-01-02"), end.Format("2006-01-02"),
				).Scan(&vat)
				v := money2(nullableFloat(vat))
				iva = &v
			}
			report := taxDueReport{
				Quarter: strings.ToUpper(quarter), Taxable: money2(amount), IRPEFSubstitute: money2(amount * rate / 100),
				IVA: iva, INPSEstimate24: money2(amount * 0.24), INPSEstimate2572: money2(amount * 0.2572),
				INPSNote: "estimate, depends on previdential fund", Regime: "forfettario", FiscalLabel: nullableString(label),
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&quarter, "quarter", "", "Quarter to project, e.g. 2026-Q2")
	return cmd
}
