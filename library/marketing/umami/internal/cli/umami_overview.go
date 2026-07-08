// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: one-call site overview (stats + top pages +
// referrers + devices + browsers + countries), absorbed from frontedu's
// get_site_overview / get_visitor_demographics MCP tools.

package cli

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newOverviewCmd(flags *rootFlags) *cobra.Command {
	var period string
	var limit int
	cmd := &cobra.Command{
		Use:   "overview [site]",
		Short: "One-call site overview: stats with comparison, top pages, referrers, devices, browsers, countries",
		Long: `Fetch a complete traffic overview for one site in a single command:
summary stats compared to the previous period, plus top pages, referrers,
devices, browsers, and countries.

Use this command for a full-site traffic summary. Do NOT use it for
acquisition-channel analysis; use 'seo' instead.`,
		Example:     "  umami-pp-cli overview restodom.fr --period 7d\n  umami-pp-cli overview f1fa838a-fc6a-49c1-bdef-c3688dc92574 --period 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch stats + top metrics for the site")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			websiteID, err := resolveWebsiteID(ctx, c, args[0])
			if err != nil {
				return err
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			stats, statsErr := getStats(ctx, c, websiteID, w, "prev")
			metrics, failures := fanoutMetricsTypes(ctx, c, websiteID, w,
				[]string{"path", "referrer", "device", "browser", "country"}, limit)
			if statsErr != nil {
				failures = append(failures, fetchFailure{ID: "stats", Error: statsErr.Error()})
			}
			view := map[string]any{
				"website":   websiteID,
				"period":    period,
				"top_pages": metrics["path"], "top_referrers": metrics["referrer"],
				"devices": metrics["device"], "browsers": metrics["browser"], "countries": metrics["country"],
			}
			if statsErr == nil {
				summary := map[string]any{
					"pageviews": stats.Pageviews, "visitors": stats.Visitors, "visits": stats.Visits,
					"bounces": stats.Bounces, "totaltime": stats.Totaltime,
				}
				if stats.Comparison != nil {
					summary["previous"] = stats.Comparison
					summary["visitors_change_pct"] = round1(pctChange(stats.Visitors, stats.Comparison.Visitors))
					summary["pageviews_change_pct"] = round1(pctChange(stats.Pageviews, stats.Comparison.Pageviews))
				}
				view["summary"] = summary
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 6 fetches failed; overview assembled from the remaining sections\n", len(failures))
				view["fetch_failures"] = failures
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "7d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Rows per breakdown section")
	return cmd
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
