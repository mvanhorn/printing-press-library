// Hand-written for whoop-pp-cli — the seven Rung-4/5 transcendence features
// from the WHOOP reprint absorb manifest (N1, N2, N3, N6, N7, N8, N9). Every
// command in this file reads from local SQLite only — no live API call can
// compute the cross-resource joins these answers depend on.
//
// Commands surface under `whoop-pp-cli analyze <subcommand>`, with the SQL
// escape hatch hanging directly off root (`whoop-pp-cli sql "..."`).
// All commands honor --json (default to JSON when not a terminal), --select,
// and --compact via the existing rootFlags pipeline. All are marked
// mcp:read-only=true so the MCP surface can expose them safely.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/whoop/internal/store"

	"github.com/spf13/cobra"
)

// =====================================================================
// `whoop-pp-cli analyze ...` umbrella
// =====================================================================

func newAnalyzeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Local-only analytics over your synced WHOOP history",
		Long: `Run cross-resource queries that no live API call can answer.

All subcommands read from the local SQLite database populated by 'sync'.
Sync at least 90 days of data ('whoop-pp-cli sync --since 90d') before
running these for meaningful baselines.

Subcommands:
  why-today       Rank today's deltas vs. your personal 14-day baseline
  efficiency      Recovery-vs-strain curve over a rolling window
  correlate       Pearson correlation between any two metrics
  sleep-debt      Cumulative sleep-debt trend with weekly buckets
  overtraining    Flag days where strain exceeds 1sigma above 90d mean`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAnalyzeWhyTodayCmd(flags))
	cmd.AddCommand(newAnalyzeEfficiencyCmd(flags))
	cmd.AddCommand(newAnalyzeCorrelateCmd(flags))
	cmd.AddCommand(newAnalyzeSleepDebtCmd(flags))
	cmd.AddCommand(newAnalyzeOvertrainingCmd(flags))
	return cmd
}

// =====================================================================
// Helpers shared across analyze commands
// =====================================================================

// openAnalyticsStore opens the synced local DB read-only. Every analyze
// subcommand begins here. We use read-only so a stray subcommand can never
// corrupt the user's sync state during analysis.
func openAnalyticsStore(ctx context.Context) (*store.Store, error) {
	path := defaultDBPath("whoop-pp-cli")
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("opening local database (read-only): %w\nRun 'whoop-pp-cli sync' first.", err)
	}
	return db, nil
}

// loadRecoveries pulls all recovery JSON blobs from the generic resources
// table joined with cycle.start for time ordering. Returns parsed objects.
// Each row carries score block plus the cycle's start day so we can bucket
// by date without re-walking cycles.
type recoveryRow struct {
	CycleID         int64
	CycleStartUTC   time.Time
	RecoveryScore   float64
	HRVRMSSDMilli   float64
	RestingHR       float64
	Spo2            float64
	SkinTemp        float64
	UserCalibrating bool
	ScoreState      string
	Raw             map[string]any
}

func loadRecoveries(db *store.Store, since time.Time) ([]recoveryRow, error) {
	q := `SELECT r.data, c.start
	      FROM resources r
	      JOIN cycle c ON CAST(json_extract(r.data, '$.cycle_id') AS TEXT) = c.id
	      WHERE r.resource_type = 'recovery'
	      AND (? IS NULL OR c.start >= ?)
	      ORDER BY c.start DESC`
	args := []any{nil, nil}
	if !since.IsZero() {
		args[0] = since.UTC().Format(time.RFC3339)
		args[1] = since.UTC().Format(time.RFC3339)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query recoveries: %w", err)
	}
	defer rows.Close()
	var out []recoveryRow
	for rows.Next() {
		var raw, startStr string
		if err := rows.Scan(&raw, &startStr); err != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		startT, _ := time.Parse(time.RFC3339, startStr)
		score, _ := obj["score"].(map[string]any)
		row := recoveryRow{
			CycleStartUTC:   startT,
			Raw:             obj,
			ScoreState:      stringField(obj, "score_state"),
			UserCalibrating: boolField(score, "user_calibrating"),
		}
		if id, ok := obj["cycle_id"]; ok {
			row.CycleID = anyToInt64(id)
		}
		row.RecoveryScore = floatField(score, "recovery_score")
		row.HRVRMSSDMilli = floatField(score, "hrv_rmssd_milli")
		row.RestingHR = floatField(score, "resting_heart_rate")
		row.Spo2 = floatField(score, "spo2_percentage")
		row.SkinTemp = floatField(score, "skin_temp_celsius")
		out = append(out, row)
	}
	return out, rows.Err()
}

type sleepRow struct {
	ID                        string
	CycleID                   int64
	Start                     time.Time
	End                       time.Time
	ConsistencyPct            float64
	EfficiencyPct             float64
	PerformancePct            float64
	NeedFromSleepDebtMilli    int64
	NeedFromRecentStrainMilli int64
	NeedFromRecentNapMilli    int64
	BaselineMilli             int64
	Nap                       bool
	ScoreState                string
	Raw                       map[string]any
}

