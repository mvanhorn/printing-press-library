// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: the multi-site rollup Umami's UI lacks (community
// issues #3362/#3174), absorbed from the web-analytics plugin's portfolio
// sweep pattern.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	var period string
	var sortBy string
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Every website's traffic at a glance — the multi-site rollup Umami's UI doesn't have",
		Long: `Fetch stats for every website concurrently and print one row per site with
visitors, pageviews, bounce rate, and the change versus the previous period.

Use this command for a cross-site sweep. Do NOT use it for anomaly
detection against local baselines; use 'watch' instead.`,
		Example:     "  umami-pp-cli portfolio --period 7d\n  umami-pp-cli portfolio --period 30d --sort growth",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch stats for every website concurrently")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sites, err := fetchAllWebsites(ctx, c)
			if err != nil {
				return err
			}
			if len(sites) == 0 {
				return notFoundErr(fmt.Errorf("no websites on this account"))
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			results := fanoutStats(ctx, c, sites, w, "prev")
			ok, failures := splitStatsResults(results)
			type row struct {
				Name             string  `json:"name"`
				Domain           string  `json:"domain"`
				ID               string  `json:"id"`
				Visitors         float64 `json:"visitors"`
				Pageviews        float64 `json:"pageviews"`
				BounceRatePct    float64 `json:"bounce_rate_pct"`
				VisitorsChangePct float64 `json:"visitors_change_pct"`
			}
			rows := make([]row, 0, len(ok))
			for _, r := range ok {
				bounce := 0.0
				if r.Stats.Visits > 0 {
					bounce = r.Stats.Bounces / r.Stats.Visits * 100
				}
				change := 0.0
				if r.Stats.Comparison != nil {
					change = pctChange(r.Stats.Visitors, r.Stats.Comparison.Visitors)
				}
				rows = append(rows, row{
					Name: r.Site.Name, Domain: r.Site.Domain, ID: r.Site.ID,
					Visitors: r.Stats.Visitors, Pageviews: r.Stats.Pageviews,
					BounceRatePct: round1(bounce), VisitorsChangePct: round1(change),
				})
			}
			switch sortBy {
			case "growth":
				sort.Slice(rows, func(i, j int) bool { return rows[i].VisitorsChangePct > rows[j].VisitorsChangePct })
			default:
				sort.Slice(rows, func(i, j int) bool { return rows[i].Visitors > rows[j].Visitors })
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d site fetches failed; rollup covers the remaining %d sites\n", len(failures), len(sites), len(rows))
			}
			out := map[string]any{"period": period, "sites": rows, "fetch_failures": failures}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "7d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().StringVar(&sortBy, "sort", "visitors", "Sort rows by: visitors | growth")
	return cmd
}
