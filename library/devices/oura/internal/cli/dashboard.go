// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type dashboardView struct {
	Date              string  `json:"date"`
	SleepScore        float64 `json:"sleep_score,omitempty"`
	HasSleepScore     bool    `json:"has_sleep_score"`
	ReadinessScore    float64 `json:"readiness_score,omitempty"`
	HasReadinessScore bool    `json:"has_readiness_score"`
	ActivityScore     float64 `json:"activity_score,omitempty"`
	HasActivityScore  bool    `json:"has_activity_score"`
	StressLevel       float64 `json:"stress_level,omitempty"`
	HasStressLevel    bool    `json:"has_stress_level"`
	SpO2Average       float64 `json:"spo2_average,omitempty"`
	HasSpO2Average    bool    `json:"has_spo2_average"`
	HRV               float64 `json:"hrv,omitempty"`
	HasHRV            bool    `json:"has_hrv"`
	ReadinessBand     string  `json:"readiness_baseline_band,omitempty"`
	Note              string  `json:"note,omitempty"`
}

func newDashboardCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Combined sleep, readiness, activity, stress, and SpO2 in one view — richer than a single daily summary call",
		Long: `One-call morning view combining sleep, readiness, activity, stress, and
SpO2 scores for a day, plus overnight HRV and a personal-baseline band on
readiness, instead of separate calls per metric.`,
		Example: `  oura-pp-cli dashboard
  oura-pp-cli dashboard --agent --select date,sleep_score,readiness_score,activity_score,stress_level,spo2_average`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show combined sleep/readiness/activity/stress/SpO2 dashboard for --date")
				return nil
			}
			date := flagDate
			if date == "" {
				date = today()
			}

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprint(cmd.ErrOrStderr(), missingMirrorMessage(dbPath))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view := dashboardView{Date: date}
			anyValue := false

			for _, m := range []struct {
				name string
				set  func(float64)
				has  *bool
			}{
				{"sleep", func(v float64) { view.SleepScore = round2(v) }, &view.HasSleepScore},
				{"readiness", func(v float64) { view.ReadinessScore = round2(v) }, &view.HasReadinessScore},
				{"activity", func(v float64) { view.ActivityScore = round2(v) }, &view.HasActivityScore},
				{"hrv", func(v float64) { view.HRV = round2(v) }, &view.HasHRV},
			} {
				spec, _ := resolveMetric(m.name)
				series, err := metricSeries(db, spec, date, date)
				if err != nil {
					return fmt.Errorf("querying %s: %w", m.name, err)
				}
				if v, ok := series[date]; ok {
					m.set(v)
					*m.has = true
					anyValue = true
				}
			}

			if v, ok, err := dailyStressHigh(db, date); err != nil {
				return fmt.Errorf("querying stress: %w", err)
			} else if ok {
				view.StressLevel = round2(v)
				view.HasStressLevel = true
				anyValue = true
			}

			if v, ok, err := genericDayField(db, "daily-spo2", date, "spo2_percentage", "average"); err != nil {
				return fmt.Errorf("querying spo2: %w", err)
			} else if ok {
				view.SpO2Average = round2(v)
				view.HasSpO2Average = true
				anyValue = true
			}

			if view.HasReadinessScore {
				readinessSpec, _ := resolveMetric("readiness")
				baseline, err := metricSeries(db, readinessSpec, addDays(date, -30), addDays(date, -1))
				if err != nil {
					return fmt.Errorf("querying readiness baseline: %w", err)
				}
				vals := make([]float64, 0, len(baseline))
				for _, v := range baseline {
					vals = append(vals, v)
				}
				if mean, stdDev := meanStdDev(vals); stdDev > 0 {
					z := (view.ReadinessScore - mean) / stdDev
					view.ReadinessBand = bandFor(z)
				}
			}

			if !anyValue {
				view.Note = fmt.Sprintf("no synced data for %s — run 'oura-pp-cli sync' first", date)
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Dashboard for %s\n", date)
			printDashboardLine(out, "sleep", view.SleepScore, view.HasSleepScore)
			printDashboardLine(out, "readiness", view.ReadinessScore, view.HasReadinessScore)
			if view.ReadinessBand != "" {
				fmt.Fprintf(out, "  (readiness baseline band: %s)\n", view.ReadinessBand)
			}
			printDashboardLine(out, "activity", view.ActivityScore, view.HasActivityScore)
			printDashboardLine(out, "stress", view.StressLevel, view.HasStressLevel)
			printDashboardLine(out, "spo2", view.SpO2Average, view.HasSpO2Average)
			printDashboardLine(out, "hrv", view.HRV, view.HasHRV)
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Day to show (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func printDashboardLine(out io.Writer, name string, value float64, has bool) {
	if has {
		fmt.Fprintf(out, "  %-10s %.1f\n", name, value)
	} else {
		fmt.Fprintf(out, "  %-10s -\n", name)
	}
}

func dailyStressHigh(db *store.Store, day string) (float64, bool, error) {
	row := db.DB().QueryRow(`SELECT stress_high FROM daily_stress WHERE day = ?`, day)
	var v sql.NullFloat64
	if err := row.Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v.Float64, v.Valid, nil
}

// genericDayField reads a nested numeric field (obj[outerKey][innerKey]) for
// a single day from the generic resources table, used for resource types
// that never got a dedicated typed table (no confirmed flat schema).
func genericDayField(db *store.Store, resourceType, day, outerKey, innerKey string) (float64, bool, error) {
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		if d, _ := obj["day"].(string); d != day {
			continue
		}
		outer, ok := obj[outerKey].(map[string]any)
		if !ok {
			continue
		}
		switch v := outer[innerKey].(type) {
		case float64:
			return v, true, nil
		}
	}
	return 0, false, rows.Err()
}
