// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to net_investment_latest.go. NSDL's
// /web/Reports/Monthly.aspx renders the current calendar month's full
// daily granular breakdown (gross purchases/sales by asset class and
// investment route) — the same rowspan GridView shape as Latest.aspx, just
// one row-group per already-published day this month instead of only the
// latest one. This is the primary source syncCDSLReports pulls into the
// local store; this command is the equivalent live read.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

func newNetInvestmentMonthlyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "monthly",
		Short:       "Current month's FPI daily trends, one row-group per day",
		Example:     "  fpi-india-pp-cli net-investment monthly --json",
		Long:        "Fetches NSDL's current-month FPI investment breakdown (Monthly.aspx), with one row-group per already-published day this month (gross purchases, gross sales, net investment, by asset class and route). NSDL resets this view at the start of each calendar month; use 'net-investment latest' for just the most recent fortnight snapshot, or 'sync --resources cdsl_reports' to build a rolling local history across month boundaries.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "GET /web/Reports/Monthly.aspx")
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err := c.Get(cmd.Context(), "/web/Reports/Monthly.aspx", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recs, err := nsdl.ParseGenericRecords(body)
			if err != nil {
				return fmt.Errorf("parsing NSDL monthly report: %w", err)
			}
			data, err := json.Marshal(recs)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
		},
	}
	return cmd
}
