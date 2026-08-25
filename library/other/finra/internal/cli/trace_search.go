// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelTraceSearchCmd(flags *rootFlags) *cobra.Command {
	var flagSubProduct string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "TRACE monthly aggregate volume lookup, optionally filtered by product category",
		Long: "TRACE monthly aggregate volume/trade-count records over the trailing --since window\n" +
			"(default 90d), optionally narrowed to --sub-product.\n\n" +
			"A basic-tier FINRA credential's only accessible TRACE dataset (TRACE Monthly Volume) is a\n" +
			"market-wide monthly aggregate with no CUSIP, symbol, or bond-identifier field at all —\n" +
			"per-CUSIP trade-level TRACE data requires a higher entitlement tier. Use this for raw\n" +
			"monthly aggregate lookups; use 'trace liquidity' for a computed month-over-month trend\n" +
			"instead.",
		Example:     "--sub-product CORP --since 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--sub-product=CORP"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s monthly records matching sub-product=%q over the trailing %s\n", flagGroup, flagName, flagSubProduct, flagSince)
				return nil
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "90d"
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
			path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", flagGroup), "name", flagName)
			// beginningOfMonth is this dataset's confirmed partition field; a
			// live probe confirmed an unfiltered fetch returns an arbitrary
			// slice (the earliest available months) rather than anything
			// scoped to recent dates, so the window is bounded server-side.
			body := map[string]any{
				"dateRangeFilters": dateRangeFilter("beginningOfMonth", start, now),
				"limit":            1000,
			}
			if strings.TrimSpace(flagSubProduct) != "" {
				// subProductCode is this dataset's confirmed categorical
				// field, also confirmed to filter server-side.
				body["compareFilters"] = equalCompareFilter("subProductCode", flagSubProduct)
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			records, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}

			matches := filterTraceMonthlyRecords(records, flagSubProduct, start, now)

			view := traceSearchView{
				Since:   since,
				Records: matches,
				Count:   len(matches),
			}
			if strings.TrimSpace(flagSubProduct) != "" {
				view.SubProduct = flagSubProduct
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d monthly record(s) over the trailing %s\n", view.Count, view.Since)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSubProduct, "sub-product", "", "Product category (subProductCode) to narrow monthly TRACE records to; omit to return all")
	cmd.Flags().StringVar(&flagSince, "since", "90d", "How far back to look for monthly TRACE records (e.g. 30d, 90d, 6mo)")
	cmd.Flags().StringVar(&flagGroup, "group", "FIXEDINCOMEMARKET", "Dataset group for TRACE Monthly Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "TRACEMONTHLYVOLUME", "Dataset name for TRACE Monthly Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type traceSearchView struct {
	Since      string           `json:"since"`
	SubProduct string           `json:"sub_product,omitempty"`
	Records    []map[string]any `json:"records"`
	Count      int              `json:"count"`
}

// filterTraceMonthlyRecords keeps TRACE Monthly Volume records whose
// beginningOfMonth (the dataset's confirmed partition field) falls within
// [start, end], optionally narrowed to an exact case-insensitive
// subProductCode match. Shared by 'trace search' and 'trace liquidity'.
func filterTraceMonthlyRecords(records []map[string]any, subProduct string, start, end time.Time) []map[string]any {
	want := strings.ToLower(strings.TrimSpace(subProduct))
	var out []map[string]any
	for _, rec := range records {
		if want != "" {
			sp, ok := rec["subProductCode"].(string)
			if !ok || strings.ToLower(sp) != want {
				continue
			}
		}
		d, ok := recordDateAtField(rec, "beginningOfMonth")
		if !ok || d.Before(start) || d.After(end) {
			continue
		}
		out = append(out, rec)
	}
	return out
}
