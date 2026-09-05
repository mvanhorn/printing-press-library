// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// review: period-over-period query-performance delta. Captures a fresh
// GetQueryStats snapshot and diffs it against a baseline ~N days old. No single
// Bing API call returns deltas; this is only possible with the local snapshot
// history. Hand-authored transcendence command.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newReviewCmd(flags *rootFlags) *cobra.Command {
	var site string
	var days int
	cmd := &cobra.Command{
		Use:         "review",
		Short:       "Query-performance deltas vs the previous period (gained/lost queries, CTR & position shifts)",
		Long:        "Capture a fresh query-performance snapshot and compare it against a baseline from ~N days ago: which queries you gained or lost, and how impressions, clicks, and average position moved. Requires local snapshot history, so the first run captures a baseline.",
		Example:     "  bing-webmaster-pp-cli review --site https://example.com --days 7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site,
				fmt.Sprintf("would capture a query snapshot for %q and diff against ~%d days ago", site, days))
			if err != nil || !proceed {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/json/GetQueryStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			current := bParseQueryRows(data)

			db, err := openSnapshots()
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now()
			if err := db.Capture(site, "queries", data, now); err != nil {
				return err
			}
			baseline, ok, err := db.Before(site, "queries", now.AddDate(0, 0, -days))
			if err != nil {
				return err
			}
			if !ok {
				if baseline, ok, err = db.Prior(site, "queries", now); err != nil {
					return err
				}
			}
			if !ok {
				out := map[string]any{
					"baseline_captured": true,
					"queries_now":       len(current),
					"message":           fmt.Sprintf("Baseline captured for %s (%d queries). Run again in ~%d days to see deltas.", site, len(current), days),
				}
				return emitIntel(cmd, flags, out, func() {
					cmd.Println(out["message"].(string))
				})
			}
			result := bComputeReview(bParseQueryRows(baseline.Data), current, days)
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Query review for %s (baseline %s)\n", site, baseline.CapturedAt.Format("2006-01-02"))
				fmt.Fprintf(cmd.OutOrStdout(), "  gained %d   lost %d   improved %d   declined %d\n",
					result.Summary.GainedCount, result.Summary.LostCount, result.Summary.ImprovedCount, result.Summary.DeclinedCount)
				if len(current) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  (Bing returned no query data — common below its data threshold)")
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	cmd.Flags().IntVar(&days, "days", 7, "Comparison window in days")
	return cmd
}