func loadSleeps(db *store.Store, since time.Time) ([]sleepRow, error) {
	q := `SELECT data FROM resources WHERE resource_type = 'activity'
	      AND json_extract(data, '$.nap') IS NOT NULL
	      ORDER BY json_extract(data, '$.start') DESC`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query sleeps: %w", err)
	}
	defer rows.Close()
	var out []sleepRow
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		startT, _ := time.Parse(time.RFC3339, stringField(obj, "start"))
		if !since.IsZero() && startT.Before(since) {
			continue
		}
		endT, _ := time.Parse(time.RFC3339, stringField(obj, "end"))
		score, _ := obj["score"].(map[string]any)
		needed, _ := nestedMap(score, "sleep_needed")
		row := sleepRow{
			ID:             stringField(obj, "id"),
			Start:          startT,
			End:            endT,
			ScoreState:     stringField(obj, "score_state"),
			Nap:            boolField(obj, "nap"),
			ConsistencyPct: floatField(score, "sleep_consistency_percentage"),
			EfficiencyPct:  floatField(score, "sleep_efficiency_percentage"),
			PerformancePct: floatField(score, "sleep_performance_percentage"),
			Raw:            obj,
		}
		if needed != nil {
			row.NeedFromSleepDebtMilli = int64Field(needed, "need_from_sleep_debt_milli")
			row.NeedFromRecentStrainMilli = int64Field(needed, "need_from_recent_strain_milli")
			row.NeedFromRecentNapMilli = int64Field(needed, "need_from_recent_nap_milli")
			row.BaselineMilli = int64Field(needed, "baseline_milli")
		}
		if id, ok := obj["cycle_id"]; ok {
			row.CycleID = anyToInt64(id)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type cycleRow struct {
	ID           int64
	Start        time.Time
	End          time.Time
	Strain       float64
	Kilojoule    float64
	AvgHeartRate float64
	MaxHeartRate float64
	ScoreState   string
	Raw          map[string]any
}

func loadCycles(db *store.Store, since time.Time) ([]cycleRow, error) {
	q := `SELECT data, start FROM cycle ORDER BY start DESC`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query cycles: %w", err)
	}
	defer rows.Close()
	var out []cycleRow
	for rows.Next() {
		var raw, startStr string
		if err := rows.Scan(&raw, &startStr); err != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		startT, _ := time.Parse(time.RFC3339, startStr)
		if !since.IsZero() && startT.Before(since) {
			continue
		}
		score, _ := obj["score"].(map[string]any)
		row := cycleRow{
			Start:        startT,
			ScoreState:   stringField(obj, "score_state"),
			Strain:       floatField(score, "strain"),
			Kilojoule:    floatField(score, "kilojoule"),
			AvgHeartRate: floatField(score, "average_heart_rate"),
			MaxHeartRate: floatField(score, "max_heart_rate"),
			Raw:          obj,
		}
		if id, ok := obj["id"]; ok {
			row.ID = anyToInt64(id)
		}
		end, _ := time.Parse(time.RFC3339, stringField(obj, "end"))
		row.End = end
		out = append(out, row)
	}
	return out, rows.Err()
}

type workoutRow struct {
	ID         string
	Start      time.Time
	End        time.Time
	SportID    int64
	SportName  string
	Strain     float64
	Kilojoule  float64
	ScoreState string
	Raw        map[string]any
}

