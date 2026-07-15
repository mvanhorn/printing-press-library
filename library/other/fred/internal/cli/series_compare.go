// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// compareRow is one date with the value of each requested series at that date.
type compareRow struct {
	Date   string            `json:"date"`
	Values map[string]string `json:"values"`
}

type compareView struct {
	Series        []string     `json:"series"`
	Rows          []compareRow `json:"rows"`
	FetchFailures []fetchFail  `json:"fetch_failures,omitempty"`
}

type fetchFail struct {
	SeriesID string `json:"series_id"`
	Error    string `json:"error"`
}

// pp:data-source live
func newNovelSeriesCompareCmd(flags *rootFlags) *cobra.Command {
	var flagStart string
	var flagEnd string
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "compare <series_id> <series_id> [series_id...]",
		Short:       "Align observations for multiple series by date for comparison",
		Long:        "Pull observations for two or more series and align them by date into a single table or JSON structure — ready for correlation or side-by-side reading. FRED has no multi-series endpoint; this command joins the series locally.",
		Example:     "  fred-pp-cli series compare UNRATE CPIAUCSL --start 2020-01-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two series ids are required, e.g. UNRATE CPIAUCSL"))
			}

			// Build the HTTP client and bound context once, then reuse across the
			// fan-out so we don't reconstruct a client per series.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fetch := func(seriesID string) ([]fredObservation, error) {
				params := map[string]string{
					"series_id":  seriesID,
					"file_type":  "json",
					"sort_order": "asc",
				}
				if flagStart != "" {
					params["observation_start"] = flagStart
				}
				if flagEnd != "" {
					params["observation_end"] = flagEnd
				}
				if flagLimit > 0 {
					params["limit"] = fmt.Sprintf("%d", flagLimit)
				}
				data, err := c.Get(ctx, "/series/observations", params)
				if err != nil {
					return nil, classifyAPIError(err, flags)
				}
				var env observationsEnvelope
				if err := json.Unmarshal(data, &env); err != nil {
					return nil, apiErr(fmt.Errorf("parsing observations for %s: %w", seriesID, err))
				}
				return env.Observations, nil
			}

			byDate := map[string]map[string]string{}
			failures := make([]fetchFail, 0)
			for _, id := range args {
				obs, err := fetch(id)
				if err != nil {
					failures = append(failures, fetchFail{SeriesID: id, Error: err.Error()})
					continue
				}
				for _, o := range obs {
					if byDate[o.Date] == nil {
						byDate[o.Date] = map[string]string{}
					}
					byDate[o.Date][id] = o.Value
				}
			}

			dates := make([]string, 0, len(byDate))
			for d := range byDate {
				dates = append(dates, d)
			}
			sort.Strings(dates)

			rows := make([]compareRow, 0, len(dates))
			for _, d := range dates {
				rows = append(rows, compareRow{Date: d, Values: byDate[d]})
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d series failed to fetch; comparison covers the remaining %d\n", len(failures), len(args), len(args)-len(failures))
			}

			view := compareView{Series: args, Rows: rows, FetchFailures: failures}
			// --csv: the compare view is nested ({series, rows:[{date, values{}}]}),
			// which the array-based CSV renderer cannot flatten on its own — so the
			// flag was silently ignored and JSON was emitted regardless. Flatten to
			// one row per date (a column per series) and route through the shared
			// filtered writer so --csv, --select, --compact, and --quiet behave
			// identically to every other command. Default JSON output is unchanged.
			if flags.csv {
				// Emit columns only for series that actually returned data. A failed
				// series is surfaced on stderr and in the JSON fetch_failures; in CSV
				// it would otherwise appear as a silent column of blanks that a caller
				// could mistake for "no observations in range".
				okSeries := make([]string, 0, len(args))
				for _, id := range args {
					failed := false
					for _, f := range failures {
						if f.SeriesID == id {
							failed = true
							break
						}
					}
					if !failed {
						okSeries = append(okSeries, id)
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), flattenCompareRows(okSeries, rows), flags)
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagStart, "start", "", "Start date YYYY-MM-DD (observation_start)")
	cmd.Flags().StringVar(&flagEnd, "end", "", "End date YYYY-MM-DD (observation_end)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max observations per series (0 = API default)")
	return cmd
}

// flattenCompareRows turns the nested compare view into a flat table — one row
// per date with a column per requested series — so the shared array-based CSV
// renderer can emit it. A series with no value at a given date becomes an empty
// string in that row.
func flattenCompareRows(series []string, rows []compareRow) []map[string]string {
	flat := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		m := map[string]string{"date": r.Date}
		for _, id := range series {
			m[id] = r.Values[id]
		}
		flat = append(flat, m)
	}
	return flat
}
