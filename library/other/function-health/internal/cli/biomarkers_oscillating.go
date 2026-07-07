// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelBiomarkersOscillatingCmd(flags *rootFlags) *cobra.Command {
	var rounds int
	var minCrossings int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "oscillating",
		Short:       "Biomarkers that crossed the Function-optimal range boundary multiple times in the last N rounds",
		Long:        "Distinguishes 'unstable measurement noise' from 'consistently out of range' by counting how many times each biomarker crossed in/out of Function's optimal range across the last N rounds.",
		Example:     "  function-health-pp-cli biomarkers oscillating\n  function-health-pp-cli biomarkers oscillating --rounds 6 --crossings 3 --json",
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
				return noStoreData("oscillating")
			}
			groups := groupByBiomarker(rows)

			type entry struct {
				Biomarker  string  `json:"biomarker"`
				Category   string  `json:"category"`
				Crossings  int     `json:"crossings"`
				DrawsUsed  int     `json:"draws_used"`
				LatestVal  float64 `json:"latest_value"`
				Unit       string  `json:"unit"`
				LatestDraw string  `json:"latest_draw_date"`
			}
			var list []entry
			for name, series := range groups {
				if len(series) < 2 {
					continue
				}
				tail := series
				if rounds > 0 && rounds < len(series) {
					tail = series[len(series)-rounds:]
				}
				crossings, defined := optimalCrossings(tail)
				// Need at least two classifiable draws to judge oscillation at all.
				if len(defined) < 2 {
					continue
				}
				if crossings < minCrossings {
					continue
				}
				latest := defined[len(defined)-1]
				list = append(list, entry{
					Biomarker:  name,
					Category:   latest.Category,
					Crossings:  crossings,
					DrawsUsed:  len(defined),
					LatestVal:  latest.Value,
					Unit:       latest.Unit,
					LatestDraw: formatDrawDate(latest.DrawDate),
				})
			}
			if len(list) == 0 {
				return notFoundErr(fmt.Errorf("no biomarkers oscillated >= %d times in the last %d rounds", minCrossings, rounds))
			}
			sort.Slice(list, func(i, j int) bool { return list[i].Crossings > list[j].Crossings })
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, list)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Biomarkers oscillating across Function-optimal boundary (last %d rounds, >= %d crossings):\n", rounds, minCrossings)
			for i, e := range list {
				fmt.Fprintf(w, "  %d. %-32s crossings=%d  latest %.2f %s (%s)\n",
					i+1, e.Biomarker, e.Crossings, e.LatestVal, e.Unit, e.LatestDraw)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&rounds, "rounds", 4, "Window of recent rounds to inspect for crossings")
	cmd.Flags().IntVar(&minCrossings, "crossings", 2, "Minimum number of optimal-boundary crossings to flag")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}

// optimalCrossings counts how many times a biomarker crossed the
// Function-optimal boundary across a chronological series, and returns the
// subset of draws that were actually classifiable.
//
// Only draws with a defined Function-optimal range can be classified as
// above / below / in-range; draws lacking one are inconclusive and are
// excluded from the series rather than treated as in-range. This is the fix
// for the "oscillating inconclusive" bug: optimalSign returns 0 for BOTH
// "in range" and "no optimal range defined", so feeding raw draws to the
// crossing counter let an undefined-optimal draw absorb a real above->below
// crossing and silently under-report oscillation.
//
// A crossing is ANY change in classification between consecutive defined draws,
// including in-range↔out-of-range transitions — not just above↔below flips.
// The command's contract is "crossed in/out of Function's optimal range", so a
// biomarker cycling above → in-range → above (managed on/off a supplement) must
// register each boundary crossing rather than being treated as stable because
// the in-range draws were transparent.
func optimalCrossings(series []resultRow) (crossings int, defined []resultRow) {
	defined = make([]resultRow, 0, len(series))
	for _, r := range series {
		if hasOptimal(r) {
			defined = append(defined, r)
		}
	}
	if len(defined) < 2 {
		return 0, defined
	}
	prevSign := optimalSign(defined[0])
	for i := 1; i < len(defined); i++ {
		s := optimalSign(defined[i])
		if s != prevSign {
			crossings++
		}
		prevSign = s
	}
	return crossings, defined
}

// hasOptimal reports whether a draw carries a usable Function-optimal range
// (at least one bound set). This is the distinction optimalSign cannot make:
// optimalSign returns 0 for BOTH "value is in range" and "no optimal range
// defined", so callers that must exclude inconclusive draws (e.g. the
// oscillation detector) gate on hasOptimal instead.
func hasOptimal(r resultRow) bool {
	return r.OptimalLow > 0 || r.OptimalHigh > 0
}

// optimalSign returns +1 above optimal high, -1 below optimal low, 0 in range
// (or undefined optimal). Each comparison is guarded on its bound being set, so
// a biomarker that supplies only a lower bound (OptimalHigh == 0) is never
// flagged "above" for an arbitrary positive value — it can only be in-range or
// below. Used to detect crossings across rounds.
func optimalSign(r resultRow) int {
	if r.OptimalHigh > 0 && r.Value > r.OptimalHigh {
		return 1
	}
	if r.OptimalLow > 0 && r.Value < r.OptimalLow {
		return -1
	}
	return 0
}