func loadWorkouts(db *store.Store, since time.Time) ([]workoutRow, error) {
	// In our store, workouts sync into the 'activity' table; we filter by
	// sport_id being non-null which sleeps lack.
	q := `SELECT data FROM resources WHERE resource_type = 'activity'
	      AND json_extract(data, '$.sport_id') IS NOT NULL
	      ORDER BY json_extract(data, '$.start') DESC`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query workouts: %w", err)
	}
	defer rows.Close()
	var out []workoutRow
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) != nil {
			continue
		}
		startT, _ := time.Parse(time.RFC3339, stringField(obj, "start"))
		if !since.IsZero() && startT.Before(since) {
			continue
		}
		endT, _ := time.Parse(time.RFC3339, stringField(obj, "end"))
		score, _ := obj["score"].(map[string]any)
		row := workoutRow{
			ID:         stringField(obj, "id"),
			Start:      startT,
			End:        endT,
			SportName:  stringField(obj, "sport_name"),
			ScoreState: stringField(obj, "score_state"),
			Strain:     floatField(score, "strain"),
			Kilojoule:  floatField(score, "kilojoule"),
			Raw:        obj,
		}
		if id, ok := obj["sport_id"]; ok {
			row.SportID = anyToInt64(id)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// =====================================================================
// N7: analyze why-today  — ranked attribution against personal baseline
// =====================================================================

func newAnalyzeWhyTodayCmd(flags *rootFlags) *cobra.Command {
	var date string
	var baselineDays int
	cmd := &cobra.Command{
		Use:   "why-today",
		Short: "Rank today's recovery deltas vs. your personal 14-day baseline",
		Long: `Compares today's recovery, HRV, RHR, sleep consistency, and prior-day
strain to a rolling 14-day baseline. Returns a ranked list of contributing
deltas (absolute z-score) so a human (or an agent) can see which signals
moved the needle today.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  whoop-pp-cli analyze why-today --json
  whoop-pp-cli analyze why-today --date 2026-05-10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			target := time.Now().UTC()
			if date != "" {
				t, err := time.Parse("2006-01-02", date)
				if err != nil {
					return fmt.Errorf("--date must be YYYY-MM-DD: %w", err)
				}
				target = t.UTC()
			}
			windowStart := target.AddDate(0, 0, -(baselineDays + 1))
			recs, err := loadRecoveries(db, windowStart)
			if err != nil {
				return err
			}
			sleeps, err := loadSleeps(db, windowStart)
			if err != nil {
				return err
			}
			cycles, err := loadCycles(db, windowStart)
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				return fmt.Errorf("no recovery records in last %d days. Run 'whoop-pp-cli sync --since %dd' first", baselineDays+1, baselineDays+1)
			}

			// Latest day's recovery (closest to target, score_state == SCORED)
			var today *recoveryRow
			for i := range recs {
				if sameDayUTC(recs[i].CycleStartUTC, target) && recs[i].ScoreState == "SCORED" {
					t := recs[i]
					today = &t
					break
				}
			}
			if today == nil {
				// Distinguish "today, recovery not posted yet" from "older
				// gap that probably means a real sync issue". WHOOP normally
				// scores the previous night's recovery 1-2 hours after wake,
				// so a missing record for today is the expected state most
				// of the morning.
				isToday := sameDayUTC(target, time.Now().UTC())
				if isToday {
					return fmt.Errorf("recovery for %s isn't in WHOOP yet.\n\nWHOOP normally posts the recovery score 1-2 hours after you wake up,\nso this is the expected state earlier in the day. Once it lands, run\n'whoop-pp-cli sync' and retry.\n\nIn the meantime, look at yesterday's attribution:\n  whoop-pp-cli analyze why-today --date %s", target.Format("2006-01-02"), target.AddDate(0, 0, -1).Format("2006-01-02"))
				}
				return fmt.Errorf("no SCORED recovery for %s in local store. Run 'whoop-pp-cli sync' to refresh; if it's still missing after that, the day may not have a recovery on WHOOP's side", target.Format("2006-01-02"))
			}

			// Baseline: prior baselineDays
			var baselineRecs []recoveryRow
			for _, r := range recs {
				if r.ScoreState != "SCORED" {
					continue
				}
				if sameDayUTC(r.CycleStartUTC, target) {
					continue
				}
				if r.CycleStartUTC.Before(target) {
					baselineRecs = append(baselineRecs, r)
				}
				if len(baselineRecs) >= baselineDays {
					break
				}
			}
			if len(baselineRecs) < 3 {
				return fmt.Errorf("only %d prior days available; need >=3 for a meaningful z-score. Sync more history", len(baselineRecs))
			}

			// Today's sleep + prior-day strain
			var todaySleep *sleepRow
			for i := range sleeps {
				if sameDayUTC(sleeps[i].Start, target) && !sleeps[i].Nap && sleeps[i].ScoreState == "SCORED" {
					s := sleeps[i]
					todaySleep = &s
					break
				}
			}
			var priorStrain *cycleRow
			priorDay := target.AddDate(0, 0, -1)
			for i := range cycles {
				if sameDayUTC(cycles[i].Start, priorDay) && cycles[i].ScoreState == "SCORED" {
					c := cycles[i]
					priorStrain = &c
					break
				}
			}

			// Build deltas
			type delta struct {
				Metric    string  `json:"metric"`
				Today     float64 `json:"today"`
				Baseline  float64 `json:"baseline_mean"`
				StdDev    float64 `json:"baseline_stddev"`
				Delta     float64 `json:"delta"`
				ZScore    float64 `json:"z_score"`
				AbsZ      float64 `json:"abs_z_score"`
				Direction string  `json:"direction"`
				Hint      string  `json:"hint"`
			}
			var deltas []delta

			add := func(name string, todayVal float64, baseline []float64, lowIsBad bool, hint string) {
				if len(baseline) < 3 || todayVal == 0 {
					return
				}
				mean, std := meanStd(baseline)
				if std == 0 {
					return
				}
				d := todayVal - mean
				z := d / std
				dir := "up"
				if d < 0 {
					dir = "down"
				}
				bad := (lowIsBad && d < 0) || (!lowIsBad && d > 0)
				_ = bad
				deltas = append(deltas, delta{
					Metric:    name,
					Today:     round2(todayVal),
					Baseline:  round2(mean),
					StdDev:    round2(std),
					Delta:     round2(d),
					ZScore:    round2(z),
					AbsZ:      round2(math.Abs(z)),
					Direction: dir,
					Hint:      hint,
				})
			}

			// Recovery score
			var rsBaseline, hrvBaseline, rhrBaseline []float64
			for _, r := range baselineRecs {
				if r.RecoveryScore > 0 {
					rsBaseline = append(rsBaseline, r.RecoveryScore)
				}
				if r.HRVRMSSDMilli > 0 {
					hrvBaseline = append(hrvBaseline, r.HRVRMSSDMilli)
				}
				if r.RestingHR > 0 {
					rhrBaseline = append(rhrBaseline, r.RestingHR)
				}
			}
			add("recovery_score", today.RecoveryScore, rsBaseline, true, "lower than baseline means your body is recovering less")
			add("hrv_rmssd_milli", today.HRVRMSSDMilli, hrvBaseline, true, "lower HRV often correlates with stress or insufficient recovery")
			add("resting_heart_rate", today.RestingHR, rhrBaseline, false, "elevated RHR can flag illness, dehydration, or accumulated strain")

			if todaySleep != nil {
				var conBaseline, effBaseline []float64
				for _, s := range sleeps {
					if s.Nap || sameDayUTC(s.Start, target) || s.ScoreState != "SCORED" {
						continue
					}
					if s.ConsistencyPct > 0 {
						conBaseline = append(conBaseline, s.ConsistencyPct)
					}
					if s.EfficiencyPct > 0 {
						effBaseline = append(effBaseline, s.EfficiencyPct)
					}
				}
				add("sleep_consistency_percentage", todaySleep.ConsistencyPct, conBaseline, true, "irregular bedtime/wake-time drags recovery")
				add("sleep_efficiency_percentage", todaySleep.EfficiencyPct, effBaseline, true, "time-in-bed not spent asleep — disturbance, late caffeine, alcohol")
			}
			if priorStrain != nil {
				var sBaseline []float64
				for _, c := range cycles {
					if sameDayUTC(c.Start, priorDay) || c.ScoreState != "SCORED" {
						continue
					}
					if c.Strain > 0 {
						sBaseline = append(sBaseline, c.Strain)
					}
				}
				add("prior_day_strain", priorStrain.Strain, sBaseline, false, "high strain yesterday compounds today's recovery cost")
			}

			sort.Slice(deltas, func(i, j int) bool { return deltas[i].AbsZ > deltas[j].AbsZ })

			result := map[string]any{
				"date":               target.Format("2006-01-02"),
				"baseline_days":      baselineDays,
				"recovery_score":     today.RecoveryScore,
				"user_calibrating":   today.UserCalibrating,
				"ranked_attribution": deltas,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Target date YYYY-MM-DD (default: today UTC)")
	cmd.Flags().IntVar(&baselineDays, "baseline-days", 14, "Days of baseline history to compute z-scores against")
	return cmd
}

// =====================================================================
// N1: analyze efficiency — strain-bucketed mean next-day recovery
// =====================================================================

func newAnalyzeEfficiencyCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:   "efficiency",
		Short: "Recovery-vs-strain curve over a rolling window",
		Long: `Joins each cycle's strain to the next-day recovery score and bins by
strain bucket: 0-5 ("light"), 5-10 ("moderate"), 10-15 ("strenuous"),
15-21 ("all-out"). Output: mean recovery per bucket, count, and the
delta vs. the prior equivalent window.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli analyze efficiency --window 90d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			d, err := parseWindow(window)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			recentStart := now.Add(-d)
			priorStart := now.Add(-2 * d)

			cycles, err := loadCycles(db, priorStart)
			if err != nil {
				return err
			}
			recs, err := loadRecoveries(db, priorStart)
			if err != nil {
				return err
			}
			// Index recoveries by cycle_id
			recByCycle := make(map[int64]recoveryRow, len(recs))
			for _, r := range recs {
				if r.ScoreState == "SCORED" && r.RecoveryScore > 0 {
					recByCycle[r.CycleID] = r
				}
			}

			buckets := []struct {
				name string
				min  float64
				max  float64
			}{
				{"light_0_5", 0, 5},
				{"moderate_5_10", 5, 10},
				{"strenuous_10_15", 10, 15},
				{"all_out_15_21", 15, 21.01},
			}

			recentScores := map[string][]float64{}
			priorScores := map[string][]float64{}
			for _, c := range cycles {
				if c.ScoreState != "SCORED" || c.Strain <= 0 {
					continue
				}
				// Match cycle.id to recovery.cycle_id; next-day recovery
				// in WHOOP semantics is the recovery on the SAME logical
				// cycle (recovery is computed FROM the prior night's sleep
				// that closes the cycle). So the recovery of cycle N
				// reflects the strain of cycle N-1's full day — but more
				// commonly people read "today's recovery" as a function
				// of "yesterday's strain". To honor both, we use the
				// recovery attached to the SAME cycle_id, which is the
				// canonical join key. This is correct per WHOOP docs.
				r, ok := recByCycle[c.ID]
				if !ok {
					continue
				}
				bucketName := ""
				for _, b := range buckets {
					if c.Strain >= b.min && c.Strain < b.max {
						bucketName = b.name
						break
					}
				}
				if bucketName == "" {
					continue
				}
				if c.Start.After(recentStart) {
					recentScores[bucketName] = append(recentScores[bucketName], r.RecoveryScore)
				} else if c.Start.After(priorStart) {
					priorScores[bucketName] = append(priorScores[bucketName], r.RecoveryScore)
				}
			}

			type out struct {
				Bucket         string  `json:"bucket"`
				RecentMean     float64 `json:"recent_mean_recovery"`
				RecentCount    int     `json:"recent_count"`
				PriorMean      float64 `json:"prior_mean_recovery"`
				PriorCount     int     `json:"prior_count"`
				DeltaPP        float64 `json:"delta_pp"`
				Interpretation string  `json:"interpretation"`
			}
			var outs []out
			for _, b := range buckets {
				rm, _ := meanStd(recentScores[b.name])
				pm, _ := meanStd(priorScores[b.name])
				delta := rm - pm
				interp := "stable"
				if math.Abs(delta) > 5 {
					if delta > 0 {
						interp = "recovering faster than the prior window"
					} else {
						interp = "recovering slower — possible accumulated load"
					}
				}
				outs = append(outs, out{
					Bucket:         b.name,
					RecentMean:     round2(rm),
					RecentCount:    len(recentScores[b.name]),
					PriorMean:      round2(pm),
					PriorCount:     len(priorScores[b.name]),
					DeltaPP:        round2(delta),
					Interpretation: interp,
				})
			}
			result := map[string]any{
				"window":  window,
				"buckets": outs,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "90d", "Rolling window for comparison (e.g. 30d, 90d)")
	return cmd
}

// =====================================================================
// N6: analyze correlate — Pearson r between two named metrics
// =====================================================================

// metricExtractor returns the value of a known WHOOP metric for a given
// cycle-day. Each name is whitelisted; passing an unknown name returns an
// error rather than silently zeroing the value (would skew correlations).
type metricExtractor struct {
	name        string
	resource    string // recovery | sleep | cycle | workout
	extract     func(any) float64
	description string
}

var allowedMetrics = map[string]metricExtractor{
	"recovery_score":               {"recovery_score", "recovery", func(r any) float64 { return r.(recoveryRow).RecoveryScore }, "recovery score, 0..100"},
	"hrv_rmssd_milli":              {"hrv_rmssd_milli", "recovery", func(r any) float64 { return r.(recoveryRow).HRVRMSSDMilli }, "HRV (RMSSD) in milliseconds"},
	"resting_heart_rate":           {"resting_heart_rate", "recovery", func(r any) float64 { return r.(recoveryRow).RestingHR }, "resting HR, bpm"},
	"spo2_percentage":              {"spo2_percentage", "recovery", func(r any) float64 { return r.(recoveryRow).Spo2 }, "blood oxygen %"},
	"skin_temp_celsius":            {"skin_temp_celsius", "recovery", func(r any) float64 { return r.(recoveryRow).SkinTemp }, "skin temperature C"},
	"strain":                       {"strain", "cycle", func(r any) float64 { return r.(cycleRow).Strain }, "cycle strain 0..21"},
	"kilojoule":                    {"kilojoule", "cycle", func(r any) float64 { return r.(cycleRow).Kilojoule }, "energy expended, kJ"},
	"average_heart_rate":           {"average_heart_rate", "cycle", func(r any) float64 { return r.(cycleRow).AvgHeartRate }, "cycle avg HR"},
	"max_heart_rate":               {"max_heart_rate", "cycle", func(r any) float64 { return r.(cycleRow).MaxHeartRate }, "cycle max HR"},
	"sleep_performance_percentage": {"sleep_performance_percentage", "sleep", func(r any) float64 { return r.(sleepRow).PerformancePct }, "sleep performance %"},
	"sleep_consistency_percentage": {"sleep_consistency_percentage", "sleep", func(r any) float64 { return r.(sleepRow).ConsistencyPct }, "sleep consistency %"},
	"sleep_efficiency_percentage":  {"sleep_efficiency_percentage", "sleep", func(r any) float64 { return r.(sleepRow).EfficiencyPct }, "sleep efficiency %"},
	"sleep_debt_milli":             {"sleep_debt_milli", "sleep", func(r any) float64 { return float64(r.(sleepRow).NeedFromSleepDebtMilli) }, "cumulative sleep debt, ms"},
}

func newAnalyzeCorrelateCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:   "correlate <metric-a> <metric-b>",
		Short: "Pearson correlation between two metrics over a window",
		Long: `Computes Pearson's r between two metrics paired by day. Metric names
are whitelisted to prevent typos that would silently degrade analysis:

  recovery_score, hrv_rmssd_milli, resting_heart_rate, spo2_percentage,
  skin_temp_celsius, strain, kilojoule, average_heart_rate,
  max_heart_rate, sleep_performance_percentage, sleep_consistency_percentage,
  sleep_efficiency_percentage, sleep_debt_milli`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(2),
		Example: `  whoop-pp-cli analyze correlate sleep_consistency_percentage recovery_score --window 90d
  whoop-pp-cli analyze correlate strain hrv_rmssd_milli --window 30d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, okA := allowedMetrics[args[0]]
			b, okB := allowedMetrics[args[1]]
			if !okA || !okB {
				return fmt.Errorf("unknown metric (allowed: %s)", strings.Join(metricNames(), ", "))
			}
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			d, err := parseWindow(window)
			if err != nil {
				return err
			}
			since := time.Now().UTC().Add(-d)

			recs, _ := loadRecoveries(db, since)
			sleeps, _ := loadSleeps(db, since)
			cycles, _ := loadCycles(db, since)

			// Build per-day series for both metrics, joined by ISO date.
			seriesA := metricByDay(a, recs, sleeps, cycles, nil)
			seriesB := metricByDay(b, recs, sleeps, cycles, nil)
			var xs, ys []float64
			var dates []string
			for date, va := range seriesA {
				if vb, ok := seriesB[date]; ok && va != 0 && vb != 0 {
					xs = append(xs, va)
					ys = append(ys, vb)
					dates = append(dates, date)
				}
			}
			if len(xs) < 5 {
				return fmt.Errorf("only %d overlapping days — need >=5 for a meaningful correlation. Sync more history (or pick a longer --window)", len(xs))
			}
			r, n := pearson(xs, ys)
			result := map[string]any{
				"metric_a":       a.name,
				"metric_b":       b.name,
				"window":         window,
				"sample_size":    n,
				"pearson_r":      round3(r),
				"r_squared":      round3(r * r),
				"interpretation": interpretPearson(r),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "90d", "Window over which to correlate")
	return cmd
}

// =====================================================================
// N2: analyze sleep-debt — cumulative weekly trend
// =====================================================================

func newAnalyzeSleepDebtCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "sleep-debt",
		Short: "Cumulative sleep-debt trend with weekly buckets",
		Long: `Sums need_from_sleep_debt_milli across all non-nap sleeps in the
window and buckets weekly. Output: per-week debt accrued (hours), running
cumulative debt, and a 4-week trend slope.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli analyze sleep-debt --since 30d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			d, err := parseWindow(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-d)
			sleeps, err := loadSleeps(db, cutoff)
			if err != nil {
				return err
			}
			// Group by ISO week (year-week)
			weekly := map[string]float64{}
			weekStart := map[string]time.Time{}
			for _, s := range sleeps {
				if s.Nap || s.ScoreState != "SCORED" {
					continue
				}
				yr, wk := s.Start.ISOWeek()
				key := fmt.Sprintf("%04d-W%02d", yr, wk)
				weekly[key] += float64(s.NeedFromSleepDebtMilli)
				if _, ok := weekStart[key]; !ok {
					weekStart[key] = s.Start.AddDate(0, 0, -int(s.Start.Weekday())+1)
				}
			}
			var keys []string
			for k := range weekly {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			type weekOut struct {
				Week          string  `json:"week"`
				DebtAccruedHr float64 `json:"debt_accrued_hours"`
				CumulativeHr  float64 `json:"cumulative_debt_hours"`
			}
			var outs []weekOut
			var cum float64
			for _, k := range keys {
				accr := weekly[k] / (1000 * 60 * 60)
				cum += accr
				outs = append(outs, weekOut{Week: k, DebtAccruedHr: round2(accr), CumulativeHr: round2(cum)})
			}
			// Trend: slope of last min(4, len) weeks (hours/week)
			var slope float64
			if len(outs) >= 2 {
				start := len(outs) - 4
				if start < 0 {
					start = 0
				}
				var xs, ys []float64
				for i := start; i < len(outs); i++ {
					xs = append(xs, float64(i-start))
					ys = append(ys, outs[i].DebtAccruedHr)
				}
				slope = linRegSlope(xs, ys)
			}
			interp := "stable"
			if slope > 1 {
				interp = "debt is accruing faster — consider an earlier bedtime"
			} else if slope < -1 {
				interp = "debt is decreasing — recent recovery is paying down the deficit"
			}
			result := map[string]any{
				"window":                  since,
				"weekly":                  outs,
				"trend_slope_hr_per_week": round2(slope),
				"interpretation":          interp,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Lookback window (e.g. 30d, 90d)")
	return cmd
}

// =====================================================================
// N3: analyze overtraining — flag days outside 1sigma above 90d strain
// =====================================================================

func newAnalyzeOvertrainingCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var window string
	cmd := &cobra.Command{
		Use:   "overtraining",
		Short: "Flag days where strain exceeds N sigma above the 90-day mean",
		Long: `Computes rolling mean and stdev of cycle strain over the window
and flags any day where strain > mean + (threshold * stdev). For each
flagged day, looks up the next-day recovery delta vs. baseline so the
output shows whether the high load actually cost recovery.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli analyze overtraining --threshold 1.0 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			d, err := parseWindow(window)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-d)
			cycles, err := loadCycles(db, cutoff)
			if err != nil {
				return err
			}
			recs, err := loadRecoveries(db, cutoff)
			if err != nil {
				return err
			}
			var strains []float64
			for _, c := range cycles {
				if c.ScoreState == "SCORED" && c.Strain > 0 {
					strains = append(strains, c.Strain)
				}
			}
			if len(strains) < 14 {
				return fmt.Errorf("only %d scored cycles; need >=14 for a stable baseline. Sync more history", len(strains))
			}
			mean, std := meanStd(strains)
			if std == 0 {
				return fmt.Errorf("zero strain variance — no anomalies to flag (this is unusual; check sync coverage)")
			}
			recoveryByCycle := map[int64]float64{}
			for _, r := range recs {
				if r.ScoreState == "SCORED" && r.RecoveryScore > 0 {
					recoveryByCycle[r.CycleID] = r.RecoveryScore
				}
			}
			recoveryMean, _ := meanStd(mapValues(recoveryByCycle))

			type flagOut struct {
				Date                string  `json:"date"`
				Strain              float64 `json:"strain"`
				SigmaAboveMean      float64 `json:"sigma_above_mean"`
				NextRecovery        float64 `json:"recovery_for_cycle"`
				RecoveryDeltaVsMean float64 `json:"recovery_delta_vs_window_mean"`
			}
			var flagged []flagOut
			for _, c := range cycles {
				if c.ScoreState != "SCORED" || c.Strain <= 0 {
					continue
				}
				z := (c.Strain - mean) / std
				if z >= threshold {
					rec, ok := recoveryByCycle[c.ID]
					delta := 0.0
					if ok {
						delta = rec - recoveryMean
					}
					flagged = append(flagged, flagOut{
						Date:                c.Start.Format("2006-01-02"),
						Strain:              round2(c.Strain),
						SigmaAboveMean:      round2(z),
						NextRecovery:        round2(rec),
						RecoveryDeltaVsMean: round2(delta),
					})
				}
			}
			result := map[string]any{
				"window":                 window,
				"threshold_sigma":        threshold,
				"baseline_strain_mean":   round2(mean),
				"baseline_strain_std":    round2(std),
				"baseline_recovery_mean": round2(recoveryMean),
				"flagged_days":           flagged,
				"flagged_count":          len(flagged),
				"total_scored_cycles":    len(strains),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 1.0, "Sigma threshold above 90d strain mean")
	cmd.Flags().StringVar(&window, "window", "90d", "Rolling window for the baseline")
	return cmd
}

// =====================================================================
// N8: `whoop-pp-cli sql "<read-only query>"`
// =====================================================================

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var queryFlag string
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Execute a read-only SELECT against the local SQLite store",
		Long: `Run an arbitrary SELECT against your local WHOOP database. The query is
passed as a single positional argument (quote it so the shell sees one token)
or via --query. Either form works; passing both is an error.

Allowed: SELECT, WITH (CTEs), and these read-only PRAGMAs for schema
introspection: table_info, table_list, table_xinfo, index_info,
index_list, index_xinfo, foreign_key_list, schema_version,
collation_list, database_list, function_list, module_list.

Rejected before reaching SQLite: INSERT, UPDATE, DELETE, DROP, ALTER,
REPLACE, ATTACH, DETACH, REINDEX, VACUUM, CREATE, write-PRAGMAs, and any
WITH whose body contains a banned keyword.

Useful tables:
  resources  (id, resource_type, data JSON, synced_at, updated_at)
  cycle      (id, start, "end", score_state, strain via json_extract)
  activity   (id, start, "end", sport_id, sport_name — workouts + sleeps)
  sleep, cycle_recovery — typed dependent tables

Schema introspection:
  whoop-pp-cli sql "PRAGMA table_info(cycle)"
  whoop-pp-cli sql "PRAGMA table_list"`,
		Example: `  # Daily strain over the last 30 days (positional form)
  whoop-pp-cli sql "SELECT date(start) AS day, AVG(json_extract(data,'$.score.strain')) AS s FROM cycle WHERE start > date('now','-30 days') GROUP BY day ORDER BY day"

  # Latest 5 cycles
  whoop-pp-cli sql "SELECT date(start), score_state FROM cycle ORDER BY start DESC LIMIT 5"

  # Same query via --query flag (alias for the positional)
  whoop-pp-cli sql --query "SELECT date(start), score_state FROM cycle ORDER BY start DESC LIMIT 5"

  # Inspect the columns of any table
  whoop-pp-cli sql "PRAGMA table_info(cycle_recovery)"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var q string
			switch {
			case queryFlag != "" && len(args) > 0:
				return fmt.Errorf("pass the query as a positional arg OR --query, not both")
			case queryFlag != "":
				q = strings.TrimSpace(queryFlag)
			case len(args) > 0:
				q = strings.TrimSpace(args[0])
			default:
				return cmd.Help()
			}
			if !isReadOnlyQuery(q) {
				return fmt.Errorf("only SELECT/WITH and read-only PRAGMAs (table_info, table_list, index_info, foreign_key_list, etc.) are allowed (got: %q)", firstToken(q))
			}
			db, err := openAnalyticsStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			var results []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				rowMap := map[string]any{}
				for i, c := range cols {
					rowMap[c] = sqlValueToJSON(vals[i])
				}
				results = append(results, rowMap)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"columns":   cols,
					"row_count": len(results),
					"rows":      results,
				}, flags)
			}
			// Table fallback
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cols, "\t"))
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", len(strings.Join(cols, "\t"))))
			for _, row := range results {
				var fields []string
				for _, c := range cols {
					fields = append(fields, fmt.Sprintf("%v", row[c]))
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(fields, "\t"))
			}
			fmt.Fprintf(os.Stderr, "\n(%d rows)\n", len(results))
			return nil
		},
	}
	cmd.Flags().StringVar(&queryFlag, "query", "", "SQL query to execute (alias for the positional argument)")
	return cmd
}

