// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored VIES single and bulk checks.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type viesEntry struct {
	CustomerID  string `json:"customer_id,omitempty"`
	CompanyName string `json:"companyname,omitempty"`
	PIVA        string `json:"p_iva"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type viesBulkReport struct {
	Checked int         `json:"checked"`
	Valid   []viesEntry `json:"valid"`
	Invalid []viesEntry `json:"invalid"`
	Errors  []viesEntry `json:"errors"`
}

func newViesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vies",
		Short: "Check EU VAT IDs through VIES",
		Long:  "Validate partita IVA values through the Partitaiva24 VIES tool.",
		Example: `  partitaiva24-pp-cli vies check DE 123456789
  partitaiva24-pp-cli vies bulk --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newViesCheckCmd(flags))
	cmd.AddCommand(newViesBulkCmd(flags))
	return cmd
}

func newViesCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <countryCode> <taxId>",
		Short: "Check one EU VAT ID",
		Long:  "Call /tools/check-vies/{countryCode}/{taxId} for a single VAT ID.",
		Example: `  partitaiva24-pp-cli vies check DE 123456789
  partitaiva24-pp-cli vies check FR FR12345678901 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) != 2 {
				return usageErr(fmt.Errorf("provide countryCode and taxId"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(fmt.Sprintf("/tools/check-vies/%s/%s", strings.ToUpper(args[0]), args[1]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func newViesBulkCmd(flags *rootFlags) *cobra.Command {
	limit := 0
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Check all synced EU customer VAT IDs",
		Long:  "Read EU customers from the local store and check each partita IVA through /tools/check-vies.",
		Example: `  partitaiva24-pp-cli vies bulk --limit 10
  partitaiva24-pp-cli vies bulk --json --select checked,invalid`,
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
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT id, companyname, p_iva, country FROM customers WHERE country_type = 'eu' AND COALESCE(p_iva, '') != '' ORDER BY companyname`)
			if err != nil {
				return err
			}
			defer rows.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report := viesBulkReport{}
			for rows.Next() {
				if limit > 0 && report.Checked >= limit {
					break
				}
				var id, name, piva, country sql.NullString
				if err := rows.Scan(&id, &name, &piva, &country); err != nil {
					return err
				}
				cc := strings.ToUpper(strings.TrimSpace(nullableString(country)))
				if len(cc) != 2 {
					cc = euCountryCode(cc)
				}
				if len(cc) != 2 {
					p := strings.TrimSpace(nullableString(piva))
					cc = strings.ToUpper(p[:min(2, len(p))])
				}
				tax := strings.TrimSpace(nullableString(piva))
				if strings.HasPrefix(strings.ToUpper(tax), cc) {
					tax = strings.TrimSpace(tax[2:])
				}
				entry := viesEntry{CustomerID: nullableString(id), CompanyName: nullableString(name), PIVA: nullableString(piva)}
				data, err := c.Get(fmt.Sprintf("/tools/check-vies/%s/%s", cc, tax), nil)
				report.Checked++
				if err != nil {
					entry.Status = "error"
					entry.Message = err.Error()
					report.Errors = append(report.Errors, entry)
				} else if viesLooksValid(data) {
					entry.Status = "valid"
					entry.Message = string(data)
					report.Valid = append(report.Valid, entry)
				} else {
					entry.Status = "invalid"
					entry.Message = string(data)
					report.Invalid = append(report.Invalid, entry)
				}
				time.Sleep(200 * time.Millisecond)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", limit, "Maximum customers to check (0 means unlimited)")
	return cmd
}

func euCountryCode(country string) string {
	names := map[string]string{
		"AUSTRIA": "AT", "BELGIUM": "BE", "BULGARIA": "BG", "CROATIA": "HR", "CYPRUS": "CY",
		"CZECHIA": "CZ", "CZECH REPUBLIC": "CZ", "DENMARK": "DK", "ESTONIA": "EE", "FINLAND": "FI",
		"FRANCE": "FR", "GERMANY": "DE", "GREECE": "EL", "HUNGARY": "HU", "IRELAND": "IE",
		"ITALY": "IT", "LATVIA": "LV", "LITHUANIA": "LT", "LUXEMBOURG": "LU", "MALTA": "MT",
		"NETHERLANDS": "NL", "POLAND": "PL", "PORTUGAL": "PT", "ROMANIA": "RO", "SLOVAKIA": "SK",
		"SLOVENIA": "SI", "SPAIN": "ES", "SWEDEN": "SE",
		"BELGIO": "BE", "CROAZIA": "HR", "CIPRO": "CY",
		"DANIMARCA": "DK", "FINLANDIA": "FI", "FRANCIA": "FR", "GERMANIA": "DE",
		"GRECIA": "EL", "UNGHERIA": "HU", "IRLANDA": "IE", "ITALIA": "IT", "LETTONIA": "LV",
		"LITUANIA": "LT", "LUSSEMBURGO": "LU", "PAESI BASSI": "NL", "POLONIA": "PL",
		"PORTOGALLO": "PT", "REPUBBLICA CECA": "CZ", "SLOVACCHIA": "SK",
		"SPAGNA": "ES", "SVEZIA": "SE",
	}
	return names[strings.ToUpper(strings.TrimSpace(country))]
}

func viesLooksValid(data json.RawMessage) bool {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return strings.Contains(strings.ToLower(string(data)), "valid")
	}
	var walk func(any) bool
	walk = func(x any) bool {
		switch t := x.(type) {
		case bool:
			return t
		case string:
			s := strings.ToLower(t)
			return s == "valid" || s == "true" || strings.Contains(s, "valid")
		case map[string]any:
			for k, v := range t {
				if strings.Contains(strings.ToLower(k), "valid") && walk(v) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}
