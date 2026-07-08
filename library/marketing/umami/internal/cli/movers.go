// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: movers report. Self-joins two adjacent windows of page and
// referrer breakdowns to rank risers and fallers.

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	var limit int
	var dimension string
	cmd := &cobra.Command{
		Use:   "movers [site]",
		Short: "Biggest page- and referrer-level risers and fallers this period vs the previous one",
		Long: `Fetch the page and referrer breakdowns for the current and previous
windows, join them by name, and rank the biggest absolute movements with
absolute and percentage deltas.

Use this command for page/referrer-level week-over-week risers and fallers
on one site. Do NOT use it for site-level growth acceleration across
periods; use 'trends' instead.`,
		Example:     "  umami-pp-cli movers restodom.fr --period 7d --agent\n  umami-pp-cli movers restodom.fr --dimension referrer",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank page/referrer risers and fallers")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			if dimension != "path" && dimension != "referrer" && dimension != "both" {
				return usageErr(fmt.Errorf("--dimension must be path, referrer, or both"))
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
			cur, err := parsePeriod(flagPeriod, time.Now())
			if err != nil {
				return usageErr(err)
			}
			prev := previousWindow(cur)
			dims := []string{"path", "referrer"}
			if dimension != "both" {
				dims = []string{dimension}
			}
			curM, curFail := fanoutMetricsTypes(ctx, c, websiteID, cur, dims, 500)
			prevM, prevFail := fanoutMetricsTypes(ctx, c, websiteID, prev, dims, 500)
			if len(curFail) > 0 {
				return apiErr(fmt.Errorf("fetching current window: %s", curFail[0].Error))
			}
			if len(prevFail) > 0 {
				return apiErr(fmt.Errorf("fetching previous window: %s", prevFail[0].Error))
			}
			type mover struct {
				Dimension string  `json:"dimension"`
				Name      string  `json:"name"`
				Current   float64 `json:"current"`
				Previous  float64 `json:"previous"`
				Delta     float64 `json:"delta"`
				ChangePct float64 `json:"change_pct"`
				IsNew     bool    `json:"new,omitempty"`
			}
			var movers []mover
			for _, dim := range dims {
				prevVals := map[string]float64{}
				for _, r := range prevM[dim] {
					prevVals[r.X] = r.Y
				}
				seen := map[string]bool{}
				for _, r := range curM[dim] {
					seen[r.X] = true
					movers = append(movers, mover{
						Dimension: dim, Name: orDirect(r.X),
						Current: r.Y, Previous: prevVals[r.X],
						Delta: r.Y - prevVals[r.X], ChangePct: round1(pctChange(r.Y, prevVals[r.X])),
						IsNew: prevVals[r.X] == 0 && r.Y > 0,
					})
				}
				// Fallers that disappeared entirely from the current window.
				for _, r := range prevM[dim] {
					if !seen[r.X] {
						movers = append(movers, mover{
							Dimension: dim, Name: orDirect(r.X),
							Current: 0, Previous: r.Y, Delta: -r.Y, ChangePct: -100,
						})
					}
				}
			}
			sort.Slice(movers, func(i, j int) bool {
				return math.Abs(movers[i].Delta) > math.Abs(movers[j].Delta)
			})
			risers := make([]mover, 0, limit)
			fallers := make([]mover, 0, limit)
			for _, m := range movers {
				if m.Delta > 0 && len(risers) < limit {
					risers = append(risers, m)
				}
				if m.Delta < 0 && len(fallers) < limit {
					fallers = append(fallers, m)
				}
			}
			out := map[string]any{
				"site": args[0], "website": websiteID, "period": flagPeriod,
				"risers": risers, "fallers": fallers,
				"scanned_rows": len(movers),
			}
			if len(movers) == 0 {
				out["note"] = "no traffic rows in either window"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "7d", "Window length (compared against the previous equal window)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max risers and fallers returned each")
	cmd.Flags().StringVar(&dimension, "dimension", "both", "Dimension to rank: path | referrer | both")
	return cmd
}
