// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGoatCmd(flags *rootFlags) *cobra.Command {
	var rounds int
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "goat",
		Short:       "Single most-worrying biomarker right now (distance-from-optimal × drift-away across last N rounds)",
		Long:        "Ranks every biomarker by (distance from Function-optimal midpoint) × (slope drifting away from optimal across the last N rounds) and returns the top result with reasoning fields. Pure SQL over the local synced store — no LLM, no external service.",
		Example:     "  function-health-pp-cli goat\n  function-health-pp-cli goat --rounds 4 --top 5 --json",
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
				return noStoreData("goat")
			}
			groups := groupByBiomarker(rows)

			type scored struct {
				Biomarker     string  `json:"biomarker"`
				BiomarkerID   string  `json:"biomarker_id"`
				Category      string  `json:"category"`
				LatestValue   float64 `json:"latest_value"`
				Unit          string  `json:"unit"`
				LatestStatus  string  `json:"latest_status"`
				OptimalLow    float64 `json:"optimal_low"`
				OptimalHigh   float64 `json:"optimal_high"`
				Distance      float64 `json:"distance_from_optimal"`
				DriftAway     float64 `json:"drift_away_per_round"`
				Score         float64 `json:"score"`
				DrawsAnalyzed int     `json:"draws_analyzed"`
				LatestDraw    string  `json:"latest_draw_date"`
			}
			var results []scored
			for name, series := range groups {
				if len(series) == 0 {
					continue
				}
				latest := series[len(series)-1]
				d := distanceFromOptimal(latest)
				drift := driftAway(series, rounds)
				if d == 0 && drift == 0 {
					continue
				}
				score := goatScore(d, drift)
				results = append(results, scored{
					Biomarker:     name,
					BiomarkerID:   latest.BiomarkerID,
					Category:      latest.Category,
					LatestValue:   latest.Value,
					Unit:          latest.Unit,
					LatestStatus:  latest.Status,
					OptimalLow:    latest.OptimalLow,
					OptimalHigh:   latest.OptimalHigh,
					Distance:      d,
					DriftAway:     drift,
					Score:         score,
					DrawsAnalyzed: len(series),
					LatestDraw:    formatDrawDate(latest.DrawDate),
				})
			}
			if len(results) == 0 {
				return noStoreData("goat — every biomarker is within optimal range or no Function-optimal ranges are set")
			}
			sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
			if topN > 0 && topN < len(results) {
				results = results[:topN]
			}
			if flags != nil && flags.asJSON {
				if topN == 1 {
					return flags.printJSON(cmd, results[0])
				}
				return flags.printJSON(cmd, results)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Most worrying biomarkers right now (by distance-from-optimal × drift-away):")
			for i, r := range results {
				fmt.Fprintf(w, "  %d. %-32s %.2f %-10s  %s  (optimal %.1f-%.1f, drift/rd %+.3f, %d draws)\n",
					i+1, r.Biomarker, r.LatestValue, r.Unit, r.LatestStatus,
					r.OptimalLow, r.OptimalHigh, r.DriftAway, r.DrawsAnalyzed)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&rounds, "rounds", 3, "Number of recent rounds to compute the drift-away slope across")
	cmd.Flags().IntVar(&topN, "top", 1, "How many top-ranked biomarkers to return (1 = the goat)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}

// goatScore weights a biomarker's distance from its Function-optimal midpoint by
// how fast it is drifting away. Drift toward optimal (negative) is clamped to
// zero so only worsening trends amplify the score; a perfectly flat slope still
// earns a small baseline weight so a far-but-stable biomarker can rank.
func goatScore(distance, drift float64) float64 {
	score := distance * math.Max(0, drift)
	if score == 0 {
		score = distance * 0.1 // baseline weight when slope is flat
	}
	return score
}
