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

func newNPFilingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "filings <ein-or-name>",
		Short:       "List all Form 990 filings for an organization (financial trend by year)",
		Long:        "List all Form 990 filings for an organization (financial trend by year).\nAccepts an EIN (with or without dash) or a nonprofit name (auto-resolves to the top match).",
		Example:     "  nonprofit-explorer-pp-cli filings 530196605\n  nonprofit-explorer-pp-cli filings \"american red cross\"",
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
			if flags.asJSON {
				out := map[string]any{
					"ein":                  einDash(ein),
					"name":                 resp.Organization.Name,
					"filings_with_data":    filings,
					"filings_without_data": resp.FilingsWithoutData,
				}
				if resolved != nil {
					out["resolved"] = resolved
				}
				return printJSONLive(cmd, flags, out)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (EIN %s)\n%d filings with financial data\n\n", resp.Organization.Name, einDash(ein), len(filings))
			rows := make([][]string, 0, len(filings))
			for _, f := range filings {
				rows = append(rows, []string{
					strconv.Itoa(f.TaxPrdYr),
					fmtUSD(f.TotRevenue),
					fmtUSD(f.TotExpenses),
					fmtUSD(f.TotAssets),
					fmtUSD(f.TotLiab),
				})
			}
			if err := flags.printTable(cmd, []string{"YEAR", "REVENUE", "EXPENSES", "ASSETS", "LIABILITIES"}, rows); err != nil {
				return err
			}
			if len(resp.FilingsWithoutData) > 0 {
				fmt.Fprintf(w, "\n(+%d older PDF-only filings without parsed data)\n", len(resp.FilingsWithoutData))
			}
			return nil
		},
	}
	return cmd
}
