// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type lateWindowRow struct {
	CourseID       string  `json:"course_id"`
	LateAccepted   int     `json:"late_accepted"`
	LateRejected   int     `json:"late_rejected"`
	AvgDaysLate    float64 `json:"avg_days_late"`
	MaxDaysLate    float64 `json:"max_days_late"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

func newLateWindowCmd(flags *rootFlags) *cobra.Command {
	var course string
	var minSamples int

	cmd := &cobra.Command{
		Use:         "late-window",
		Short:       "Late Submission Window Detector — how forgiving are your instructors?",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Analyzes past late submissions to estimate how many days late an instructor
typically accepts work and still gives credit.

Acceptance is defined as: late=true AND score > 0.
Rejection is defined as: late=true AND (score=0 OR score is null).`,
		Example: strings.Trim(`
  canvas-lms-pp-cli late-window --json
  canvas-lms-pp-cli late-window --course 12345
  canvas-lms-pp-cli late-window --min-samples 2 --agent --select course_id,acceptance_rate,avg_days_late`, "\n"),
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

			// Build assignment due_at lookup
			dueMap := map[string]time.Time{}
			aRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT id, data FROM courses_assignments`)
			if err == nil {
				defer aRows.Close()
				for aRows.Next() {
					var id string
					var raw []byte
					if err := aRows.Scan(&id, &raw); err != nil {
						continue
					}
					var a struct {
						DueAt string `json:"due_at"`
					}
					if err := json.Unmarshal(raw, &a); err != nil {
						continue
					}
					if a.DueAt != "" && a.DueAt != "null" {
						if t, err := time.Parse(time.RFC3339, a.DueAt); err == nil {
							dueMap[id] = t
						}
					}
				}
			}

			type courseStat struct {
				accepted  int
				rejected  int
				totalDays float64
				maxDays   float64
			}
			stats := map[string]*courseStat{}

			sRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_submissions`)
			if err != nil {
				return fmt.Errorf("querying submissions: %w", err)
			}
			defer sRows.Close()

			for sRows.Next() {
				var cid string
				var raw []byte
				if err := sRows.Scan(&cid, &raw); err != nil {
					continue
				}
				var s struct {
					AssignmentID string   `json:"assignment_id"`
					CourseID     string   `json:"course_id"`
					SubmittedAt  string   `json:"submitted_at"`
					Late         bool     `json:"late"`
					Score        *float64 `json:"score"`
				}
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}
				if !s.Late {
					continue
				}
				ecid := s.CourseID
				if ecid == "" {
					ecid = cid
				}
				if course != "" && !strings.Contains(ecid, course) {
					continue
				}

				if stats[ecid] == nil {
					stats[ecid] = &courseStat{}
				}
				st := stats[ecid]

				var daysLate float64
				if due, ok := dueMap[s.AssignmentID]; ok && s.SubmittedAt != "" {
					if subTime, err := time.Parse(time.RFC3339, s.SubmittedAt); err == nil {
						daysLate = subTime.Sub(due).Hours() / 24
						if daysLate < 0 {
							daysLate = 0
						}
					}
				}

				if s.Score != nil && *s.Score > 0 {
					st.accepted++
					st.totalDays += daysLate
					if daysLate > st.maxDays {
						st.maxDays = daysLate
					}
				} else {
					st.rejected++
				}
			}

			var results []lateWindowRow
			for cid, st := range stats {
				total := st.accepted + st.rejected
				if total < minSamples {
					continue
				}
				avg := 0.0
				if st.accepted > 0 {
					avg = st.totalDays / float64(st.accepted)
				}
				rate := 0.0
				if total > 0 {
					rate = float64(st.accepted) / float64(total) * 100
				}
				results = append(results, lateWindowRow{
					CourseID:       cid,
					LateAccepted:   st.accepted,
					LateRejected:   st.rejected,
					AvgDaysLate:    math.Round(avg*100) / 100,
					MaxDaysLate:    math.Round(st.maxDays*100) / 100,
					AcceptanceRate: math.Round(rate*100) / 100,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No late submission history found (need at least --min-samples records).")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tACCEPTED\tREJECTED\tAVG DAYS LATE\tMAX DAYS LATE\tACCEPT RATE")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f\t%.1f\t%.0f%%\n",
					r.CourseID, r.LateAccepted, r.LateRejected,
					r.AvgDaysLate, r.MaxDaysLate, r.AcceptanceRate)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&course, "course", "", "Filter by course ID")
	cmd.Flags().IntVar(&minSamples, "min-samples", 3, "Minimum late submissions to include a course")
	return cmd
}
