// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type digestMetric struct {
	Mean       float64 `json:"mean"`
	PriorMean  float64 `json:"prior_week_mean,omitempty"`
	DeltaWoW   float64 `json:"delta_wow,omitempty"`
	SampleSize int     `json:"sample_size"`
}

type digestDay struct {
	Day   string  `json:"day"`
	Score float64 `json:"score"`
}

type digestTag struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type digestView struct {
	WeekStart    string                  `json:"week_start"`
	WeekEnd      string                  `json:"week_end"`
	Metrics      map[string]digestMetric `json:"metrics"`
	BestDay      *digestDay              `json:"best_recovery_day,omitempty"`
	WorstDay     *digestDay              `json:"worst_recovery_day,omitempty"`
	WorkoutCount int                     `json:"workout_count"`
	WorkoutCals  float64                 `json:"workout_total_calories"`
	TopTags      []digestTag             `json:"most_logged_tags,omitempty"`
	Note         string                  `json:"note,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagEnd string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "One structured call returns the week's metric averages, week-over-week deltas, best and worst recovery days",
		Long: `Summarizes the trailing 7-day window: average sleep, readiness, activity,
and stress, week-over-week deltas against the prior 7 days, the best and
worst readiness day, workout totals, and the most-logged tags.`,
		Example:     `  oura-pp-cli digest --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize the trailing 7-day window with week-over-week deltas")
				return nil
			}
			end := flagEnd
			if end == "" {
				end = today()
			}
			start := addDays(end, -6)
			priorStart := addDays(start, -7)
			priorEnd := addDays(start, -1)

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

			view := digestView{WeekStart: start, WeekEnd: end, Metrics: map[string]digestMetric{}}
			totalSamples := 0
			for _, name := range []string{"sleep", "readiness", "activity", "stress"} {
				spec, _ := resolveMetric(name)
				thisWeek, err := metricSeries(db, spec, start, end)
				if err != nil {
					return fmt.Errorf("querying %s: %w", name, err)
				}
				priorWeek, err := metricSeries(db, spec, priorStart, priorEnd)
				if err != nil {
					return fmt.Errorf("querying prior-week %s: %w", name, err)
				}
				m := digestMetric{SampleSize: len(thisWeek)}
				if len(thisWeek) > 0 {
					vals := make([]float64, 0, len(thisWeek))
					for _, v := range thisWeek {
						vals = append(vals, v)
					}
					mean, _ := meanStdDev(vals)
					m.Mean = round2(mean)
				}
				if len(priorWeek) > 0 {
					vals := make([]float64, 0, len(priorWeek))
					for _, v := range priorWeek {
						vals = append(vals, v)
					}
					priorMean, _ := meanStdDev(vals)
					m.PriorMean = round2(priorMean)
					m.DeltaWoW = round2(m.Mean - m.PriorMean)
				}
				view.Metrics[name] = m
				totalSamples += len(thisWeek)

				if name == "readiness" {
					for d, v := range thisWeek {
						if view.BestDay == nil || v > view.BestDay.Score {
							view.BestDay = &digestDay{Day: d, Score: round2(v)}
						}
						if view.WorstDay == nil || v < view.WorstDay.Score {
							view.WorstDay = &digestDay{Day: d, Score: round2(v)}
						}
					}
				}
			}

			count, totalCal, err := workoutTotals(db, start, end)
			if err != nil {
				return fmt.Errorf("querying workouts: %w", err)
			}
			view.WorkoutCount = count
			view.WorkoutCals = round2(totalCal)

			tags, err := topTags(db, start, end, 3)
			if err != nil {
				return fmt.Errorf("querying tags: %w", err)
			}
			view.TopTags = tags

			if totalSamples == 0 {
				view.Note = "no synced data in this window — run 'oura-pp-cli sync' first"
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Digest %s — %s\n", view.WeekStart, view.WeekEnd)
			for _, name := range []string{"sleep", "readiness", "activity", "stress"} {
				m := view.Metrics[name]
				fmt.Fprintf(out, "  %-10s mean=%.1f (n=%d)", name, m.Mean, m.SampleSize)
				if m.SampleSize > 0 && m.DeltaWoW != 0 {
					fmt.Fprintf(out, "  ΔWoW=%+.1f", m.DeltaWoW)
				}
				fmt.Fprintln(out)
			}
			if view.BestDay != nil {
				fmt.Fprintf(out, "  best readiness day:  %s (%.0f)\n", view.BestDay.Day, view.BestDay.Score)
			}
			if view.WorstDay != nil {
				fmt.Fprintf(out, "  worst readiness day: %s (%.0f)\n", view.WorstDay.Day, view.WorstDay.Score)
			}
			fmt.Fprintf(out, "  workouts: %d (total %.0f cal)\n", view.WorkoutCount, view.WorkoutCals)
			if len(view.TopTags) > 0 {
				fmt.Fprint(out, "  top tags:")
				for _, t := range view.TopTags {
					fmt.Fprintf(out, " %s(%d)", t.Tag, t.Count)
				}
				fmt.Fprintln(out)
			}
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagEnd, "end", "", "Last day of the digest week (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func workoutTotals(db *store.Store, start, end string) (count int, totalCalories float64, err error) {
	row := db.DB().QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(calories), 0) FROM workout WHERE day >= ? AND day <= ?`,
		start, end,
	)
	if err := row.Scan(&count, &totalCalories); err != nil {
		return 0, 0, err
	}
	return count, totalCalories, nil
}

func topTags(db *store.Store, start, end string, limit int) ([]digestTag, error) {
	rows, err := db.DB().Query(
		`SELECT tag_type_code, COUNT(*) FROM enhanced_tag WHERE day >= ? AND day <= ? AND tag_type_code IS NOT NULL GROUP BY tag_type_code`,
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []digestTag
	for rows.Next() {
		var tag sql.NullString
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			continue
		}
		if tag.Valid {
			tags = append(tags, digestTag{Tag: tag.String, Count: count})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
	if len(tags) > limit {
		tags = tags[:limit]
	}
	return tags, nil
}
