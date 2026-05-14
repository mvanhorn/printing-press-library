// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type impactRow struct {
	CourseID          string  `json:"course_id"`
	CurrentScore      float64 `json:"current_score"`
	MaxAchievable     float64 `json:"max_achievable"`
	MinAchievable     float64 `json:"min_achievable"`
	UnsubmittedCount  int     `json:"unsubmitted_count"`
	UnsubmittedPoints float64 `json:"unsubmitted_points"`
	TotalPoints       float64 `json:"total_points"`
}

func newImpactCmd(flags *rootFlags) *cobra.Command {
	var course string

	cmd := &cobra.Command{
		Use:         "impact",
		Short:       "Grade Impact Calculator — show max/min achievable final score per course",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `For each enrolled course, computes the best and worst final score still achievable
given your current grade and all remaining unsubmitted assignments.

Max achievable: assume 100% on all remaining work.
Min achievable: assume 0% on all remaining work.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli impact --json
  canvas-lms-pp-cli impact --course 12345
  canvas-lms-pp-cli impact --agent --select course_id,current_score,max_achievable`, "\n"),
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

			// Fetch enrollments to get current scores
			type enrollment struct {
				CourseID     string
				CurrentScore float64
				FinalScore   float64
			}
			enrollments := map[string]enrollment{}

			eRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_enrollments`)
			if err != nil {
				return fmt.Errorf("querying enrollments: %w", err)
			}
			defer eRows.Close()
			for eRows.Next() {
				var cid string
				var raw []byte
				if err := eRows.Scan(&cid, &raw); err != nil {
					continue
				}
				var e struct {
					Type   string `json:"type"`
					Grades struct {
						CurrentScore *float64 `json:"current_score"`
						FinalScore   *float64 `json:"final_score"`
					} `json:"grades"`
					CourseID string `json:"course_id"`
				}
				if err := json.Unmarshal(raw, &e); err != nil {
					continue
				}
				if e.Type != "StudentEnrollment" {
					continue
				}
				ecid := e.CourseID
				if ecid == "" {
					ecid = cid
				}
				if course != "" && !strings.Contains(ecid, course) {
					continue
				}
				var cs, fs float64
				if e.Grades.CurrentScore != nil {
					cs = *e.Grades.CurrentScore
				}
				if e.Grades.FinalScore != nil {
					fs = *e.Grades.FinalScore
				}
				enrollments[ecid] = enrollment{CourseID: ecid, CurrentScore: cs, FinalScore: fs}
			}

			if len(enrollments) == 0 {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), []impactRow{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No student enrollments found. Run 'canvas-lms-pp-cli sync' first.")
				return nil
			}

			// Fetch all assignments, group by course
			type assignInfo struct {
				ID             string
				PointsPossible float64
			}
			courseAssignments := map[string][]assignInfo{}
			totalPointsMap := map[string]float64{}

			aRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT id, courses_id, data FROM courses_assignments`)
			if err != nil {
				return fmt.Errorf("querying assignments: %w", err)
			}
			defer aRows.Close()
			for aRows.Next() {
				var id, cid string
				var raw []byte
				if err := aRows.Scan(&id, &cid, &raw); err != nil {
					continue
				}
				var a struct {
					CourseID       string  `json:"course_id"`
					PointsPossible float64 `json:"points_possible"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					continue
				}
				ecid := a.CourseID
				if ecid == "" {
					ecid = cid
				}
				totalPointsMap[ecid] += a.PointsPossible
				courseAssignments[ecid] = append(courseAssignments[ecid], assignInfo{ID: id, PointsPossible: a.PointsPossible})
			}

			// Build submission lookup
			submittedIDs := map[string]bool{}
			sRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT data FROM courses_submissions`)
			if err == nil {
				defer sRows.Close()
				for sRows.Next() {
					var raw []byte
					if sRows.Scan(&raw) == nil {
						var s struct {
							AssignmentID  string `json:"assignment_id"`
							WorkflowState string `json:"workflow_state"`
						}
						if json.Unmarshal(raw, &s) == nil {
							if s.WorkflowState == "submitted" || s.WorkflowState == "graded" || s.WorkflowState == "pending_review" {
								submittedIDs[s.AssignmentID] = true
							}
						}
					}
				}
			}

			var results []impactRow
			for cid, enr := range enrollments {
				var unsubPts float64
				var unsubCount int
				for _, a := range courseAssignments[cid] {
					if !submittedIDs[a.ID] {
						unsubPts += a.PointsPossible
						unsubCount++
					}
				}
				total := totalPointsMap[cid]
				if total == 0 {
					total = 1 // avoid division by zero
				}
				currentEarned := enr.CurrentScore / 100.0 * (total - unsubPts)
				maxScore := 0.0
				minScore := 0.0
				if total > 0 {
					maxScore = (currentEarned + unsubPts) / total * 100
					minScore = currentEarned / total * 100
					if maxScore > 100 {
						maxScore = 100
					}
					if minScore < 0 {
						minScore = 0
					}
				}
				results = append(results, impactRow{
					CourseID:          cid,
					CurrentScore:      enr.CurrentScore,
					MaxAchievable:     roundTo2(maxScore),
					MinAchievable:     roundTo2(minScore),
					UnsubmittedCount:  unsubCount,
					UnsubmittedPoints: unsubPts,
					TotalPoints:       total,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tCURRENT\tMAX\tMIN\tUNSUBMITTED\tUNSUB PTS")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%.1f%%\t%.1f%%\t%.1f%%\t%d\t%.0f\n",
					r.CourseID, r.CurrentScore, r.MaxAchievable, r.MinAchievable,
					r.UnsubmittedCount, r.UnsubmittedPoints)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&course, "course", "", "Filter by course ID")
	return cmd
}

func roundTo2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
