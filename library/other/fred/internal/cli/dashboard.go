// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"github.com/spf13/cobra"
)

// dashboardIndicator pairs a series id with a human label for the snapshot.
type dashboardIndicator struct {
	SeriesID string
	Label    string
}

// headlineIndicators is the curated macro snapshot.
var headlineIndicators = []dashboardIndicator{
	{"UNRATE", "Unemployment rate (%)"},
	{"CPIAUCSL", "CPI, all urban consumers (index)"},
	{"GDP", "Gross domestic product ($B)"},
	{"FEDFUNDS", "Effective federal funds rate (%)"},
	{"DGS10", "10-year Treasury yield (%)"},
	{"PAYEMS", "Total nonfarm payrolls (thousands)"},
}

type dashboardRow struct {
	SeriesID string `json:"series_id"`
	Label    string `json:"label"`
	Date     string `json:"date"`
	Value    string `json:"value"`
	Error    string `json:"error,omitempty"`
}

type dashboardView struct {
	Indicators    []dashboardRow `json:"indicators"`
	FetchFailures []fetchFail    `json:"fetch_failures,omitempty"`
}

// pp:data-source live
func newNovelDashboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "dashboard",
		Short:       "Snapshot the headline U.S. macro indicators in one call",
		Long:        "Fetch the latest value of a curated set of headline U.S. indicators — unemployment, CPI, GDP, fed funds, 10-year Treasury, and nonfarm payrolls — and assemble them into a single macro snapshot. Existing FRED tools return one series per call; this fans out and aggregates.",
		Example:     "  fred-pp-cli dashboard --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := make([]dashboardRow, 0, len(headlineIndicators))
			failures := make([]fetchFail, 0)
			for _, ind := range headlineIndicators {
				view, err := fetchLatestObservation(cmd, flags, ind.SeriesID)
				if err != nil {
					failures = append(failures, fetchFail{SeriesID: ind.SeriesID, Error: err.Error()})
					rows = append(rows, dashboardRow{SeriesID: ind.SeriesID, Label: ind.Label, Error: err.Error()})
					continue
				}
				rows = append(rows, dashboardRow{
					SeriesID: ind.SeriesID,
					Label:    ind.Label,
					Date:     view.Date,
					Value:    view.Value,
				})
			}
			return flags.printJSON(cmd, dashboardView{Indicators: rows, FetchFailures: failures})
		},
	}
	return cmd
}
