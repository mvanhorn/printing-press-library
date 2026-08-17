// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

type cdslDiffView struct {
	RequestedDate string  `json:"requested_date"`
	CDSLDate      string  `json:"cdsl_reporting_date,omitempty"`
	CDSLTotal     float64 `json:"cdsl_net_investment_total_inr_cr,omitempty"`
	NSDLDate      string  `json:"nsdl_reporting_date,omitempty"`
	NSDLTotal     float64 `json:"nsdl_net_investment_total_inr_cr,omitempty"`
	Match         bool    `json:"match"`
	Note          string  `json:"note"`
}

// cdslReportTotal fetches a CDSL/NSDL daily FPI report and returns the
// "Total" row's Reporting Date and Net Investment (Rs. Crore) figure.
// Both sources render the same rowspan GridView shape with identical
// column names ("Reporting Date", "Investment Route", "Net Investment
// (Rs. Crore)"), so one helper covers both.
func cdslReportTotal(ctx context.Context, baseURL, path string) (date string, total float64, err error) {
	body, ferr := nsdl.FetchStaticReport(ctx, baseURL, path)
	if ferr != nil {
		return "", 0, ferr
	}
	recs, perr := nsdl.ParseGenericRecords(body)
	if perr != nil {
		return "", 0, perr
	}
	for _, rec := range recs {
		if !strings.EqualFold(strings.TrimSpace(rec["Investment Route"]), "Total") {
			continue
		}
		n, ok := nsdl.ParseNumber(rec["Net Investment (Rs. Crore)"])
		if !ok {
			continue
		}
		return strings.TrimSpace(rec["Reporting Date"]), n, nil
	}
	return "", 0, fmt.Errorf("no Total row found in report")
}

// parseReportDate parses the "DD-Mon-YYYY" / "DD-MON-YYYY" reporting-date
// format both NSDL and CDSL emit (e.g. "10-Jul-2026", "10-JUL-2026").
func parseReportDate(s string) (time.Time, bool) {
	for _, layout := range []string{"02-Jan-2006", "02-JAN-2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ddmmyyyyToReportDate converts the CLI's DDMMYYYY --date flag format to
// the same day the report-date fields use, for equality comparison.
func ddmmyyyyToReportDate(ddmmyyyy string) (time.Time, bool) {
	if len(ddmmyyyy) != 8 {
		return time.Time{}, false
	}
	day, err1 := strconv.Atoi(ddmmyyyy[0:2])
	month, err2 := strconv.Atoi(ddmmyyyy[2:4])
	year, err3 := strconv.Atoi(ddmmyyyy[4:8])
	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}, false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// time.Date silently normalizes an out-of-range day (e.g. 31-Apr) into
	// the following month instead of erroring; reject those rather than
	// let callers treat a rolled-over date as the one the user asked for.
	if t.Day() != day || int(t.Month()) != month || t.Year() != year {
		return time.Time{}, false
	}
	return t, true
}

func newNovelCdslDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDate string

	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Compare NSDL's and CDSL's latest daily net-investment totals and flag any mismatch.",
		Example: "  fpi-india-pp-cli cdsl diff --date 10072026 --json",
		Long: "Fetches CDSL's and NSDL's current daily FPI net-investment totals (CDSL's /eservices/publications/FIIDaily and NSDL's " +
			"Latest.aspx) and reconciles them against each other and against the requested date. Both sources only expose their most " +
			"recently published day through this path — --date is checked against what's actually available and the command reports " +
			"a clear mismatch note rather than silently comparing the wrong days. Use 'cdsl-reports monthly' for CDSL's fuller current-month " +
			"history, or 'cdsl-reports snapshot <date>' for the legacy per-day binary XLS archive.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would reconcile NSDL/CDSL daily net-investment totals for a date")
				return nil
			}
			if flagDate == "" {
				_ = cmd.Usage()
				return usageErrf("--date is required, in DDMMYYYY format, e.g. --date 10072026")
			}
			requested, ok := ddmmyyyyToReportDate(flagDate)
			if !ok {
				_ = cmd.Usage()
				return usageErrf("--date must be 8 digits in DDMMYYYY format, e.g. --date 10072026")
			}

			view := cdslDiffView{RequestedDate: flagDate}

			cdslDateStr, cdslTotal, cdslErr := cdslReportTotal(cmd.Context(), "https://www.cdslindia.com", "/eservices/publications/FIIDaily")
			if cdslErr == nil {
				view.CDSLDate = cdslDateStr
				view.CDSLTotal = cdslTotal
			}

			nsdlDateStr, nsdlTotal, nsdlErr := cdslReportTotal(cmd.Context(), "https://www.fpi.nsdl.co.in", "/web/Reports/Latest.aspx")
			if nsdlErr == nil {
				view.NSDLDate = nsdlDateStr
				view.NSDLTotal = nsdlTotal
			}

			if cdslErr != nil && nsdlErr != nil {
				return fmt.Errorf("fetching CDSL daily report: %w; fetching NSDL latest report: %w", cdslErr, nsdlErr)
			}

			cdslDate, cdslDateOK := parseReportDate(cdslDateStr)
			nsdlDate, nsdlDateOK := parseReportDate(nsdlDateStr)

			switch {
			case cdslErr != nil:
				view.Note = fmt.Sprintf("CDSL daily report unavailable (%v); showing NSDL only", cdslErr)
			case nsdlErr != nil:
				view.Note = fmt.Sprintf("NSDL latest report unavailable (%v); showing CDSL only", nsdlErr)
			case !cdslDateOK || !nsdlDateOK:
				view.Note = "could not parse one or both sources' reporting dates for comparison"
			case !cdslDate.Equal(nsdlDate):
				view.Note = fmt.Sprintf("NSDL and CDSL are reporting different latest dates (NSDL: %s, CDSL: %s) — each source only exposes its own most recent day through this path, not arbitrary history", nsdlDateStr, cdslDateStr)
			default:
				diff := cdslTotal - nsdlTotal
				if diff < 0 {
					diff = -diff
				}
				view.Match = diff < 0.01
				if view.Match {
					view.Note = fmt.Sprintf("NSDL and CDSL agree on %s: net investment %.2f Rs. Crore", nsdlDateStr, nsdlTotal)
				} else {
					view.Note = fmt.Sprintf("NSDL and CDSL both report %s but disagree: NSDL %.2f vs CDSL %.2f Rs. Crore (delta %.2f)", nsdlDateStr, nsdlTotal, cdslTotal, diff)
				}
			}

			if cdslDateOK && !cdslDate.Equal(requested) {
				view.Note += fmt.Sprintf("; requested date %s does not match CDSL's latest available date %s", flagDate, cdslDateStr)
			}
			if nsdlDateOK && !nsdlDate.Equal(requested) {
				view.Note += fmt.Sprintf("; requested date %s does not match NSDL's latest available date %s", flagDate, nsdlDateStr)
			}

			return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to check in DDMMYYYY format, e.g. 10072026")
	return cmd
}
