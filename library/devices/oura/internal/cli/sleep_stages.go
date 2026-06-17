// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type stageComparison struct {
	Stage       string  `json:"stage"`
	Night       float64 `json:"night_minutes"`
	PersonalAvg float64 `json:"personal_30day_avg_minutes"`
	DeltaPct    float64 `json:"delta_pct"`
	Notable     bool    `json:"notable"`
}

type sleepStagesView struct {
	Date     string            `json:"date"`
	Stages   []stageComparison `json:"stages"`
	Baseline int               `json:"baseline_nights"`
	Note     string            `json:"note,omitempty"`
}

func newNovelSleepStagesCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sleep-stages",
		Short: "Compare any night's sleep stage durations against your personal 30-day averages",
		Long: `Compares REM, deep, light, and awake durations for a given night against
your own trailing 30-day average for the main sleep period, flagging any
stage that deviates more than 20% from your personal norm.`,
		Example: `  oura-pp-cli sleep-stages
  oura-pp-cli sleep-stages --date 2026-06-10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare a night's sleep stages against the personal 30-day average")
				return nil
			}
			date := flagDate
			if date == "" {
				date = addDays(today(), -1)
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

			night, ok, err := mainSleepNight(db, date)
			if err != nil {
				return fmt.Errorf("querying sleep for %s: %w", date, err)
			}
			view := sleepStagesView{Date: date}
			if !ok {
				view.Note = fmt.Sprintf("no main sleep period synced for %s — run 'oura-pp-cli sync' first", date)
				return emitSleepStages(cmd, flags, view)
			}

			baseline, err := mainSleepBaseline(db, addDays(date, -30), addDays(date, -1))
			if err != nil {
				return fmt.Errorf("querying sleep baseline: %w", err)
			}
			view.Baseline = baseline.count

			stages := []struct {
				name string
				val  sql.NullFloat64
				avg  float64
			}{
				{"rem", night.rem, baseline.remAvg},
				{"deep", night.deep, baseline.deepAvg},
				{"light", night.light, baseline.lightAvg},
				{"awake", night.awake, baseline.awakeAvg},
			}
			for _, s := range stages {
				if !s.val.Valid {
					continue
				}
				nightMin := s.val.Float64 / 60
				avgMin := s.avg / 60
				pct := 0.0
				if avgMin > 0 {
					pct = (nightMin - avgMin) / avgMin * 100
				}
				view.Stages = append(view.Stages, stageComparison{
					Stage:       s.name,
					Night:       round2(nightMin),
					PersonalAvg: round2(avgMin),
					DeltaPct:    round2(pct),
					Notable:     pct > 20 || pct < -20,
				})
			}
			if baseline.count < 7 {
				view.Note = fmt.Sprintf("only %d prior night(s) of baseline data — sync more history for a reliable comparison", baseline.count)
			}

			return emitSleepStages(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Night to compare (YYYY-MM-DD, default yesterday)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type sleepNight struct {
	rem, deep, light, awake sql.NullFloat64
}

func mainSleepNight(db *store.Store, day string) (sleepNight, bool, error) {
	row := db.DB().QueryRow(
		`SELECT rem_sleep_duration, deep_sleep_duration, light_sleep_duration, awake_time
		 FROM sleep WHERE day = ? AND type = 'long_sleep'
		 ORDER BY total_sleep_duration DESC LIMIT 1`,
		day,
	)
	var n sleepNight
	if err := row.Scan(&n.rem, &n.deep, &n.light, &n.awake); err != nil {
		if err == sql.ErrNoRows {
			return n, false, nil
		}
		return n, false, err
	}
	return n, true, nil
}

type sleepBaseline struct {
	count                               int
	remAvg, deepAvg, lightAvg, awakeAvg float64
}

func mainSleepBaseline(db *store.Store, start, end string) (sleepBaseline, error) {
	rows, err := db.DB().Query(
		`SELECT rem_sleep_duration, deep_sleep_duration, light_sleep_duration, awake_time
		 FROM sleep WHERE day >= ? AND day <= ? AND type = 'long_sleep'`,
		start, end,
	)
	if err != nil {
		return sleepBaseline{}, err
	}
	defer rows.Close()

	var rem, deep, light, awake []float64
	for rows.Next() {
		var r, d, l, a sql.NullFloat64
		if err := rows.Scan(&r, &d, &l, &a); err != nil {
			continue
		}
		if r.Valid {
			rem = append(rem, r.Float64)
		}
		if d.Valid {
			deep = append(deep, d.Float64)
		}
		if l.Valid {
			light = append(light, l.Float64)
		}
		if a.Valid {
			awake = append(awake, a.Float64)
		}
	}
	if err := rows.Err(); err != nil {
		return sleepBaseline{}, err
	}
	remAvg, _ := meanStdDev(rem)
	deepAvg, _ := meanStdDev(deep)
	lightAvg, _ := meanStdDev(light)
	awakeAvg, _ := meanStdDev(awake)
	return sleepBaseline{
		count:    len(rem),
		remAvg:   remAvg,
		deepAvg:  deepAvg,
		lightAvg: lightAvg,
		awakeAvg: awakeAvg,
	}, nil
}

func emitSleepStages(cmd *cobra.Command, flags *rootFlags, view sleepStagesView) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Sleep stages for %s (baseline: %d nights)\n", view.Date, view.Baseline)
	for _, s := range view.Stages {
		flag := ""
		if s.Notable {
			flag = "  <- notable"
		}
		fmt.Fprintf(out, "  %-6s %.0fm vs personal avg %.0fm (%+.0f%%)%s\n", s.Stage, s.Night, s.PersonalAvg, s.DeltaPct, flag)
	}
	if view.Note != "" {
		fmt.Fprintln(out, "note:", view.Note)
	}
	return nil
}
