// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to net_investment_monthly.go. NSDL's
// /web/Reports/Archive.aspx is an ASP.NET WebForms page requiring a
// VIEWSTATE POST replay (see nsdl.FetchArchiveReport's doc comment) — the
// only endpoint in this CLI that needs one. It returns the same granular
// daily breakdown as Monthly.aspx/Latest.aspx, but for an arbitrary past
// date instead of only the current month, closing the gap those two
// leave for historical lookups (e.g. "daily data for Feb 2020").

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

// archiveReportingDateRe matches a real "DD-Mon-YYYY" per-day row.
// Archive.aspx (unlike Monthly.aspx/Latest.aspx) appends trailing
// "Total for <Month>"/"Total for <Year>" summary rows and a footnote
// paragraph whose rowspan-merged cell smears the same disclaimer text
// across every column — neither carries a real Reporting Date, so
// filtering on this pattern drops both artifact classes in one pass.
var archiveReportingDateRe = regexp.MustCompile(`^\d{2}-[A-Za-z]{3}-\d{4}$`)

func newNetInvestmentArchiveCmd(flags *rootFlags) *cobra.Command {
	var flagDate string

	cmd := &cobra.Command{
		Use:     "archive",
		Short:   "Historical daily FPI trends up to a given date, from NSDL's date-driven archive",
		Example: "  fpi-india-pp-cli net-investment archive --date 29022020 --json",
		Long: "Fetches NSDL's Archive.aspx report (gross purchases, gross sales, net investment, by asset class and route) for every " +
			"trading day from the 1st of --date's month up to and including --date itself. Unlike 'net-investment monthly'/'latest' " +
			"(current month only), this reaches arbitrary historical dates — e.g. --date 29022020 returns all of February 2020's " +
			"daily granular data. This is the only endpoint the CLI reaches via an ASP.NET WebForms VIEWSTATE POST replay rather " +
			"than a plain GET; a fresh token pair is fetched and submitted on every call.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagDate == "" {
				_ = cmd.Usage()
				return usageErrf("--date is required, in DDMMYYYY format, e.g. --date 29022020")
			}
			nsdlDate, ok := ddmmyyyyToArchiveDate(flagDate)
			if !ok {
				_ = cmd.Usage()
				return usageErrf("--date must be 8 digits in DDMMYYYY format, e.g. --date 29022020")
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "POST https://www.fpi.nsdl.co.in/web/Reports/Archive.aspx (date="+nsdlDate+")")
				fmt.Fprintln(cmd.OutOrStdout(), "\n(dry run - no request sent)")
				return nil
			}
			body, err := nsdl.FetchArchiveReport(cmd.Context(), nsdlDate)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recs, err := nsdl.ParseGenericRecords(body)
			if err != nil {
				return fmt.Errorf("parsing NSDL archive report: %w", err)
			}
			daily := make([]map[string]string, 0, len(recs))
			for _, rec := range recs {
				if archiveReportingDateRe.MatchString(rec["Reporting Date"]) {
					daily = append(daily, rec)
				}
			}
			data, err := json.Marshal(daily)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "End date to fetch, in DDMMYYYY format, e.g. 29022020 (returns the whole month up to this date)")
	return cmd
}

// ddmmyyyyToArchiveDate converts the CLI's DDMMYYYY --date flag format to
// NSDL Archive.aspx's expected "DD-Mon-YYYY" form field value (e.g.
// "29-Feb-2020"), reusing cdsl_diff.go's ddmmyyyyToReportDate for parsing
// and rollover validation.
func ddmmyyyyToArchiveDate(ddmmyyyy string) (string, bool) {
	t, ok := ddmmyyyyToReportDate(ddmmyyyy)
	if !ok {
		return "", false
	}
	return t.Format("02-Jan-2006"), true
}
