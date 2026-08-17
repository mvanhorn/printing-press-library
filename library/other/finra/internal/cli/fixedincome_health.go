// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// fixedincomeDatasetSpec names one of the five datasets joined by
// `fixedincome health`. dateField is the confirmed partition field used to
// bound each dataset to the trailing window: the four daily breadth/
// sentiment datasets use tradeReportDate, while TRACE Monthly Volume is
// monthly and uses beginningOfMonth instead — a plain "date"-suffixed key
// scan would miss it.
type fixedincomeDatasetSpec struct {
	key       string
	group     string
	name      string
	dateField string
}

// pp:data-source live
func newNovelFixedincomeHealthCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "One market-condition snapshot joining TRACE, Corporate/Agency Debt Market Breadth and Sentiment",
		Long: "One market-condition report joining TRACE Monthly Volume, Corporate Market Breadth, Corporate\n" +
			"Market Sentiment, Agency Market Breadth, and Agency Market Sentiment over the trailing --since\n" +
			"window (default 7d).\n\n" +
			"TRACE Monthly Volume is a market-wide monthly aggregate with no CUSIP or per-bond identifier —\n" +
			"per-CUSIP TRACE data requires a higher entitlement tier than this dataset.",
		Example:     "--since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--since=90d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch TRACE Monthly Volume plus corporate/agency market breadth and sentiment datasets for the trailing %s\n", flagSince)
				return nil
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "7d"
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			start := now.Add(-window)

			specs := []fixedincomeDatasetSpec{
				{key: "trace", group: "FIXEDINCOMEMARKET", name: "TRACEMONTHLYVOLUME", dateField: "beginningOfMonth"},
				{key: "corporate_breadth", group: "FIXEDINCOMEMARKET", name: "CORPORATEMARKETBREADTH", dateField: "tradeReportDate"},
				{key: "corporate_sentiment", group: "FIXEDINCOMEMARKET", name: "CORPORATEMARKETSENTIMENT", dateField: "tradeReportDate"},
				{key: "agency_breadth", group: "FIXEDINCOMEMARKET", name: "AGENCYMARKETBREADTH", dateField: "tradeReportDate"},
				{key: "agency_sentiment", group: "FIXEDINCOMEMARKET", name: "AGENCYMARKETSENTIMENT", dateField: "tradeReportDate"},
			}

			results, errs := cliutil.FanoutRun(ctx, specs,
				func(s fixedincomeDatasetSpec) string { return s.key },
				func(fctx context.Context, s fixedincomeDatasetSpec) (fixedincomeDatasetResult, error) {
					return fetchFixedincomeDataset(fctx, c, s, start, now)
				},
			)

			view := fixedincomeHealthView{
				Since:    since,
				Datasets: map[string]fixedincomeDatasetResult{},
			}
			for _, r := range results {
				view.Datasets[r.Source] = r.Value
			}
			for _, e := range errs {
				view.FetchFailures = append(view.FetchFailures, fixedincomeFetchFailure{Dataset: e.Source, Error: e.Err.Error()})
			}
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, s := range specs {
					if r, ok := view.Datasets[s.key]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "%s: %d record(s) since %s\n", s.key, r.RecordCount, since)
					}
				}
				for _, f := range view.FetchFailures {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: fetch failed: %s\n", f.Dataset, f.Error)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "How far back to look across all five fixed-income datasets (e.g. 24h, 7d, 1w)")
	return cmd
}

type fixedincomeDatasetResult struct {
	Group       string           `json:"group"`
	Name        string           `json:"name"`
	RecordCount int              `json:"record_count"`
	Records     []map[string]any `json:"records"`
}

type fixedincomeFetchFailure struct {
	Dataset string `json:"dataset"`
	Error   string `json:"error"`
}

type fixedincomeHealthView struct {
	Since         string                              `json:"since"`
	Datasets      map[string]fixedincomeDatasetResult `json:"datasets"`
	FetchFailures []fixedincomeFetchFailure           `json:"fetch_failures,omitempty"`
}

// fetchFixedincomeDataset queries one dataset bounded server-side to
// [start, end] via s.dateField (the dataset's confirmed partition field), a
// live probe having confirmed that an unfiltered fetch returns an arbitrary
// slice (often years-old) rather than anything sorted or scoped to recent
// dates. Returns the record count and records so the caller can inspect
// them, rather than computing a fabricated week-over-week delta from a
// synthesized field.
func fetchFixedincomeDataset(ctx context.Context, c *client.Client, s fixedincomeDatasetSpec, start, end time.Time) (fixedincomeDatasetResult, error) {
	path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", s.group), "name", s.name)
	body := map[string]any{
		"dateRangeFilters": dateRangeFilter(s.dateField, start, end),
		"limit":            1000,
	}
	data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
	if err != nil {
		return fixedincomeDatasetResult{}, err
	}
	records, err := parseDatasetRecords(data)
	if err != nil {
		return fixedincomeDatasetResult{}, fmt.Errorf("parsing %s/%s response: %w", s.group, s.name, err)
	}
	return fixedincomeDatasetResult{Group: s.group, Name: s.name, RecordCount: len(records), Records: records}, nil
}
