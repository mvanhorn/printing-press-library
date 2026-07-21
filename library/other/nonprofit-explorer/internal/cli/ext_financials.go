// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extension for the Nonprofit Explorer CLI (recorded in
// .printing-press-patches/ext-nonprofit-commands.md). Shared helpers live in
// ext_nonprofit.go; registration is in root.go.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

func newNPFinancialsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "financials <ein-or-name>",
		Short:       "Revenue / expense / asset / liability trajectory by year (with YoY revenue change)",
		Long:        "Revenue / expense / asset / liability trajectory by year (with YoY revenue change).\nAccepts an EIN (with or without dash) or a nonprofit name (auto-resolves to the top match).",
		Example:     "  nonprofit-explorer-pp-cli financials 530196605\n  nonprofit-explorer-pp-cli financials \"american red cross\"",
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
			// chronological ascending for trajectory
			sort.Slice(filings, func(i, j int) bool { return filings[i].TaxPrdYr < filings[j].TaxPrdYr })
			latest := latestFiling(resp.FilingsWithData)
			if flags.asJSON {
				out := map[string]any{
					"ein":        einDash(ein),
					"name":       resp.Organization.Name,
					"trajectory": filings,
				}
				if comp := compositionMap(latest); comp != nil {
					out["latest_composition"] = comp
				}
				if resolved != nil {
					out["resolved"] = resolved
				}
				return printJSONLive(cmd, flags, out)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (EIN %s) — financial trajectory\n\n", resp.Organization.Name, einDash(ein))
			rows := make([][]string, 0, len(filings))
			var prevRev *float64
			for _, f := range filings {
				yoy := "—"
				if f.TotRevenue != nil && prevRev != nil && *prevRev != 0 {
					pct := (*f.TotRevenue - *prevRev) / *prevRev * 100
					yoy = fmt.Sprintf("%+.1f%%", pct)
				}
				net := "—"
				if f.TotRevenue != nil && f.TotExpenses != nil {
					n := *f.TotRevenue - *f.TotExpenses
					net = fmtUSD(&n)
				}
				rows = append(rows, []string{
					strconv.Itoa(f.TaxPrdYr),
					fmtUSD(f.TotRevenue),
					yoy,
					fmtUSD(f.TotExpenses),
					net,
					fmtUSD(f.TotAssets),
				})
				if f.TotRevenue != nil {
					prevRev = f.TotRevenue
				}
			}
			if err := flags.printTable(cmd, []string{"YEAR", "REVENUE", "YOY", "EXPENSES", "NET", "ASSETS"}, rows); err != nil {
				return err
			}
			printCompositionBlock(w, latest)
			return nil
		},
	}
	return cmd
}

// compositionMap computes the latest filing's revenue composition and cost
// shares from the Form 990/990-EZ extract fields. Returns nil when the filing
// carries none of them (e.g. 990-PF, which uses a different extract layout —
// the extract has no functional-expense program/management split, so a
// classical program-expense ratio is not computable from this API).
func compositionMap(f *npFiling) map[string]any {
	if f == nil || f.TotRevenue == nil {
		return nil
	}
	if f.Contributions == nil && f.ProgramRev == nil && f.InvestmentInc == nil && f.OfficerComp == nil {
		return nil
	}
	m := map[string]any{"fiscal_year": f.TaxPrdYr, "form": formTypeName(f.FormType)}
	rev := *f.TotRevenue
	other := rev
	addShare := func(key string, v *float64) {
		if v == nil {
			return
		}
		other -= *v
		if rev != 0 {
			m[key] = map[string]any{"amount": *v, "pct_of_revenue": round1(*v / rev * 100)}
		} else {
			m[key] = map[string]any{"amount": *v}
		}
	}
	addShare("contributions", f.Contributions)
	addShare("program_revenue", f.ProgramRev)
	addShare("investment_income", f.InvestmentInc)
	if rev != 0 {
		m["other_revenue"] = map[string]any{"amount": other, "pct_of_revenue": round1(other / rev * 100)}
	}
	if f.TotExpenses != nil && *f.TotExpenses != 0 {
		exp := *f.TotExpenses
		if f.OfficerComp != nil || f.OtherSalaries != nil || f.PayrollTax != nil {
			personnel := 0.0
			for _, v := range []*float64{f.OfficerComp, f.OtherSalaries, f.PayrollTax} {
				if v != nil {
					personnel += *v
				}
			}
			m["personnel_costs"] = map[string]any{"amount": personnel, "pct_of_expenses": round1(personnel / exp * 100)}
		}
		if f.OfficerComp != nil {
			m["officer_compensation"] = map[string]any{"amount": *f.OfficerComp, "pct_of_expenses": round1(*f.OfficerComp / exp * 100)}
		}
	}
	return m
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// printCompositionBlock renders the latest filing's revenue composition for
// human output. Silent when composition fields are unavailable (990-PF).
func printCompositionBlock(w io.Writer, f *npFiling) {
	if f == nil || f.TotRevenue == nil {
		return
	}
	if f.Contributions == nil && f.ProgramRev == nil && f.InvestmentInc == nil {
		return
	}
	fmt.Fprintf(w, "\nRevenue composition (FY %d, Form %s):\n", f.TaxPrdYr, formTypeName(f.FormType))
	other := *f.TotRevenue
	line := func(label string, v *float64) {
		if v == nil {
			return
		}
		other -= *v
		fmt.Fprintf(w, "  %-22s %15s  %s\n", label, fmtUSD(v), pctOf(v, f.TotRevenue))
	}
	line("Contributions & grants", f.Contributions)
	line("Program revenue", f.ProgramRev)
	line("Investment income", f.InvestmentInc)
	fmt.Fprintf(w, "  %-22s %15s  %s\n", "Other revenue", fmtUSD(&other), pctOf(&other, f.TotRevenue))
	if f.TotExpenses != nil && (f.OfficerComp != nil || f.OtherSalaries != nil || f.PayrollTax != nil) {
		personnel := 0.0
		for _, v := range []*float64{f.OfficerComp, f.OtherSalaries, f.PayrollTax} {
			if v != nil {
				personnel += *v
			}
		}
		fmt.Fprintf(w, "  %-22s %15s  %s of expenses\n", "Personnel costs", fmtUSD(&personnel), pctOf(&personnel, f.TotExpenses))
	}
}
