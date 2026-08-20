// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to cdsl_reports_listing.go/cdsl_reports_snapshot.go.
// CDSL's /eservices/publications/ portal (FIIDaily, FIIMonthly) is a newer,
// plain server-rendered HTML surface with real per-day FPI figures — a much
// better source than the legacy ForeignPortInvestor.html + binary-XLS
// snapshot path those two files wrap. Structurally identical rowspan
// GridView-style table to NSDL's own reports, so nsdl.ParseGenericRecords
// (already used for NSDL's Latest.aspx) parses it unchanged.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

func newCdslReportsDailyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "daily",
		Short:       "Latest day's FPI daily trends by asset class and investment route",
		Example:     "  fpi-india-pp-cli cdsl-reports daily --json",
		Long:        "Fetches CDSL's current-day FPI investment breakdown (gross purchases, gross sales, net investment, by asset class and route) from the same real-time source NSDL cross-checks against. Only shows the latest reporting date; use 'cdsl-reports monthly' for the full current month.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "GET https://www.cdslindia.com/eservices/publications/FIIDaily")
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}
			body, err := nsdl.FetchStaticReport(cmd.Context(), "https://www.cdslindia.com", "/eservices/publications/FIIDaily")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recs, err := nsdl.ParseGenericRecords(body)
			if err != nil {
				return fmt.Errorf("parsing CDSL daily report: %w", err)
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

func newCdslReportsMonthlyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "monthly",
		Short:       "Current month's FPI daily trends, one row-group per day",
		Example:     "  fpi-india-pp-cli cdsl-reports monthly --json",
		Long:        "Fetches CDSL's current-month FPI investment breakdown, with one row-group per already-published day (gross purchases, gross sales, net investment, by asset class and route). CDSL resets this view at the start of each calendar month; use 'cdsl-reports daily' for just the latest date.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "GET https://www.cdslindia.com/eservices/publications/FIIMonthly")
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}
			body, err := nsdl.FetchStaticReport(cmd.Context(), "https://www.cdslindia.com", "/eservices/publications/FIIMonthly")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recs, err := nsdl.ParseGenericRecords(body)
			if err != nil {
				return fmt.Errorf("parsing CDSL monthly report: %w", err)
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