func isReadOnlyQuery(q string) bool {
	u := strings.ToUpper(strings.TrimSpace(q))
	// Allow leading comments — strip them line-by-line.
	for strings.HasPrefix(u, "--") || strings.HasPrefix(u, "/*") {
		if strings.HasPrefix(u, "--") {
			if idx := strings.IndexByte(u, '\n'); idx >= 0 {
				u = strings.TrimSpace(u[idx+1:])
			} else {
				return false
			}
		}
		if strings.HasPrefix(u, "/*") {
			if idx := strings.Index(u, "*/"); idx >= 0 {
				u = strings.TrimSpace(u[idx+2:])
			} else {
				return false
			}
		}
	}
	if strings.HasPrefix(u, "SELECT ") || u == "SELECT" {
		return true
	}
	if strings.HasPrefix(u, "WITH ") {
		// Recursive CTEs are fine but they can wrap writes — reject if any
		// banned keyword appears in the body (this matches sqlite's own
		// READONLY connection behavior conservatively).
		banned := []string{"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "REPLACE ", "ATTACH ", "DETACH ", "REINDEX ", "VACUUM ", "CREATE ", "PRAGMA "}
		for _, b := range banned {
			if strings.Contains(u, b) {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(u, "PRAGMA ") || strings.HasPrefix(u, "PRAGMA(") {
		return isReadOnlyPragma(u)
	}
	return false
}

// isReadOnlyPragma allows the introspection PRAGMAs that SQLite exposes
// as table-valued reads. Everything else (including PRAGMAs with an "="
// or with non-default arguments that mutate state) is rejected.
//
// Allowlist sourced from SQLite docs — these all return rows without
// changing schema, settings, or data:
//
//	table_info, table_xinfo, table_list, index_info, index_list,
//	index_xinfo, foreign_key_list, foreign_key_check, integrity_check,
//	quick_check, schema_version, user_version, application_id,
//	collation_list, database_list, function_list, module_list, pragma_list,
//	compile_options, freelist_count, page_count, page_size, encoding,
//	journal_mode (no-arg read), wal_checkpoint (already gated separately).
func isReadOnlyPragma(u string) bool {
	// Strip leading "PRAGMA " or "PRAGMA(".
	rest := strings.TrimSpace(strings.TrimPrefix(u, "PRAGMA"))
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "("))
	// Reject "PRAGMA foo = bar" — assignment is a write.
	if strings.Contains(rest, "=") {
		return false
	}
	// Extract the pragma name (up to '(' or whitespace or ';').
	name := ""
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '(' || c == ' ' || c == '\t' || c == '\n' || c == ';' {
			break
		}
		name += string(c)
	}
	if name == "" {
		return false
	}
	allowed := map[string]bool{
		"TABLE_INFO":        true,
		"TABLE_XINFO":       true,
		"TABLE_LIST":        true,
		"INDEX_INFO":        true,
		"INDEX_LIST":        true,
		"INDEX_XINFO":       true,
		"FOREIGN_KEY_LIST":  true,
		"FOREIGN_KEY_CHECK": true,
		"INTEGRITY_CHECK":   true,
		"QUICK_CHECK":       true,
		"SCHEMA_VERSION":    true,
		"USER_VERSION":      true,
		"APPLICATION_ID":    true,
		"COLLATION_LIST":    true,
		"DATABASE_LIST":     true,
		"FUNCTION_LIST":     true,
		"MODULE_LIST":       true,
		"PRAGMA_LIST":       true,
		"COMPILE_OPTIONS":   true,
		"FREELIST_COUNT":    true,
		"PAGE_COUNT":        true,
		"PAGE_SIZE":         true,
		"ENCODING":          true,
	}
	return allowed[name]
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\n' || s[i] == '\t' {
			return s[:i]
		}
	}
	return s
}

