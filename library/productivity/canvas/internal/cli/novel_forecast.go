// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type forecastDay struct {
	Date            string   `json:"date"`
	LoadScore       float64  `json:"load_score"`
	AssignmentCount int      `json:"assignment_count"`
	Assignments     []string `json:"assignments"`
}

func newForecastCmd(flags *rootFlags) *cobra.Command {
	var weeks int

	effortWeights := map[string]float64{
		"online_upload":     3,
		"online_text_entry": 1,
		"discussion_topic":  2,
		"media_recording":   2,
	}

	cmd := &cobra.Command{
		Use:         "forecast",
		Short:       "Cross-Course Workload Forecast — per-day assignment load for the next N weeks",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Shows a per-day breakdown of assignment workload for the next N weeks,
weighted by submission type effort:
  online_upload=3, discussion_topic=2, media_recording=2, online_text_entry=1, other=1`,
		Example: strings.Trim(`
  canvas-lms-pp-cli forecast --json
  canvas-lms-pp-cli forecast --weeks 4
  canvas-lms-pp-cli forecast --agent --select date,load_score,assignment_count`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), flags.defaultDBPath("canvas-lms-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sqlDB := db.DB()
			now := time.Now()
			cutoff := now.Add(time.Duration(weeks) * 7 * 24 * time.Hour)

			type dayData struct {
				load  float64
				names []string
			}
			byDay := map[string]*dayData{}

			rows, err := sqlDB.QueryContext(cmd.Context(), `SELECT data FROM courses_assignments`)
			if err != nil {
				return fmt.Errorf("querying assignments: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var a struct {
					Name            string   `json:"name"`
					DueAt           string   `json:"due_at"`
					SubmissionTypes []string `json:"submission_types"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					continue
				}
				if a.DueAt == "" || a.DueAt == "null" {
					continue
				}
				due, err := time.Parse(time.RFC3339, a.DueAt)
				if err != nil {
					continue
				}
				if due.Before(now) || due.After(cutoff) {
					continue
				}

				// Compute effort
				weight := 1.0
				for _, st := range a.SubmissionTypes {
					if w, ok := effortWeights[st]; ok && w > weight {
						weight = w
					}
				}

				day := due.Format("2006-01-02")
				if byDay[day] == nil {
					byDay[day] = &dayData{}
				}
				byDay[day].load += weight
				byDay[day].names = append(byDay[day].names, a.Name)
			}

			// Build sorted results
			var results []forecastDay
			for day, data := range byDay {
				results = append(results, forecastDay{
					Date:            day,
					LoadScore:       data.load,
					AssignmentCount: len(data.names),
					Assignments:     data.names,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i].Date < results[j].Date
			})

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upcoming assignments found.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DATE\tLOAD\tCOUNT\tASSIGNMENTS")
			for _, r := range results {
				names := truncate(strings.Join(r.Assignments, "; "), 60)
				fmt.Fprintf(tw, "%s\t%.0f\t%d\t%s\n", r.Date, r.LoadScore, r.AssignmentCount, names)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&weeks, "weeks", 2, "Number of weeks to forecast")
	return cmd
}
