// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: personal bests. Not generated.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type bestRecord struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	WorkoutID string  `json:"workout_id"`
	Name      string  `json:"name"`
	Date      string  `json:"date"`
}

// bestsMetrics defines the summary metrics records are computed over, plus how
// each is displayed. Values come from per-ride summaries (averages/totals),
// not per-second streams — so "power" is a best AVERAGE-power ride, not a
// mean-maximal power curve.
var bestsMetrics = []struct {
	key   string
	unit  string
	scale float64 // multiply stored value for display (e.g. meters→km)
}{
	{"distance", "km", 0.001},
	{"ascent", "m", 1},
	{"power", "W", 1},
	{"duration", "h", 1},
	{"work", "kJ", 1},
}

func newNovelBestsCmd(flags *rootFlags) *cobra.Command {
	var metric string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bests",
		Short: "All-time records from your ride summaries",
		Long: "Find your record rides for distance, ascent, average power, duration, and work\n" +
			"from the local workout-summary mirror. These use per-ride summary values, not\n" +
			"per-second streams, so 'power' is your best AVERAGE-power ride — not a\n" +
			"mean-maximal power curve (the Wahoo API exposes no power streams).\n" +
			"For training-load trends use 'load'; for a recent window use 'digest'.",
		Example:     "  wahoo-pp-cli bests --metric power --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			metric = strings.ToLower(strings.TrimSpace(metric))
			if metric != "" && !knownBestsMetric(metric) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown --metric %q (valid: distance, ascent, power, duration, work)", metric))
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "workouts", `[]`) {
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
			records := computeBests(ws, metric)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(records) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No ride summaries with the requested metric in the local mirror.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "METRIC\tVALUE\tDATE\tRIDE")
				for _, r := range records {
					fmt.Fprintf(tw, "%s\t%.1f %s\t%s\t%s\n", r.Metric, r.Value, r.Unit, r.Date, r.Name)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), records, flags)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "", "Limit to one metric: distance, ascent, power, duration, work (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

func knownBestsMetric(m string) bool {
	for _, bm := range bestsMetrics {
		if bm.key == m {
			return true
		}
	}
	return false
}

// computeBests returns the record workout for each requested metric. A metric
// is skipped entirely when no ride carries it (rather than reporting a phantom
// zero record).
func computeBests(ws []parsedWorkout, only string) []bestRecord {
	out := make([]bestRecord, 0, len(bestsMetrics))
	for _, bm := range bestsMetrics {
		if only != "" && bm.key != only {
			continue
		}
		var best parsedWorkout
		var bestVal float64
		found := false
		for _, w := range ws {
			v, ok := w.metricValue(bm.key)
			if !ok || v <= 0 {
				continue
			}
			if !found || v > bestVal {
				best, bestVal, found = w, v, true
			}
		}
		if !found {
			continue
		}
		rec := bestRecord{
			Metric:    bm.key,
			Value:     round2(bestVal * bm.scale),
			Unit:      bm.unit,
			WorkoutID: best.ID,
			Name:      best.Name,
		}
		if best.HasStarts {
			rec.Date = best.Starts.UTC().Format("2006-01-02")
		}
		out = append(out, rec)
	}
	// Stable order matching bestsMetrics declaration for deterministic output.
	sort.SliceStable(out, func(i, j int) bool {
		return bestsMetricIndex(out[i].Metric) < bestsMetricIndex(out[j].Metric)
	})
	return out
}

func bestsMetricIndex(m string) int {
	for i, bm := range bestsMetrics {
		if bm.key == m {
			return i
		}
	}
	return len(bestsMetrics)
}