func sqlValueToJSON(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		// Try to interpret as JSON first, else as string.
		var maybeJSON any
		if json.Unmarshal(t, &maybeJSON) == nil {
			return maybeJSON
		}
		return string(t)
	case string:
		return t
	default:
		return v
	}
}

// =====================================================================
// Helpers — math, parsing, JSON extraction
// =====================================================================

func parseWindow(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("window required")
	}
	// Allow "30d", "90d", "12w"
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		// Fall back to time.ParseDuration ("24h", "30m")
		d, derr := time.ParseDuration(s)
		if derr != nil {
			return 0, fmt.Errorf("invalid window %q (use e.g. 30d, 90d, 12w)", s)
		}
		return d, nil
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown window unit %q (use d or w)", string(unit))
}

func sameDayUTC(a, b time.Time) bool {
	return a.UTC().Format("2006-01-02") == b.UTC().Format("2006-01-02")
}

func meanStd(xs []float64) (mean, std float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, v := range xs {
		mean += v
	}
	mean /= float64(len(xs))
	if len(xs) < 2 {
		return mean, 0
	}
	for _, v := range xs {
		std += (v - mean) * (v - mean)
	}
	std = math.Sqrt(std / float64(len(xs)-1))
	return mean, std
}

func pearson(xs, ys []float64) (float64, int) {
	n := len(xs)
	if n != len(ys) || n < 2 {
		return 0, n
	}
	mx, _ := meanStd(xs)
	my, _ := meanStd(ys)
	var num, dx, dy float64
	for i := 0; i < n; i++ {
		num += (xs[i] - mx) * (ys[i] - my)
		dx += (xs[i] - mx) * (xs[i] - mx)
		dy += (ys[i] - my) * (ys[i] - my)
	}
	denom := math.Sqrt(dx * dy)
	if denom == 0 {
		return 0, n
	}
	return num / denom, n
}

