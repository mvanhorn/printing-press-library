// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `digest` — one structured "what changed since <time>" snapshot across all
// metrics, built for piping into an agent.
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

// digestResult is the JSON shape emitted by `digest`.
type digestResult struct {
	Since          string   `json:"since"`
	LatestWeight   *float64 `json:"latest_weight"`
	WeightChange   *float64 `json:"weight_change"`
	StepsToday     int      `json:"steps_today"`
	LastSleepScore *int     `json:"last_sleep_score"`
	RestingHR      *int     `json:"resting_hr"`
	NewAfibEvents  int      `json:"new_afib_events"`
	NewBPReadings  int      `json:"new_bp_readings"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "One structured 'what changed since <time> across all my metrics' snapshot — built for piping into an agent.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Summarizes everything that changed across weight, activity, sleep, and heart
since a cutoff (default 24h) into one JSON object — built for piping into an
agent.

Reads local data only. Sync first with:
  withings-pp-cli sync --resources measure,activity,sleep,heart`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseSinceFlag(flagSince, 24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-since)

			db, handled, err := openLocalForAnalytics(cmd, flags, dbPath, "measure,activity,sleep,heart", false)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			res, err := computeDigest(db, cutoff, flagSinceLabel(flagSince, "24h"))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Lookback window (e.g. 24h, 3d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	return cmd
}

// computeDigest assembles the cross-metric snapshot from the local store.
func computeDigest(db *store.Store, cutoff time.Time, sinceLabel string) (digestResult, error) {
	res := digestResult{Since: sinceLabel}
	cutYMD := cutoff.UTC().Format("2006-01-02")

	// Weight: latest in window + change over window.
	groups, err := loadMeasureGroups(db, cutoff)
	if err != nil {
		return res, err
	}
	var weights []float64
	var bpReadings int
	var latestRestingHR int
	var haveRestingHR bool
	var latestRestingEpoch int64
	for _, g := range groups {
		if w, ok := g.scaledOfType(1); ok {
			weights = append(weights, w)
		}
		_, hasSys := g.scaledOfType(10)
		_, hasDia := g.scaledOfType(9)
		if hasSys || hasDia {
			bpReadings++
		}
		if hp, ok := g.scaledOfType(11); ok {
			// Track the resting HR from the most recent group carrying a pulse.
			if !haveRestingHR || g.Date >= latestRestingEpoch {
				latestRestingHR = int(hp)
				latestRestingEpoch = g.Date
				haveRestingHR = true
			}
		}
	}
	if len(weights) > 0 {
		end := roundN(weights[len(weights)-1], 3)
		res.LatestWeight = &end
		change := roundN(weights[len(weights)-1]-weights[0], 3)
		res.WeightChange = &change
	}
	res.NewBPReadings = bpReadings

	// Activity: steps for the most recent day in window.
	aRows, err := localRows(db, "activity")
	if err != nil {
		return res, err
	}
	stepsByDay := map[string]int{}
	var activityRestingHR int
	var activityRestingDate string
	for _, raw := range aRows {
		var a struct {
			Date  string `json:"date"`
			Steps int    `json:"steps"`
			HRMin int    `json:"hr_min"`
		}
		if json.Unmarshal(raw, &a) != nil {
			continue
		}
		if a.Date == "" || a.Date < cutYMD {
			continue
		}
		stepsByDay[a.Date] += a.Steps
		// Activity hr_min is a resting-HR fallback when no measure pulse exists.
		// Track by most-recent activity date (rows arrive in sync order, not
		// date order), so a partial re-sync of older rows can't surface a
		// stale value.
		if a.HRMin > 0 && a.Date >= activityRestingDate {
			activityRestingHR = a.HRMin
			activityRestingDate = a.Date
		}
	}
	if len(stepsByDay) > 0 {
		days := make([]string, 0, len(stepsByDay))
		for d := range stepsByDay {
			days = append(days, d)
		}
		sort.Strings(days)
		res.StepsToday = stepsByDay[days[len(days)-1]]
	}
	// Apply the activity fallback only when no measure-derived pulse was found.
	if !haveRestingHR && activityRestingDate != "" {
		latestRestingHR = activityRestingHR
		haveRestingHR = true
	}
	if haveRestingHR {
		rhr := latestRestingHR
		res.RestingHR = &rhr
	}

	// Sleep: most recent sleep score in window.
	res.LastSleepScore = latestSleepScore(db, cutYMD)

	// Heart: count AFib events in window.
	res.NewAfibEvents = countAfibEvents(db, cutoff)

	return res, nil
}

// latestSleepScore returns the sleep score of the most recent night at/after
// cutYMD, or nil when none.
func latestSleepScore(db *store.Store, cutYMD string) *int {
	rows, err := localRows(db, "sleep")
	if err != nil {
		return nil
	}
	bestDay := ""
	bestScore := 0
	found := false
	for _, raw := range rows {
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
		if day == "" || day < cutYMD || s.Data.SleepScore <= 0 {
			continue
		}
		if !found || day >= bestDay {
			bestDay = day
			bestScore = s.Data.SleepScore
			found = true
		}
	}
	if !found {
		return nil
	}
	return &bestScore
}

// countAfibEvents counts heart recordings with a positive AFib classification
// at/after cutoff.
func countAfibEvents(db *store.Store, cutoff time.Time) int {
	rows, err := localRows(db, "heart")
	if err != nil {
		return 0
	}
	cutYMD := cutoff.UTC().Format("2006-01-02")
	count := 0
	for _, raw := range rows {
		var h struct {
			Timestamp int64 `json:"timestamp"`
			Data      struct {
				ECG struct {
					Afib int `json:"afib"`
				} `json:"ecg"`
			} `json:"data"`
			ECG struct {
				Afib int `json:"afib"`
			} `json:"ecg"`
		}
		if json.Unmarshal(raw, &h) != nil {
			continue
		}
		day := epochToYMD(h.Timestamp)
		if day == "" || day < cutYMD {
			continue
		}
		afib := h.Data.ECG.Afib
		if afib == 0 {
			afib = h.ECG.Afib
		}
		if afib > 0 {
			count++
		}
	}
	return count
}
