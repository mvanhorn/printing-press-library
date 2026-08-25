// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelTraceLiquidityCmd(flags *rootFlags) *cobra.Command {
	var flagSubProduct string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "liquidity",
		Short: "Month-over-month TRACE volume/trade-count trend, optionally scoped to a product category",
		Long: "Month-over-month trend in TRACE Monthly Volume trade count and volume over the trailing\n" +
			"--since window (default 180d), optionally scoped to --sub-product. Omit --sub-product for a\n" +
			"market-wide trend across all product categories.\n\n" +
			"A basic-tier FINRA credential's only accessible TRACE dataset (TRACE Monthly Volume) is a\n" +
			"market-wide monthly aggregate with no CUSIP, symbol, or bond-identifier field at all — a\n" +
			"per-bond liquidity trend is not computable without a higher entitlement tier for per-CUSIP\n" +
			"TRACE data. This instead trends totalTradeCount/totalVolumeQuantity for a product category\n" +
			"month over month, which nothing else in this CLI computes.",
		Example:     "--sub-product CORP --since 180d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--sub-product=CORP"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s monthly records matching sub-product=%q over the trailing %s and compute a trend\n", flagGroup, flagName, flagSubProduct, flagSince)
				return nil
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "180d"
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
			view := traceLiquidityView{
				Since:       since,
				RecordCount: len(matches),
			}
			if strings.TrimSpace(flagSubProduct) != "" {
				view.SubProduct = flagSubProduct
			}
			view.AvgTradesPerMonth, view.LiquidityTrend, view.Note = computeLiquidityTrend(matches, start, now)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d monthly record(s) over %s, avg %.2f trades/month, trend=%s\n", view.RecordCount, view.Since, view.AvgTradesPerMonth, view.LiquidityTrend)
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSubProduct, "sub-product", "", "Product category (subProductCode) to scope the trend to; omit for a market-wide trend")
	cmd.Flags().StringVar(&flagSince, "since", "180d", "How far back to look for monthly TRACE records (e.g. 90d, 180d, 1y)")
	cmd.Flags().StringVar(&flagGroup, "group", "FIXEDINCOMEMARKET", "Dataset group for TRACE Monthly Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "TRACEMONTHLYVOLUME", "Dataset name for TRACE Monthly Volume (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type traceLiquidityView struct {
	SubProduct        string  `json:"sub_product,omitempty"`
	Since             string  `json:"since"`
	RecordCount       int     `json:"record_count"`
	AvgTradesPerMonth float64 `json:"avg_trades_per_month"`
	LiquidityTrend    string  `json:"liquidity_trend"`
	Note              string  `json:"note,omitempty"`
}

// computeLiquidityTrend derives average trades/month and a coarse trend
// signal from matched monthly records. Fewer than 2 monthly records is
// treated as insufficient data. Unlike per-trade TRACE data, every TRACE
// Monthly Volume record always carries beginningOfMonth, so there is no
// "undated records" case to account for. The trend buckets records by
// calendar month relative to the midpoint of [windowStart, windowEnd], then
// compares total trade count and total volume (both confirmed fields)
// between the two buckets: fewer trades in the second half combined with a
// flat-or-larger volume flags "deteriorating" (the same volume moving in
// fewer, larger months); more trades in the second half flags "improving";
// otherwise "stable".
func computeLiquidityTrend(records []map[string]any, windowStart, windowEnd time.Time) (avgPerMonth float64, trend string, note string) {
	if len(records) < 2 {
		return 0, "insufficient_data", "fewer than 2 monthly records in the window; not enough data to compute a trend"
	}

	months := map[string]bool{}
	for _, rec := range records {
		if d, ok := recordDateAtField(rec, "beginningOfMonth"); ok {
			months[d.Format("2006-01")] = true
		}
	}
	n := len(months)
	if n == 0 {
		n = 1
	}
	avgPerMonth = sumNumericField(records, "totalTradeCount") / float64(n)

	midpoint := windowStart.Add(windowEnd.Sub(windowStart) / 2)
	var firstHalf, secondHalf []map[string]any
	for _, rec := range records {
		d, ok := recordDateAtField(rec, "beginningOfMonth")
		if ok && d.Before(midpoint) {
			firstHalf = append(firstHalf, rec)
		} else {
			secondHalf = append(secondHalf, rec)
		}
	}
	firstTrades := sumNumericField(firstHalf, "totalTradeCount")
	secondTrades := sumNumericField(secondHalf, "totalTradeCount")
	firstVolume := sumNumericField(firstHalf, "totalVolumeQuantity")
	secondVolume := sumNumericField(secondHalf, "totalVolumeQuantity")

	switch {
	case secondTrades < firstTrades && secondVolume >= firstVolume:
		trend = "deteriorating"
	case secondTrades > firstTrades:
		trend = "improving"
	default:
		trend = "stable"
	}
	return avgPerMonth, trend, ""
}

// sumNumericField adds up a named numeric field across records, skipping
// records where the field is absent or unparseable. FINRA can encode a
// numeric field as either a JSON number or a JSON-encoded string (confirmed
// pattern for expires_in elsewhere in this CLI), so both shapes are accepted
// rather than silently dropping string-encoded values via a bare float64
// type assertion.
func sumNumericField(records []map[string]any, field string) float64 {
	var total float64
	for _, rec := range records {
		switch v := rec[field].(type) {
		case float64:
			total += v
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				total += f
			}
		}
	}
	return total
}
