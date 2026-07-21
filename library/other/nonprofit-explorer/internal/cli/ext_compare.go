// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extension for the Nonprofit Explorer CLI (recorded in
// .printing-press-patches/ext-nonprofit-commands.md). Shared helpers live in
// ext_nonprofit.go; registration is in root.go.

package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newNPCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <ein-or-name> <ein-or-name> [...]",
		Short:       "Side-by-side latest-990 comparison of two or more organizations",
		Long:        "Side-by-side latest-990 comparison of two or more organizations.\nEach argument may be an EIN (with or without dash) or a nonprofit name (auto-resolves to the top match).",
		Example:     "  nonprofit-explorer-pp-cli compare 530196605 874084202\n  nonprofit-explorer-pp-cli compare \"american red cross\" \"marcella foundation\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type row struct {
				EIN      string      `json:"ein"`
				Name     string      `json:"name"`
				State    string      `json:"state"`
				Ntee     string      `json:"ntee_code"`
				FY       int         `json:"fiscal_year"`
				Latest   *npFiling   `json:"latest_filing"`
				Resolved *resolution `json:"resolved,omitempty"`
			}
			var results []row
			for _, a := range args {
				ein, resolved, err := resolveEINOrName(cmd.Context(), c, flags, cmd, a)
				if err != nil {
					return err
				}
				resp, err := fetchOrg(cmd.Context(), c, ein)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				lf := latestFiling(resp.FilingsWithData)
				r := row{EIN: einDash(ein), Name: resp.Organization.Name, State: resp.Organization.State, Ntee: resp.Organization.NteeCode, Latest: lf, Resolved: resolved}
				if lf != nil {
					r.FY = lf.TaxPrdYr
				}
				results = append(results, r)
			}
			if flags.asJSON {
				return printJSONLive(cmd, flags, map[string]any{"organizations": results})
			}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rev, exp, ast := (*float64)(nil), (*float64)(nil), (*float64)(nil)
				fy := "—"
				if r.Latest != nil {
					rev, exp, ast = r.Latest.TotRevenue, r.Latest.TotExpenses, r.Latest.TotAssets
					fy = strconv.Itoa(r.FY)
				}
				name := r.Name
				if len(name) > 32 {
					name = name[:31] + "…"
				}
				rows = append(rows, []string{name, r.State, r.Ntee, fy, fmtUSD(rev), fmtUSD(exp), fmtUSD(ast)})
			}
			return flags.printTable(cmd, []string{"NAME", "ST", "NTEE", "FY", "REVENUE", "EXPENSES", "ASSETS"}, rows)
		},
	}
	return cmd
}
