// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Novel feature: season-over-season curve comparison. Fetches the athlete's
// best power/pace/HR curve for two date ranges from the live API and reports
// both plus the delta at standard durations. The two-range join is the
// value-add; each API call returns only one range.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type curveRange struct {
	Oldest string `json:"oldest"`
	Newest string `json:"newest"`
}

type curveCompareView struct {
	Metric    string      `json:"metric"`
	AthleteID string      `json:"athlete_id"`
	This      curveRange  `json:"this"`
	Vs        curveRange  `json:"vs"`
	Peaks     []curvePeak `json:"peaks,omitempty"`
	Note      string      `json:"note,omitempty"`
}

type curvePeak struct {
	Secs  int     `json:"secs"`
	This  float64 `json:"this"`
	Vs    float64 `json:"vs"`
	Delta float64 `json:"delta"`
}

func newNovelCurveCompareCmd(flags *rootFlags) *cobra.Command {
	var flagMetric, flagThis, flagVs, flagType string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare best power/pace/HR curves between two date ranges.",
		Long: "Fetch the athlete's best-effort curve for two windows and report the\n" +
			"delta at standard durations. Use to quantify season-over-season\n" +
			"fitness change at a given duration.",
		Example:     "  intervals-icu-pp-cli curve compare --metric power --this 90d --vs 365d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch best-effort curves for two date ranges and compare")
				return nil
			}
			metric := flagMetric
			if metric == "" {
				metric = "power"
			}
			seg, ok := map[string]string{
				"power": "power-curves",
				"pace":  "pace-curves",
				"hr":    "hr-curves",
			}[metric]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metric must be one of power, pace, hr (got %q)", metric))
			}
			thisDays := parseWindowDays(flagThis, 90)
			vsDays := parseWindowDays(flagVs, 365)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := athleteID(flags)
			path := replacePathParam("/api/v1/athlete/{id}/"+seg, "id", id)

			actType := flagType
			if actType == "" {
				actType = "Ride"
			}
			// The comparison window ends where the recent window begins, so the
			// two ranges do not overlap — otherwise "vs" (which would contain
			// "this") just reports the deeper history's bests and every delta
			// reads as <= 0. Require vs to reach further back than this.
			if vsDays <= thisDays {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--vs window (%dd) must be longer than --this (%dd) so the comparison period is earlier and non-overlapping", vsDays, thisDays))
			}
			fetch := func(oldestOff, newestOff int) (json.RawMessage, error) {
				return c.Get(cmd.Context(), path, map[string]string{
					"oldest": localDate(oldestOff),
					"newest": localDate(newestOff),
					"type":   actType, // intervals.icu curve endpoints require an ActivityType
				})
			}
			thisData, err := fetch(-thisDays, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Earlier, non-overlapping window: [now-vsDays, now-thisDays].
			vsData, err := fetch(-vsDays, -thisDays)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := curveCompareView{
				Metric:    metric,
				AthleteID: id,
				This:      curveRange{Oldest: localDate(-thisDays), Newest: localDate(0)},
				Vs:        curveRange{Oldest: localDate(-vsDays), Newest: localDate(-thisDays)},
			}
			thisSecs, thisVals := extractCurve(thisData)
			vsSecs, vsVals := extractCurve(vsData)
			if len(thisSecs) > 0 && len(vsSecs) > 0 {
				for _, d := range []int{5, 60, 300, 1200, 3600} {
					tv, okT := valueAtSecs(thisSecs, thisVals, d)
					vv, okV := valueAtSecs(vsSecs, vsVals, d)
					if okT && okV {
						view.Peaks = append(view.Peaks, curvePeak{Secs: d, This: round1(tv), Vs: round1(vv), Delta: round1(tv - vv)})
					}
				}
			} else {
				view.Note = "curve arrays not recognized in the API response; no peak comparison available"
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Best %s: last %dd vs the prior %dd (ending %dd ago)\n\n", metric, thisDays, vsDays-thisDays, thisDays)
			if len(view.Peaks) == 0 {
				fmt.Fprintln(out, view.Note)
				return nil
			}
			fmt.Fprintf(out, "%-8s %10s %10s %10s\n", "DUR", "THIS", "VS", "DELTA")
			for _, p := range view.Peaks {
				fmt.Fprintf(out, "%-8s %10.1f %10.1f %+10.1f\n", fmtSecs(p.Secs), p.This, p.Vs, p.Delta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "", "Curve metric: power, pace, or hr (default power)")
	cmd.Flags().StringVar(&flagThis, "this", "", "Recent window, e.g. 90d (default 90)")
	cmd.Flags().StringVar(&flagVs, "vs", "", "Total lookback for the earlier comparison window; must exceed --this. The comparison period is [now-vs, now-this] (default 365)")
	cmd.Flags().StringVar(&flagType, "type", "", "Activity type for the curve, e.g. Ride, Run, Swim (default Ride)")
	return cmd
}

// extractCurve finds a duration array (secs/durations) and a parallel numeric
// value array in an intervals.icu curve response. Returns nil slices when the
// shape is not recognized. Handles both a top-level object and a single-element
// array wrapping one.
func extractCurve(raw json.RawMessage) ([]float64, []float64) {
	obj := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		var arr []map[string]json.RawMessage
		if err2 := json.Unmarshal(raw, &arr); err2 != nil || len(arr) == 0 {
			return nil, nil
		}
		obj = arr[0]
	}
	// intervals.icu wraps curves as {"list":[{secs,values,...}]}; descend into
	// the first entry when the secs/values arrays are not at the top level.
	if _, hasSecs := obj["secs"]; !hasSecs {
		if listRaw, ok := obj["list"]; ok {
			var list []map[string]json.RawMessage
			if json.Unmarshal(listRaw, &list) == nil && len(list) > 0 {
				obj = list[0]
			}
		}
	}
	var secs []float64
	for _, k := range []string{"secs", "durations", "seconds"} {
		if v, ok := obj[k]; ok && json.Unmarshal(v, &secs) == nil && len(secs) > 0 {
			break
		}
		secs = nil
	}
	if len(secs) == 0 {
		return nil, nil
	}
	var vals []float64
	for _, k := range []string{"values", "watts", "bps", "y", "best"} {
		if v, ok := obj[k]; ok && json.Unmarshal(v, &vals) == nil && len(vals) == len(secs) {
			return secs, vals
		}
		vals = nil
	}
	return nil, nil
}

// valueAtSecs returns the value at the closest duration <= target (curves are
// monotonic in duration, so the closest bucket is a fair read).
func valueAtSecs(secs, vals []float64, target int) (float64, bool) {
	best := -1
	for i, s := range secs {
		if int(s) <= target && (best == -1 || s > secs[best]) {
			best = i
		}
	}
	if best == -1 {
		return 0, false
	}
	return vals[best], true
}

func fmtSecs(s int) string {
	switch {
	case s >= 3600:
		return fmt.Sprintf("%dh", s/3600)
	case s >= 60:
		return fmt.Sprintf("%dm", s/60)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
