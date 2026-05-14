// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type pressureItem struct {
	Name             string  `json:"name"`
	CourseID         string  `json:"course_id"`
	DueDate          string  `json:"due_date"`
	HoursRemaining   float64 `json:"hours_remaining"`
	PointsPossible   float64 `json:"points_possible"`
	PressureScore    float64 `json:"pressure_score"`
	SubmissionStatus string  `json:"submission_status"`
}

func newPressureCmd(flags *rootFlags) *cobra.Command {
	var days int
	var top int
	var course string

	cmd := &cobra.Command{
		Use:         "pressure",
		Short:       "Deadline Pressure Index — rank unsubmitted assignments by points-per-hour urgency",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Ranks all unsubmitted assignments across all enrolled courses by pressure score.

Pressure score = points_possible / max(hours_until_due, 0.5)

Higher scores mean more grade impact per hour of delay. Use this to prioritize
what to work on next when multiple deadlines are approaching.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli pressure --json
  canvas-lms-pp-cli pressure --days 7 --top 5
  canvas-lms-pp-cli pressure --course "CS 3398" --agent --select name,due_date,pressure_score`, "\n"),
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
			cutoff := now.Add(time.Duration(days) * 24 * time.Hour)

			type assignRow struct {
				ID             string
				CoursesID      string
				Name           string
				DueAt          string
				PointsPossible float64
			}

			rows, err := sqlDB.QueryContext(cmd.Context(), `
				SELECT a.id, a.courses_id, a.data
				FROM courses_assignments a
				WHERE json_extract(a.data, '$.due_at') IS NOT NULL
				  AND json_extract(a.data, '$.due_at') != ''
				  AND json_extract(a.data, '$.due_at') != 'null'
			`)
			if err != nil {
				return fmt.Errorf("querying assignments: %w", err)
			}
			defer rows.Close()

			// Build submission lookup: assignment_id -> workflow_state
			subMap := map[string]string{}
			subRows, err := sqlDB.QueryContext(cmd.Context(), `
				SELECT data FROM courses_submissions
			`)
			if err == nil {
				defer subRows.Close()
				for subRows.Next() {
					var raw []byte
					if subRows.Scan(&raw) == nil {
						var sub struct {
							AssignmentID  string `json:"assignment_id"`
							WorkflowState string `json:"workflow_state"`
							Missing       bool   `json:"missing"`
						}
						if json.Unmarshal(raw, &sub) == nil && sub.AssignmentID != "" {
							subMap[sub.AssignmentID] = sub.WorkflowState
						}
					}
				}
			}

			var results []pressureItem
			for rows.Next() {
				var id, coursesID string
				var raw []byte
				if err := rows.Scan(&id, &coursesID, &raw); err != nil {
					continue
				}
				var asgn struct {
					Name           string  `json:"name"`
					DueAt          string  `json:"due_at"`
					PointsPossible float64 `json:"points_possible"`
					CourseID       string  `json:"course_id"`
				}
				if err := json.Unmarshal(raw, &asgn); err != nil {
					continue
				}
				if asgn.DueAt == "" || asgn.DueAt == "null" {
					continue
				}
				dueAt, err := time.Parse(time.RFC3339, asgn.DueAt)
				if err != nil {
					continue
				}
				if dueAt.Before(now) || dueAt.After(cutoff) {
					continue
				}
				// Filter by course fragment
				cid := asgn.CourseID
				if cid == "" {
					cid = coursesID
				}
				if course != "" && !strings.Contains(strings.ToLower(cid), strings.ToLower(course)) {
					continue
				}
				// Skip submitted assignments
				status := subMap[id]
				if status == "graded" || status == "submitted" || status == "pending_review" {
					continue
				}
				if status == "" {
					status = "unsubmitted"
				}
				hoursLeft := dueAt.Sub(now).Hours()
				denom := math.Max(hoursLeft, 0.5)
				score := asgn.PointsPossible / denom

				results = append(results, pressureItem{
					Name:             asgn.Name,
					CourseID:         cid,
					DueDate:          dueAt.Format(time.RFC3339),
					HoursRemaining:   math.Round(hoursLeft*100) / 100,
					PointsPossible:   asgn.PointsPossible,
					PressureScore:    math.Round(score*100) / 100,
					SubmissionStatus: status,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].PressureScore > results[j].PressureScore
			})
			if top > 0 && len(results) > top {
				results = results[:top]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upcoming unsubmitted assignments found.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tNAME\tDUE\tHRS LEFT\tPTS\tPRESSURE")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%.0f\t%.2f\n",
					truncate(r.CourseID, 20), truncate(r.Name, 35),
					r.DueDate[:10], r.HoursRemaining, r.PointsPossible, r.PressureScore)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Look-ahead window in days")
	cmd.Flags().IntVar(&top, "top", 20, "Max results to return")
	cmd.Flags().StringVar(&course, "course", "", "Filter by course ID fragment")

	return cmd
}

// defaultDBPath is defined on rootFlags in helpers.go but we need it accessible
// in novel commands. We call flags.defaultDBPath which delegates to the package-level helper.
func (f *rootFlags) defaultDBPath(name string) string {
	return defaultDBPath(name)
}
