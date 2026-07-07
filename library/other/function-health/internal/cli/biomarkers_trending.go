// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelBiomarkersTrendingCmd(flags *rootFlags) *cobra.Command {
	var direction string
	var lastN int
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "trending",
		Short:       "Biomarkers whose slope across the last N rounds points away from (or toward) Function's optimal range",
		Long:        "Computes a linear-fit slope of value vs round for every biomarker over the last N rounds, projects onto the axis 'moving AWAY from the optimal midpoint', and sorts by magnitude. Use --direction worse for drifting away (default), better for drifting toward optimal.",
		Example:     "  function-health-pp-cli biomarkers trending\n  function-health-pp-cli biomarkers trending --direction better --last 4 --top 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			s, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer safeCloseStore(s)
			rows, err := loadAllResults(ctx, s)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return noStoreData("trending")
			}
			groups := groupByBiomarker(rows)

			type entry struct {
				Biomarker   string  `json:"biomarker"`
				Category    string  `json:"category"`
				LatestValue float64 `json:"latest_value"`
				Unit        string  `json:"unit"`
				Drift       float64 `json:"drift_per_round"`
				DrawsUsed   int     `json:"draws_used"`
				LatestDraw  string  `json:"latest_draw_date"`
				OptimalLow  float64 `json:"optimal_low"`
				OptimalHigh float64 `json:"optimal_high"`
			}
			var list []entry
			for name, series := range groups {
				if len(series) < 2 {
					continue
				}
				drift := driftAway(series, lastN)
				if drift == 0 {
					continue
				}
				if direction == "worse" && drift <= 0 {
					continue
				}
				if direction == "better" && drift >= 0 {
					continue
				}
				latest := series[len(series)-1]
				used := lastN
				if used > len(series) {
					used = len(series)
				}
				list = append(list, entry{
					Biomarker:   name,
					Category:    latest.Category,
					LatestValue: latest.Value,
					Unit:        latest.Unit,
					Drift:       drift,
					DrawsUsed:   used,
					LatestDraw:  formatDrawDate(latest.DrawDate),
					OptimalLow:  latest.OptimalLow,
					OptimalHigh: latest.OptimalHigh,
				})
			}
			if len(list) == 0 {
				return notFoundErr(fmt.Errorf("no biomarkers trending %s across last %d rounds", direction, lastN))
			}
			sort.Slice(list, func(i, j int) bool {
				if direction == "better" {
					return list[i].Drift < list[j].Drift
				}
				return list[i].Drift > list[j].Drift
			})
			if topN > 0 && topN < len(list) {
				list = list[:topN]
			}
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, list)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Biomarkers trending %s (last %d rounds):\n", direction, lastN)
			for i, e := range list {
				fmt.Fprintf(w, "  %d. %-32s %.2f %s  drift/rd %+.3f  (%s)\n",
					i+1, e.Biomarker, e.LatestValue, e.Unit, e.Drift, e.LatestDraw)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "worse", "worse (drift away from optimal) | better (drift toward optimal)")
	cmd.Flags().IntVar(&lastN, "last", 3, "Number of recent rounds to compute the slope across")
	cmd.Flags().IntVar(&topN, "top", 25, "Maximum biomarkers to return (0 = unlimited)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}
