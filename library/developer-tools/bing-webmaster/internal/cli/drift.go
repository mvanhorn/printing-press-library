// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// drift: ranking-drift detection. Ranks average-position movement per query
// over time using local snapshot history. Hand-authored transcendence command.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var site string
	var top int
	var days int
	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Ranking drift: biggest average-position climbers and droppers vs ~N days ago",
		Long:        "Capture a fresh query snapshot and rank average-position movement against a baseline from ~N days ago. Climbers improved (moved up); droppers fell. Requires local snapshot history, so the first run captures a baseline.",
		Example:     "  bing-webmaster-pp-cli drift --site https://example.com --top 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site,
				fmt.Sprintf("would capture a query snapshot for %q and rank position drift (top %d) vs ~%d days ago", site, top, days))
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
					"message":           fmt.Sprintf("Baseline captured for %s. Run again in ~%d days to see ranking drift.", site, days),
				}
				return emitIntel(cmd, flags, out, func() { cmd.Println(out["message"].(string)) })
			}
			result := bComputeDrift(bParseQueryRows(baseline.Data), current, top, days)
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Ranking drift for %s (baseline %s)\n", site, baseline.CapturedAt.Format("2006-01-02"))
				fmt.Fprintf(cmd.OutOrStdout(), "  climbers: %d   droppers: %d\n", len(result.Climbers), len(result.Droppers))
				for _, r := range result.Climbers {
					fmt.Fprintf(cmd.OutOrStdout(), "  ^ %-40s %.1f -> %.1f (%.1f)\n", r.Query, r.OldPosition, r.NewPosition, r.PositionDelta)
				}
				for _, r := range result.Droppers {
					fmt.Fprintf(cmd.OutOrStdout(), "  v %-40s %.1f -> %.1f (+%.1f)\n", r.Query, r.OldPosition, r.NewPosition, r.PositionDelta)
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	cmd.Flags().IntVar(&top, "top", 20, "Number of climbers/droppers to show")
	cmd.Flags().IntVar(&days, "days", 7, "Comparison window in days")
	return cmd
}
