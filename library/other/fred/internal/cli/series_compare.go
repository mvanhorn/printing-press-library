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
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagStart, "start", "", "Start date YYYY-MM-DD (observation_start)")
	cmd.Flags().StringVar(&flagEnd, "end", "", "End date YYYY-MM-DD (observation_end)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Max observations per series (0 = API default)")
	return cmd
}
