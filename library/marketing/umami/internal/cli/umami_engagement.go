// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: 0-100 engagement score, absorbed from frontedu's
// get_engagement_score MCP tool. Formula is documented and reproducible.

package cli

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newEngagementCmd(flags *rootFlags) *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   "engagement [site]",
		Short: "Engagement score 0-100 from bounce rate, visit duration, and pages per visit",
		Long: `Compute a 0-100 engagement score for one site:
score = (1 - bounce_rate)*40 + min(avg_visit_seconds/180, 1)*30 + min(pages_per_visit/4, 1)*30.
The components are returned alongside the score so the formula is auditable.`,
		Example:     "  umami-pp-cli engagement restodom.fr --period 30d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute the engagement score from site stats")
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
			stats, err := getStats(ctx, c, websiteID, w, "")
			if err != nil {
				return apiErr(fmt.Errorf("fetching stats: %w", err))
			}
			if stats.Visits == 0 {
				out := map[string]any{"website": websiteID, "period": period, "score": 0,
					"note": "no visits in this period; score is 0 by definition"}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			bounceRate := stats.Bounces / stats.Visits
			avgSeconds := stats.Totaltime / stats.Visits
			pagesPerVisit := stats.Pageviews / stats.Visits
			score := (1-bounceRate)*40 + math.Min(avgSeconds/180, 1)*30 + math.Min(pagesPerVisit/4, 1)*30
			out := map[string]any{
				"website":         websiteID,
				"period":          period,
				"score":           round1(score),
				"bounce_rate_pct": round1(bounceRate * 100),
				"avg_visit_sec":   round1(avgSeconds),
				"pages_per_visit": round1(pagesPerVisit),
				"visits":          stats.Visits,
				"formula":         "(1-bounce)*40 + min(avg_sec/180,1)*30 + min(pages_per_visit/4,1)*30",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "30d", "Period (24h, 7d, 30d, this-month, ...)")
	return cmd
}
