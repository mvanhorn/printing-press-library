// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: recent-window training digest. Not generated.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type digestView struct {
	Days                int     `json:"days"`
	Since               string  `json:"since"`
	Workouts            int     `json:"workouts"`
	TotalDistanceKm     float64 `json:"total_distance_km"`
	TotalTimeHours      float64 `json:"total_time_hours"`
	TotalAscentM        float64 `json:"total_ascent_m"`
	TotalWorkKJ         float64 `json:"total_work_kj"`
	TotalCalories       float64 `json:"total_calories"`
	AvgPowerW           float64 `json:"avg_power_w"`
	EstimatedLoad       float64 `json:"estimated_load"`
	RidesMissingSummary int     `json:"rides_missing_summary"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "One-shot rollup of the last N days of riding",
		Long: "Aggregate ride count, distance, moving time, ascent, work, calories, average\n" +
			"power, and estimated training load over the last N days from the local mirror —\n" +
			"a windowed total no single API call returns (use --days 365 for a year in review).\n" +
			"For the continuous Fitness/Fatigue/Form curve use 'load'; for all-time records use 'bests'.",
		Example:     "  wahoo-pp-cli digest --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "workouts", `{"workouts":0}`) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "workouts") {
				hintIfStale(cmd, db, "workouts", flags.maxAge)
			}
			ws, err := loadWorkouts(db)
			if err != nil {
				return fmt.Errorf("reading workouts: %w", err)
			}
			ftp, err := latestFTP(db)
			if err != nil {
				return fmt.Errorf("reading power zones: %w", err)
			}
			view := computeDigest(ws, ftp, days, time.Now())

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Last %d days (since %s):\n", view.Days, view.Since)
				fmt.Fprintf(out, "  Rides:     %d\n", view.Workouts)
				fmt.Fprintf(out, "  Distance:  %.1f km\n", view.TotalDistanceKm)
				fmt.Fprintf(out, "  Time:      %.1f h\n", view.TotalTimeHours)
				fmt.Fprintf(out, "  Ascent:    %.0f m\n", view.TotalAscentM)
				fmt.Fprintf(out, "  Work:      %.0f kJ\n", view.TotalWorkKJ)
				if view.AvgPowerW > 0 {
					fmt.Fprintf(out, "  Avg power: %.0f W\n", view.AvgPowerW)
				}
				fmt.Fprintf(out, "  Est. load: %.0f\n", view.EstimatedLoad)
				if view.RidesMissingSummary > 0 {
					fmt.Fprintf(out, "  (%d ride(s) had no summary metrics)\n", view.RidesMissingSummary)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Window in days to roll up")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

// computeDigest rolls up workouts within the last `days` of `now`. Missing
// metrics are excluded from their totals (never averaged in as phantom zeros);
// average power is computed only over rides that actually carry power.
func computeDigest(ws []parsedWorkout, ftp float64, days int, now time.Time) digestView {
	cutoff := now.AddDate(0, 0, -days)
	view := digestView{Days: days, Since: cutoff.UTC().Format("2006-01-02")}
	var powerSum float64
	var powerN int
	for _, w := range ws {
		// Skip undated workouts (can't be placed in the window) and dated
		// workouts before the cutoff. Without the !HasStarts guard, dateless
		// rides would be counted in every --days window.
		if !w.HasStarts || w.Starts.Before(cutoff) {
			continue
		}
		view.Workouts++
		if !w.HasSummary {
			view.RidesMissingSummary++
		}
		if w.HasDistance {
			view.TotalDistanceKm += w.DistanceM / 1000
		}
		view.TotalTimeHours += w.DurationHours()
		if w.HasAscent {
			view.TotalAscentM += w.AscentM
		}
		if w.HasWork {
			view.TotalWorkKJ += w.WorkKJ
		}
		view.TotalCalories += w.Calories
		view.EstimatedLoad += estimateRideLoad(w, ftp)
		if w.HasPower && w.AvgPowerW > 0 {
			powerSum += w.AvgPowerW
			powerN++
		}
	}
	if powerN > 0 {
		view.AvgPowerW = round1(powerSum / float64(powerN))
	}
	view.TotalDistanceKm = round2(view.TotalDistanceKm)
	view.TotalTimeHours = round2(view.TotalTimeHours)
	view.TotalAscentM = round1(view.TotalAscentM)
	view.TotalWorkKJ = round1(view.TotalWorkKJ)
	view.TotalCalories = round1(view.TotalCalories)
	view.EstimatedLoad = round1(view.EstimatedLoad)
	return view
}