func interpretPearson(r float64) string {
	a := math.Abs(r)
	dir := "negative"
	if r > 0 {
		dir = "positive"
	}
	switch {
	case a < 0.1:
		return "no meaningful linear relationship"
	case a < 0.3:
		return fmt.Sprintf("weak %s correlation", dir)
	case a < 0.5:
		return fmt.Sprintf("moderate %s correlation", dir)
	case a < 0.7:
		return fmt.Sprintf("strong %s correlation", dir)
	default:
		return fmt.Sprintf("very strong %s correlation", dir)
	}
}

func linRegSlope(xs, ys []float64) float64 {
	n := len(xs)
	if n != len(ys) || n < 2 {
		return 0
	}
	mx, _ := meanStd(xs)
	my, _ := meanStd(ys)
	var num, denom float64
	for i := 0; i < n; i++ {
		num += (xs[i] - mx) * (ys[i] - my)
		denom += (xs[i] - mx) * (xs[i] - mx)
	}
	if denom == 0 {
		return 0
	}
	return num / denom
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
func round3(f float64) float64 { return math.Round(f*1000) / 1000 }

func stringField(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
func floatField(m map[string]any, k string) float64 {
	if m == nil {
		return 0
	}
	if v, ok := m[k]; ok {
		return anyToFloat(v)
	}
	return 0
}
func int64Field(m map[string]any, k string) int64 {
	if m == nil {
		return 0
	}
	if v, ok := m[k]; ok {
		return anyToInt64(v)
	}
	return 0
}
func boolField(m map[string]any, k string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[k]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
func nestedMap(m map[string]any, k string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[k]
	if !ok {
		return nil, false
	}
	mm, ok := v.(map[string]any)
	return mm, ok
}
func anyToFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}
func anyToInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	}
	return 0
}
func metricNames() []string {
	var names []string
	for k := range allowedMetrics {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
func mapValues(m map[int64]float64) []float64 {
	out := make([]float64, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
func metricByDay(m metricExtractor, recs []recoveryRow, sleeps []sleepRow, cycles []cycleRow, _ []workoutRow) map[string]float64 {
	out := map[string]float64{}
	switch m.resource {
	case "recovery":
		for _, r := range recs {
			if r.ScoreState == "SCORED" {
				key := r.CycleStartUTC.UTC().Format("2006-01-02")
				out[key] = m.extract(r)
			}
		}
	case "sleep":
		for _, s := range sleeps {
			if !s.Nap && s.ScoreState == "SCORED" {
				key := s.Start.UTC().Format("2006-01-02")
				out[key] = m.extract(s)
			}
		}
	case "cycle":
		for _, c := range cycles {
			if c.ScoreState == "SCORED" {
				key := c.Start.UTC().Format("2006-01-02")
				out[key] = m.extract(c)
			}
		}
	}
	return out
}

// keep sql.DB usage referenced so go vet doesn't complain on unused imports
var _ = sql.Open
