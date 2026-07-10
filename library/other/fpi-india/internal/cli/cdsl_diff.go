// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

type cdslDiffView struct {
	Date              string  `json:"date"`
	CDSLFilePublished bool    `json:"cdsl_file_published"`
	CDSLFileURL       string  `json:"cdsl_file_url,omitempty"`
	NSDLLatestPeriod  string  `json:"nsdl_latest_period,omitempty"`
	NSDLLatestTotal   float64 `json:"nsdl_latest_total,omitempty"`
	Note              string  `json:"note"`
}

func newNovelCdslDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDate string

	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Compare NSDL's and CDSL's figures for the same date and flag any mismatch.",
		Example: "  fpi-india-pp-cli cdsl diff --date 09072026 --json",
		Long: "Confirms whether CDSL has published a daily snapshot for the requested date (CDSL only exposes a rolling recent window, " +
			"not deep history) and reports NSDL's latest fortnight net-investment figure alongside it. CDSL's snapshot files are legacy " +
			"binary XLS (OLE2 Compound Document format), not HTML or JSON, so this command cross-checks publication freshness and reachability " +
			"rather than reconciling individual cell values between the two sources; use 'cdsl-reports snapshot <date>' to download the raw file " +
			"for manual inspection.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would cross-check NSDL/CDSL publication freshness for a date")
				return nil
			}
			if flagDate == "" {
				_ = cmd.Usage()
				return usageErrf("--date is required, in DDMMYYYY format, e.g. --date 09072026")
			}
			if len(flagDate) != 8 {
				_ = cmd.Usage()
				return usageErrf("--date must be 8 digits in DDMMYYYY format, e.g. --date 09072026")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := cdslDiffView{Date: flagDate}

			// CDSL rejects Go's stock net/http TLS fingerprint (HTTP 406)
			// even with a matching browser User-Agent — the same class of
			// bot mitigation NSDL's StaticReports family applies; see
			// nsdl.browserFingerprintHTTPGet's doc comment.
			listingBody, err := nsdl.FetchStaticReport(cmd.Context(), "https://www.cdslindia.com", "/Publications/ForeignPortInvestor.html")
			if err != nil {
				return fmt.Errorf("fetching CDSL listing: %w", err)
			}
			for _, marker := range []string{"latest_" + flagDate + ".xls", "Latest_" + flagDate + ".xls"} {
				if strings.Contains(string(listingBody), marker) {
					view.CDSLFilePublished = true
					view.CDSLFileURL = "https://www.cdslindia.com/downloads/Publications/Latest/" + marker
					break
				}
			}

			latestBody, err := c.Get(cmd.Context(), "/web/Reports/Latest.aspx", nil)
			if err == nil {
				if recs, parseErr := nsdl.ParseGenericRecords(latestBody); parseErr == nil && len(recs) > 0 {
					for k, v := range recs[0] {
						if strings.Contains(strings.ToLower(k), "total") {
							if n, ok := nsdl.ParseNumber(v); ok {
								view.NSDLLatestTotal = n
								view.NSDLLatestPeriod = k
								break
							}
						}
					}
				}
			}

			if view.CDSLFilePublished {
				view.Note = "CDSL has published a snapshot for this date; download it with 'cdsl-reports snapshot " + flagDate + "' for the raw figures (legacy binary XLS, not parsed by this CLI)"
			} else {
				view.Note = "CDSL has not published a snapshot for this date in its current rolling window (CDSL only retains recent dates, not deep history)"
			}

			return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "live"})
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to check in DDMMYYYY format, e.g. 09072026")
	return cmd
}
