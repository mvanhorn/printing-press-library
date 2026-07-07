// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `recovery` — workout HR-zone load weighed against recovery markers.
// pp:data-source local
//
// Hand-authored implementation (RunE body replaced from the generated stub).

package cli

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// recoveryDay is one dated row of the recovery report.
type recoveryDay struct {
	Date       string  `json:"date"`
	LoadMin    float64 `json:"load_min"`
	RestingHR  int     `json:"resting_hr"`
	SleepScore int     `json:"sleep_score"`
}

// recoverySummary captures the trend + the overtraining flag.
type recoverySummary struct {
	Trend string `json:"trend"`
	Flag  bool   `json:"flag"`
}

// recoveryResult is the JSON shape emitted by `recovery`.
type recoveryResult struct {
	Since   string          `json:"since"`
	Days    []recoveryDay   `json:"days"`
	Summary recoverySummary `json:"summary"`
}

func newNovelRecoveryCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "recovery",
		Short:       "Weigh recent workout HR-zone load against your recovery markers (resting HR, sleep score)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Joins, per day, the workout HR-zone load (minutes in zones 0-3) against your
recovery markers (resting heart rate proxy, sleep score) from the local mirror,
and flags days where training load is climbing while recovery is falling.

Reads local data only. Sync first with:
  withings-pp-cli sync --resources workouts,measure,activity,sleep`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseSinceFlag(flagSince, 14*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-since)

			db, handled, err := openLocalForAnalytics(cmd, flags, dbPath, "workouts,measure,activity,sleep", false)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			res, err := computeRecovery(db, cutoff, flagSinceLabel(flagSince, "14d"))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "14d", "Lookback window (e.g. 14d, 2w, 336h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	return cmd
}

// computeRecovery builds the per-day recovery rows and the summary from the
// local store, considering only days at/after cutoff.
func computeRecovery(db *store.Store, cutoff time.Time, sinceLabel string) (recoveryResult, error) {
	res := recoveryResult{Since: sinceLabel, Days: make([]recoveryDay, 0)}

	loadByDay := map[string]float64{}   // minutes in HR zones
	restingByDay := map[string]int{}    // min heart pulse / hr_min
	sleepScoreByDay := map[string]int{} // sleep score

	cutYMD := cutoff.UTC().Format("2006-01-02")
	inWindow := func(day string) bool { return day != "" && day >= cutYMD }

	// Workouts: sum hr_zone_0..3 (seconds) -> minutes, keyed by day.
	wRows, err := localRows(db, "workouts")
	if err != nil {
		return res, err
	}
	for _, raw := range wRows {
		var w struct {
			Date      string `json:"date"`
			StartDate int64  `json:"startdate"`
			Data      struct {
				Zone0 float64 `json:"hr_zone_0"`
				Zone1 float64 `json:"hr_zone_1"`
				Zone2 float64 `json:"hr_zone_2"`
				Zone3 float64 `json:"hr_zone_3"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &w) != nil {
			continue
		}
		day := w.Date
		if day == "" {
			day = epochToYMD(w.StartDate)
		}
		if !inWindow(day) {
			continue
		}
		secs := w.Data.Zone0 + w.Data.Zone1 + w.Data.Zone2 + w.Data.Zone3
		loadByDay[day] += secs / 60.0
	}

	// Measures: resting HR proxy = min heart_pulse (type 11) per day.
	groups, err := loadMeasureGroups(db, cutoff)
	if err != nil {
		return res, err
	}
	for _, g := range groups {
		day := epochToYMD(g.Date)
		if !inWindow(day) {
			continue
		}
		if hp, ok := g.scaledOfType(11); ok {
			v := int(hp)
			if cur, exists := restingByDay[day]; !exists || v < cur {
				restingByDay[day] = v
			}
		}
	}

	// Activity: fall back to hr_min when no measure-derived resting HR exists.
	aRows, err := localRows(db, "activity")
	if err != nil {
		return res, err
	}
	for _, raw := range aRows {
		var a struct {
			Date  string `json:"date"`
			HRMin int    `json:"hr_min"`
		}
		if json.Unmarshal(raw, &a) != nil {
			continue
		}
		if !inWindow(a.Date) || a.HRMin <= 0 {
			continue
		}
		if _, exists := restingByDay[a.Date]; !exists {
			restingByDay[a.Date] = a.HRMin
		} else if a.HRMin < restingByDay[a.Date] {
			restingByDay[a.Date] = a.HRMin
		}
	}

	// Sleep summaries: sleep_score per day.
	sRows, err := localRows(db, "sleep")
	if err != nil {
		return res, err
	}
	for _, raw := range sRows {
		var s struct {
			Date      string `json:"date"`
			StartDate int64  `json:"startdate"`
			Data      struct {
				SleepScore int `json:"sleep_score"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		day := s.Date
		if day == "" {
			day = epochToYMD(s.StartDate)
		}
		if !inWindow(day) {
			continue
		}
		if s.Data.SleepScore > 0 {
			sleepScoreByDay[day] = s.Data.SleepScore
		}
	}

	// Union of all days that have any datum.
	daySet := map[string]struct{}{}
	for d := range loadByDay {
		daySet[d] = struct{}{}
	}
	for d := range restingByDay {
		daySet[d] = struct{}{}
	}
	for d := range sleepScoreByDay {
		daySet[d] = struct{}{}
	}
	days := make([]string, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Strings(days)

	for _, d := range days {
		res.Days = append(res.Days, recoveryDay{
			Date:       d,
			LoadMin:    roundN(loadByDay[d], 1),
			RestingHR:  restingByDay[d],
			SleepScore: sleepScoreByDay[d],
		})
	}

	res.Summary = recoveryTrend(res.Days)
	return res, nil
}

// recoveryTrend compares the most recent half of the window against the earlier
// half: it flags when training load is rising while recovery (sleep score,
// resting HR) is deteriorating. Days missing a given marker are excluded from
// that marker's average (no zero-padding of denominators).
func recoveryTrend(days []recoveryDay) recoverySummary {
	if len(days) == 0 {
		return recoverySummary{Trend: "no data", Flag: false}
	}
	if len(days) == 1 {
		d := days[0]
		// Single high-load, low-recovery day is itself a flag.
		flag := d.LoadMin >= 45 && d.SleepScore > 0 && d.SleepScore < 50
		return recoverySummary{Trend: "single-day", Flag: flag}
	}

	mid := len(days) / 2
	early := days[:mid]
	late := days[mid:]

	loadEarly := avgLoad(early)
	loadLate := avgLoad(late)
	sleepEarly, hasSleepEarly := avgSleep(early)
	sleepLate, hasSleepLate := avgSleep(late)
	rhrEarly, hasRHREarly := avgRHR(early)
	rhrLate, hasRHRLate := avgRHR(late)

	loadUp := loadLate > loadEarly*1.05
	recoveryDown := false
	if hasSleepEarly && hasSleepLate && sleepLate < sleepEarly-2 {
		recoveryDown = true
	}
	if hasRHREarly && hasRHRLate && rhrLate > rhrEarly+1 {
		recoveryDown = true
	}

	trend := "stable"
	switch {
	case loadUp && recoveryDown:
		trend = "load up, recovery down"
	case loadUp:
		trend = "load up"
	case loadLate < loadEarly*0.95:
		trend = "load down"
	}
	return recoverySummary{Trend: trend, Flag: loadUp && recoveryDown}
}

func avgLoad(days []recoveryDay) float64 {
	var sum float64
	var n int
	for _, d := range days {
		if d.LoadMin > 0 {
			sum += d.LoadMin
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func avgSleep(days []recoveryDay) (float64, bool) {
	var sum, n float64
	for _, d := range days {
		if d.SleepScore > 0 {
			sum += float64(d.SleepScore)
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / n, true
}

func avgRHR(days []recoveryDay) (float64, bool) {
	var sum, n float64
	for _, d := range days {
		if d.RestingHR > 0 {
			sum += float64(d.RestingHR)
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / n, true
}
