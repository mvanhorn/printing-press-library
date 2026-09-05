// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// watch: indexation regression detector. Snapshots rank/traffic and crawl
// stats, then diffs the latest two captures to surface per-site regressions.
// Hand-authored transcendence command.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type bWatchResult struct {
	Site         string   `json:"site"`
	Regressions  []string `json:"regressions"`
	Improvements []string `json:"improvements"`
	Note         string   `json:"note,omitempty"`
}

// sumDailyStats sums a numeric field across a stats array (one row per day).
// Pure for testability.
func sumDailyStats(data json.RawMessage, field string) float64 {
	var total float64
	for _, it := range bArray(data) {
		m := bCIMap(it)
		if m == nil {
			continue
		}
		if v, ok := bNum(m, field); ok {
			total += v
		}
	}
	return total
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Diff the latest sync against the previous one and surface per-site regressions",
		Long:        "Capture rank/traffic and crawl-stats snapshots and compare them to the previous capture: total impressions/clicks change, crawl-error change, and in-index change. A single 'what regressed' digest. First run captures a baseline.",
		Example:     "  bing-webmaster-pp-cli watch --site https://example.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site, fmt.Sprintf("would capture and diff rank/traffic + crawl snapshots for %q", site))
			if err != nil || !proceed {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openSnapshots()
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now()

			result := bWatchResult{Site: site, Regressions: []string{}, Improvements: []string{}}
			anyBaseline := false

			// rank/traffic
			rt, err := c.Get(cmd.Context(), "/json/GetRankAndTrafficStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			prevRT, hadRT, err := db.Latest(site, "rank_traffic")
			if err != nil {
				return err
			}
			if err := db.Capture(site, "rank_traffic", rt, now); err != nil {
				return err
			}
			if hadRT {
				diffMetric(&result, "impressions", sumDailyStats(prevRT.Data, "Impressions"), sumDailyStats(rt, "Impressions"), false)
				diffMetric(&result, "clicks", sumDailyStats(prevRT.Data, "Clicks"), sumDailyStats(rt, "Clicks"), false)
			} else {
				anyBaseline = true
			}

			// crawl
			cr, err := c.Get(cmd.Context(), "/json/GetCrawlStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			prevCR, hadCR, err := db.Latest(site, "crawl")
			if err != nil {
				return err
			}
			if err := db.Capture(site, "crawl", cr, now); err != nil {
				return err
			}
			if hadCR {
				// More crawl errors is a regression (higherIsWorse = true).
				diffMetric(&result, "crawl_errors", sumDailyStats(prevCR.Data, "CrawlErrors"), sumDailyStats(cr, "CrawlErrors"), true)
				diffMetric(&result, "in_index", sumDailyStats(prevCR.Data, "InIndex"), sumDailyStats(cr, "InIndex"), false)
			} else {
				anyBaseline = true
			}

			if anyBaseline && len(result.Regressions) == 0 && len(result.Improvements) == 0 {
				result.Note = "Baseline captured. Run again after the next sync to detect regressions."
			}
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Watch for %s\n", site)
				if result.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", result.Note)
				}
				for _, r := range result.Regressions {
					fmt.Fprintf(cmd.OutOrStdout(), "  REGRESSION: %s\n", r)
				}
				for _, im := range result.Improvements {
					fmt.Fprintf(cmd.OutOrStdout(), "  improved: %s\n", im)
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	return cmd
}

// diffMetric records a regression or improvement based on the delta and
// whether higher values are worse (e.g. crawl errors).
func diffMetric(r *bWatchResult, name string, old, current float64, higherIsWorse bool) {
	delta := current - old
	if delta == 0 {
		return
	}
	worse := (delta > 0 && higherIsWorse) || (delta < 0 && !higherIsWorse)
	msg := fmt.Sprintf("%s %.0f -> %.0f (%+.0f)", name, old, current, delta)
	if worse {
		r.Regressions = append(r.Regressions, msg)
	} else {
		r.Improvements = append(r.Improvements, msg)
	}
}
