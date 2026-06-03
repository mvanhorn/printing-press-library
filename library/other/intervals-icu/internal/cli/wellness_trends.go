// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Novel feature: wellness vs training-load correlation. Joins the wellness
// time series and the daily activity-load series from the live API and reports
// a Pearson correlation per wellness metric. The cross-series join is the
// value-add; no single endpoint returns it.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/intervals-icu/internal/cliutil"

	"github.com/spf13/cobra"
)

type wellnessPoint struct {
	Date      string  `json:"date"`
	HRV       float64 `json:"hrv,omitempty"`
	RestingHR float64 `json:"resting_hr,omitempty"`
	SleepHrs  float64 `json:"sleep_hrs,omitempty"`
	Weight    float64 `json:"weight,omitempty"`
	Load      float64 `json:"load"`
}

type wellnessCorr struct {
	Metric  string  `json:"metric"`
	R       float64 `json:"r"`
	N       int     `json:"n"`
	Comment string  `json:"comment"`
}

type wellnessTrendsView struct {
	Days         int             `json:"days"`
	AthleteID    string          `json:"athlete_id"`
	Series       []wellnessPoint `json:"series"`
	Correlations []wellnessCorr  `json:"correlations"`
	Note         string          `json:"note,omitempty"`
}

func newNovelWellnessTrendsCmd(flags *rootFlags) *cobra.Command {
	var flagDays string

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Correlate HRV / resting HR / sleep against training load.",
		Long: "Join the wellness series with daily training load over a window and\n" +
			"report a Pearson correlation per metric. Use to see whether HRV or\n" +
			"resting HR track accumulated fatigue.",
		Example:     "  intervals-icu-pp-cli wellness trends --days 60 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch wellness + activity load and correlate")
				return nil
			}
			days := parseWindowDays(flagDays, 60)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id := athleteID(flags)
			oldest, newest := localDate(-days), localDate(0)

			wPath := replacePathParam("/api/v1/athlete/{id}/wellness", "id", id)
			wData, err := c.Get(cmd.Context(), wPath, map[string]string{"oldest": oldest, "newest": newest})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var records []map[string]json.RawMessage
			if err := json.Unmarshal(wData, &records); err != nil {
				return fmt.Errorf("parsing wellness: %w", err)
			}

			// Daily training load from activities.
			aPath := replacePathParam("/api/v1/athlete/{id}/activities", "id", id)
			aData, err := c.Get(cmd.Context(), aPath, map[string]string{"oldest": oldest, "newest": newest})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var acts []map[string]json.RawMessage
			if err := json.Unmarshal(aData, &acts); err != nil {
				return fmt.Errorf("parsing activities: %w", err)
			}
			loadByDay := map[string]float64{}
			for _, a := range acts {
				d := dayOf(jsonStr(a, "start_date_local"))
				if d == "" {
					d = dayOf(jsonStr(a, "start_date"))
				}
				if l, ok := cliutil.ExtractNumber(a, "icu_training_load"); ok {
					loadByDay[d] += l
				}
			}

			series := make([]wellnessPoint, 0, len(records))
			for _, r := range records {
				d := dayOf(jsonStr(r, "id"))
				p := wellnessPoint{Date: d, Load: loadByDay[d]}
				if v, ok := firstNumber(r, "hrv", "hrvSDNN"); ok {
					p.HRV = round1(v)
				}
				if v, ok := firstNumber(r, "restingHR", "restingHr"); ok {
					p.RestingHR = round1(v)
				}
				if v, ok := firstNumber(r, "sleepSecs"); ok {
					p.SleepHrs = round1(v / 3600)
				}
				if v, ok := firstNumber(r, "weight"); ok {
					p.Weight = round1(v)
				}
				series = append(series, p)
			}
			sort.SliceStable(series, func(i, j int) bool { return series[i].Date < series[j].Date })

			view := wellnessTrendsView{Days: days, AthleteID: id, Series: series}
			for _, m := range []struct {
				name string
				sel  func(wellnessPoint) (float64, bool)
			}{
				{"hrv", func(p wellnessPoint) (float64, bool) { return p.HRV, p.HRV != 0 }},
				{"resting_hr", func(p wellnessPoint) (float64, bool) { return p.RestingHR, p.RestingHR != 0 }},
				{"sleep_hrs", func(p wellnessPoint) (float64, bool) { return p.SleepHrs, p.SleepHrs != 0 }},
			} {
				xs, ys := pairsVsLoad(series, m.sel)
				if len(xs) < 3 {
					continue
				}
				r := pearson(xs, ys)
				view.Correlations = append(view.Correlations, wellnessCorr{
					Metric: m.name, R: round2(r), N: len(xs), Comment: corrComment(m.name, r),
				})
			}
			if len(series) == 0 {
				view.Note = fmt.Sprintf("no wellness records in the last %d days", days)
			} else if len(view.Correlations) == 0 {
				view.Note = "not enough overlapping wellness+load days to correlate (need >= 3)"
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Wellness vs training load, last %dd\n\n", days)
			for _, c := range view.Correlations {
				fmt.Fprintf(out, "  %-12s r=%+.2f (n=%d)  %s\n", c.Metric, c.R, c.N, c.Comment)
			}
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "", "Window to analyze, e.g. 60d (default 60)")
	return cmd
}

func firstNumber(m map[string]json.RawMessage, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := cliutil.ExtractNumber(m, k); ok {
			return v, true
		}
	}
	return 0, false
}

func pairsVsLoad(series []wellnessPoint, sel func(wellnessPoint) (float64, bool)) (xs, ys []float64) {
	for _, p := range series {
		v, ok := sel(p)
		if !ok {
			continue
		}
		xs = append(xs, v)
		ys = append(ys, p.Load)
	}
	return xs, ys
}

// pearson computes the Pearson correlation coefficient. Returns 0 for
// degenerate input (zero variance).
func pearson(xs, ys []float64) float64 {
	n := float64(len(xs))
	if n == 0 {
		return 0
	}
	var sx, sy, sxx, syy, sxy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		syy += ys[i] * ys[i]
		sxy += xs[i] * ys[i]
	}
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sxx - sx*sx) * (n*syy - sy*sy))
	if den == 0 {
		return 0
	}
	return num / den
}

func corrComment(metric string, r float64) string {
	mag := math.Abs(r)
	strength := "no"
	switch {
	case mag >= 0.6:
		strength = "strong"
	case mag >= 0.3:
		strength = "moderate"
	case mag >= 0.1:
		strength = "weak"
	}
	if strength == "no" {
		return "no clear relationship with load"
	}
	dir := "rises with"
	if r < 0 {
		dir = "falls with"
	}
	return fmt.Sprintf("%s correlation; %s training load", strength, dir)
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
