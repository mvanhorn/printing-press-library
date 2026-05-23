// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored forfettario turnover meter.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type turnoverReport struct {
	Year          int     `json:"year"`
	FiscalLabel   string  `json:"fiscal_label"`
	TurnoverTotal float64 `json:"turnover_total"`
	TurnoverLimit float64 `json:"turnover_limit"`
	PctUsed       float64 `json:"pct_used"`
	DaysElapsed   int     `json:"days_elapsed"`
	RunRatePerDay float64 `json:"run_rate_per_day"`
	DaysToLimit   *int    `json:"days_to_limit"`
	ProjectedEOY  float64 `json:"projected_eoy"`
	Warning       *string `json:"warning"`
}

func newTurnoverCmd(flags *rootFlags) *cobra.Command {
	year := currentYear()
	cmd := &cobra.Command{
		Use:   "turnover",
		Short: "Show forfettario turnover against the annual limit",
		Long: `Show the synced forfettario turnover meter for a fiscal year.

The command reads local invoices and fiscal_year metadata populated by sync.
For forfettari it treats non-draft invoice taxable amounts as compenso.`,
		Example: `  partitaiva24-pp-cli turnover --year 2026
  partitaiva24-pp-cli turnover --year 2026 --json --select turnover_total,pct_used,warning`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			var total sql.NullFloat64
			if err := s.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(SUM(COALESCE(taxable, total)), 0) FROM invoices WHERE strftime('%Y', date) = ? AND COALESCE(status, '') != 'draft'`,
				fmt.Sprintf("%04d", year),
			).Scan(&total); err != nil {
				return err
			}
			var label sql.NullString
			var limit sql.NullFloat64
			_ = s.DB().QueryRowContext(cmd.Context(),
				`SELECT label, turnover_limit FROM fiscal_year WHERE year = ? ORDER BY synced_at DESC LIMIT 1`,
				year,
			).Scan(&label, &limit)

			// fiscal_year endpoint is a GET-by-year, not a list — sync may not
			// populate it correctly. When the local row is missing or empty,
			// hit the live endpoint so turnover can compute against the real
			// regime cap (e.g. 85k for forfettari 2026).
			if !limit.Valid || limit.Float64 == 0 {
				if c, ferr := flags.newClient(); ferr == nil {
					if body, gerr := c.Get(fmt.Sprintf("/user/fiscal_year/%d", year), nil); gerr == nil {
						var fy struct {
							Label         string  `json:"label"`
							TurnoverLimit float64 `json:"turnover_limit"`
						}
						if json.Unmarshal(body, &fy) == nil {
							if fy.Label != "" {
								label = sql.NullString{String: fy.Label, Valid: true}
							}
							if fy.TurnoverLimit > 0 {
								limit = sql.NullFloat64{Float64: fy.TurnoverLimit, Valid: true}
							}
						}
					}
				}
			}

			now := time.Now()
			start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
			elapsed := 365
			if now.Year() == year {
				elapsed = int(now.Sub(start).Hours()/24) + 1
				if elapsed < 1 {
					elapsed = 1
				}
				if elapsed > 365 {
					elapsed = 365
				}
			} else if now.Year() < year {
				elapsed = 1
			}
			turnover := nullableFloat(total)
			turnoverLimit := nullableFloat(limit)
			runRate := turnover / float64(elapsed)
			projected := runRate * 365
			var pct float64
			if turnoverLimit > 0 {
				pct = turnover / turnoverLimit * 100
			}
			var daysToLimit *int
			if runRate > 0 && turnoverLimit > turnover {
				d := int((turnoverLimit - turnover) / runRate)
				daysToLimit = &d
			}
			var warning *string
			if turnoverLimit > 0 && projected > turnoverLimit {
				w := "Forecasted to exceed limit before EOY"
				warning = &w
			}
			if pct >= 80 {
				w := "Above 80% of limit"
				warning = &w
			}
			report := turnoverReport{
				Year: year, FiscalLabel: nullableString(label), TurnoverTotal: money2(turnover),
				TurnoverLimit: money2(turnoverLimit), PctUsed: pct1(pct), DaysElapsed: elapsed,
				RunRatePerDay: money2(runRate), DaysToLimit: daysToLimit, ProjectedEOY: money2(projected), Warning: warning,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "year: %d\nfiscal_label: %s\nturnover_total: %.2f\nturnover_limit: %.2f\npct_used: %.1f\n", report.Year, report.FiscalLabel, report.TurnoverTotal, report.TurnoverLimit, report.PctUsed)
			fmt.Fprintf(w, "days_elapsed: %d\nrun_rate_per_day: %.2f\n", report.DaysElapsed, report.RunRatePerDay)
			if report.DaysToLimit != nil {
				fmt.Fprintf(w, "days_to_limit: %d\n", *report.DaysToLimit)
			} else {
				fmt.Fprintln(w, "days_to_limit: null")
			}
			fmt.Fprintf(w, "projected_eoy: %.2f\n", report.ProjectedEOY)
			if report.Warning != nil {
				fmt.Fprintf(w, "warning: %s\n", red(*report.Warning))
			} else {
				fmt.Fprintln(w, "warning: null")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&year, "year", year, "Fiscal year")
	return cmd
}
