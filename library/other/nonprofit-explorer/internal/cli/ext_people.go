// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extension for the Nonprofit Explorer CLI (recorded in
// .printing-press-patches/ext-nonprofit-commands.md). Shared helpers live in
// ext_nonprofit.go; registration is in root.go.

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

func newNPPeopleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people <ein-or-name>",
		Short: "Officer compensation and staffing cost aggregates from Form 990 extracts",
		Long: "Officer compensation and staffing cost aggregates by year, from the IRS Form 990\n" +
			"extract fields ProPublica exposes (officer compensation total and its share of\n" +
			"expenses, other salaries & wages, payroll taxes, professional fundraising fees).\n" +
			"Accepts an EIN (with or without dash) or a nonprofit name (auto-resolves).\n\n" +
			"Limitation: the extract carries AGGREGATES only. Individual officer/director\n" +
			"names, titles, and per-person compensation exist only in the filed 990 PDF\n" +
			"(Part VII) — each row links the PDF for that. 990-PF filings use a different\n" +
			"extract layout without these fields and render as unavailable.",
		Example:     "  nonprofit-explorer-pp-cli people 530196605\n  nonprofit-explorer-pp-cli people \"american red cross\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ein, resolved, err := resolveEINOrName(cmd.Context(), c, flags, cmd, args[0])
			if err != nil {
				return err
			}
			resp, err := fetchOrg(cmd.Context(), c, ein)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			filings := append([]npFiling{}, resp.FilingsWithData...)
			sort.Slice(filings, func(i, j int) bool { return filings[i].TaxPrdYr > filings[j].TaxPrdYr })
			type peopleYear struct {
				FiscalYear     int      `json:"fiscal_year"`
				Form           string   `json:"form"`
				OfficerComp    *float64 `json:"officer_compensation,omitempty"`
				OfficerCompPct string   `json:"officer_comp_pct_of_expenses,omitempty"`
				OtherSalaries  *float64 `json:"other_salaries_wages,omitempty"`
				PayrollTax     *float64 `json:"payroll_taxes,omitempty"`
				ProFundraising *float64 `json:"professional_fundraising_fees,omitempty"`
				PdfURL         string   `json:"pdf_url,omitempty"`
			}
			var years []peopleYear
			for _, f := range filings {
				y := peopleYear{FiscalYear: f.TaxPrdYr, Form: formTypeName(f.FormType), OfficerComp: f.OfficerComp,
					OtherSalaries: f.OtherSalaries, PayrollTax: f.PayrollTax, ProFundraising: f.ProFundraising, PdfURL: f.PdfURL}
				if p := pctOf(f.OfficerComp, f.TotExpenses); p != "—" {
					y.OfficerCompPct = p
				}
				years = append(years, y)
			}
			if flags.asJSON {
				out := map[string]any{
					"ein":        einDash(ein),
					"name":       resp.Organization.Name,
					"years":      years,
					"limitation": "IRS extract carries aggregates only; individual officer names/titles/comp are in the 990 PDF (Part VII) linked per year",
				}
				if resolved != nil {
					out["resolved"] = resolved
				}
				return printJSONLive(cmd, flags, out)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (EIN %s) — compensation & staffing aggregates\n\n", resp.Organization.Name, einDash(ein))
			rows := make([][]string, 0, len(years))
			for _, y := range years {
				pct := y.OfficerCompPct
				if pct == "" {
					pct = "—"
				}
				rows = append(rows, []string{
					strconv.Itoa(y.FiscalYear), y.Form,
					fmtUSD(y.OfficerComp), pct,
					fmtUSD(y.OtherSalaries), fmtUSD(y.PayrollTax), fmtUSD(y.ProFundraising),
				})
			}
			if err := flags.printTable(cmd, []string{"YEAR", "FORM", "OFFICER COMP", "% OF EXP", "OTHER SALARIES", "PAYROLL TAX", "PRO FUNDRAISING"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(w, "\nNote: extract aggregates only — officer names/titles/per-person comp live in the\nfiled 990 PDF (Part VII). Latest PDF: %s\n", func() string {
				if lf := latestFiling(resp.FilingsWithData); lf != nil && lf.PdfURL != "" {
					return lf.PdfURL
				}
				return "(none available)"
			}())
			return nil
		},
	}
	return cmd
}
