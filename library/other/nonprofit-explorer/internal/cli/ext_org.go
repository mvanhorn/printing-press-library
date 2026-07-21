// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extension for the Nonprofit Explorer CLI (recorded in
// .printing-press-patches/ext-nonprofit-commands.md). Shared helpers live in
// ext_nonprofit.go; registration is in root.go.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNPOrgCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "org <ein-or-name>",
		Short:       "Show an organization profile plus its latest Form 990 financials",
		Long:        "Show an organization profile plus its latest Form 990 financials.\nAccepts an EIN (with or without dash) or a nonprofit name (auto-resolves to the top match).",
		Example:     "  nonprofit-explorer-pp-cli org 530196605\n  nonprofit-explorer-pp-cli org \"american red cross\"\n  nonprofit-explorer-pp-cli org 87-4084202 --agent",
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
			o := resp.Organization
			latest := latestFiling(resp.FilingsWithData)
			if flags.asJSON {
				out := map[string]any{
					"organization":               o,
					"ntee_name":                  nteeName(o.NteeCode),
					"ntee_category":              nteeCategory(o.NteeCode),
					"latest_filing":              latest,
					"filings_with_data_count":    len(resp.FilingsWithData),
					"filings_without_data_count": len(resp.FilingsWithoutData),
				}
				if resolved != nil {
					out["resolved"] = resolved
				}
				return printJSONLive(cmd, flags, out)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", o.Name)
			fmt.Fprintf(w, "EIN %s  •  501(c)(%d)\n", einDash(ein), o.Subseccd)
			loc := strings.TrimSpace(o.Address)
			cityLine := strings.TrimSpace(o.City + ", " + o.State + " " + o.Zipcode)
			if loc != "" {
				fmt.Fprintf(w, "%s, %s\n", loc, cityLine)
			} else {
				fmt.Fprintf(w, "%s\n", cityLine)
			}
			if name := nteeName(o.NteeCode); name != "" {
				if cat := nteeCategory(o.NteeCode); cat != "" && cat != name {
					fmt.Fprintf(w, "NTEE %s — %s (%s)\n", o.NteeCode, name, cat)
				} else {
					fmt.Fprintf(w, "NTEE %s — %s\n", o.NteeCode, name)
				}
			}
			if o.Ruling != "" {
				fmt.Fprintf(w, "IRS ruling: %s\n", o.Ruling)
			}
			fmt.Fprintf(w, "\nFilings: %d with financial data, %d PDF-only\n", len(resp.FilingsWithData), len(resp.FilingsWithoutData))
			if latest != nil {
				fmt.Fprintf(w, "\nLatest 990 (FY %d):\n", latest.TaxPrdYr)
				fmt.Fprintf(w, "  Revenue:     %s\n", fmtUSD(latest.TotRevenue))
				fmt.Fprintf(w, "  Expenses:    %s\n", fmtUSD(latest.TotExpenses))
				fmt.Fprintf(w, "  Assets:      %s\n", fmtUSD(latest.TotAssets))
				fmt.Fprintf(w, "  Liabilities: %s\n", fmtUSD(latest.TotLiab))
				if latest.TotRevenue != nil && latest.TotExpenses != nil {
					net := *latest.TotRevenue - *latest.TotExpenses
					fmt.Fprintf(w, "  Net:         %s\n", fmtUSD(&net))
				}
				if latest.PdfURL != "" {
					fmt.Fprintf(w, "  PDF: %s\n", latest.PdfURL)
				}
			}
			return nil
		},
	}
	return cmd
}

func einDash(d string) string {
	if len(d) == 9 {
		return d[:2] + "-" + d[2:]
	}
	return d
}
