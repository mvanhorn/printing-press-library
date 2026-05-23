// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored esterometro CSV export.

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newEsterometroExportCmd(flags *rootFlags) *cobra.Command {
	output := ""
	cmd := &cobra.Command{
		Use:   "export <year>",
		Short: "Export simplified esterometro CSV",
		Long:  "Fetch live foreign-mgmt records and combine them with synced EU/extra-EU active invoices into a simplified AdE esterometro CSV.",
		Example: `  partitaiva24-pp-cli esterometro export 2026
  partitaiva24-pp-cli esterometro export 2026 -o esterometro-2026.csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			year, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("year must be numeric"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			live, err := c.Get("/user/foreign-mgmt", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var out io.Writer = cmd.OutOrStdout()
			var f *os.File
			if output != "" {
				f, err = os.Create(homeExpanded(output))
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}
			w := csv.NewWriter(out)
			defer w.Flush()
			if err := w.Write([]string{"tipo", "data", "paese", "p_iva_estero", "denominazione", "imponibile", "iva", "natura", "fattura_n", "fattura_data"}); err != nil {
				return err
			}
			for _, row := range foreignMgmtRows(live, year) {
				if err := w.Write(row); err != nil {
					return err
				}
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT date, number, taxable, vat, data FROM invoices WHERE strftime('%Y', date) = ? AND json_extract(data, '$.to.country_type') IN ('eu','extra')`, fmt.Sprintf("%04d", year))
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var date, number, data sql.NullString
				var taxable, vat sql.NullFloat64
				if err := rows.Scan(&date, &number, &taxable, &vat, &data); err != nil {
					return err
				}
				raw := nullableString(data)
				row := []string{"attivo", nullableString(date), firstJSONText(raw, "to.country"), firstJSONText(raw, "to.p_iva"), firstJSONText(raw, "to.companyname", "to.surname"), fmt.Sprintf("%.2f", nullableFloat(taxable)), fmt.Sprintf("%.2f", nullableFloat(vat)), firstJSONText(raw, "nature", "natura"), nullableString(number), nullableString(date)}
				if err := w.Write(row); err != nil {
					return err
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write CSV to path instead of stdout")
	return cmd
}

func foreignMgmtRows(data json.RawMessage, year int) [][]string {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	var out [][]string
	for _, item := range rows {
		date := firstMapString(item, "date", "data", "invoice_date", "fattura_data")
		if len(date) < 4 || date[:4] != fmt.Sprintf("%04d", year) {
			continue
		}
		out = append(out, []string{"passivo", date, firstMapString(item, "country", "paese"), firstMapString(item, "p_iva", "vat", "p_iva_estero"), firstMapString(item, "companyname", "denominazione", "sender"), firstMapString(item, "taxable", "imponibile"), firstMapString(item, "vat", "iva"), firstMapString(item, "natura", "nature"), firstMapString(item, "number", "fattura_n"), date})
	}
	return out
}
