// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: 3-window growth trend with acceleration verdict,
// absorbed from frontedu's get_growth_trends MCP tool.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   "trends [site]",
		Short: "Growth trend across three adjacent periods with an acceleration verdict",
		Long: `Fetch stats for the current period and the two before it, then report
period-over-period growth and whether growth is accelerating, decelerating,
or steady.

Use this command for site-level growth acceleration across periods. Do NOT
use it for page/referrer-level risers and fallers; use 'movers' instead.`,
		Example:     "  umami-pp-cli trends restodom.fr --period 7d",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch three adjacent periods and compute growth")
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
			w2, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			w1 := previousWindow(w2)
			w0 := previousWindow(w1)
			windows := []periodWindow{w0, w1, w2}
			type windowStats struct {
				Visitors  float64 `json:"visitors"`
				Pageviews float64 `json:"pageviews"`
			}
			results := make([]windowStats, 3)
			for i, win := range windows {
				s, err := getStats(ctx, c, websiteID, win, "")
				if err != nil {
					return apiErr(fmt.Errorf("fetching window %d: %w", i, err))
				}
				results[i] = windowStats{Visitors: s.Visitors, Pageviews: s.Pageviews}
			}
			growthRecent := pctChange(results[2].Visitors, results[1].Visitors)
			growthPrior := pctChange(results[1].Visitors, results[0].Visitors)
			verdict := "steady"
			switch {
			case growthRecent > growthPrior+5:
				verdict = "accelerating"
			case growthRecent < growthPrior-5:
				verdict = "decelerating"
			}
			out := map[string]any{
				"website":            websiteID,
				"period":             period,
				"windows":            results,
				"growth_recent_pct":  round1(growthRecent),
				"growth_prior_pct":   round1(growthPrior),
				"acceleration":       verdict,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "7d", "Window length for each of the three periods")
	return cmd
}
