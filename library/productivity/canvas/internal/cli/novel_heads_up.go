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

type headsUpMatch struct {
	CourseID             string  `json:"course_id"`
	AnnouncementTitle    string  `json:"announcement_title"`
	AnnouncementPostedAt string  `json:"announcement_posted_at"`
	AssignmentName       string  `json:"assignment_name"`
	AssignmentDueAt      string  `json:"assignment_due_at"`
	HoursBeforeDue       float64 `json:"hours_before_due"`
}

func newHeadsUpCmd(flags *rootFlags) *cobra.Command {
	var hours int

	cmd := &cobra.Command{
		Use:         "heads-up",
		Short:       "Pre-Deadline Announcement Correlation — find announcements posted near due dates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Surfaces announcements posted within N hours before an assignment due date
for assignments you haven't submitted yet. Instructors often post clarifications,
hints, or rubric updates close to deadlines.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli heads-up --json
  canvas-lms-pp-cli heads-up --hours 48
  canvas-lms-pp-cli heads-up --agent --select course_id,announcement_title,assignment_name`, "\n"),
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

			// Build unsubmitted assignment lookup
			type assignInfo struct {
				Name     string
				DueAt    time.Time
				CourseID string
			}
			assignMap := map[string]assignInfo{}

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
						if json.Unmarshal(raw, &s) == nil && (s.WorkflowState == "submitted" || s.WorkflowState == "graded") {
							submittedIDs[s.AssignmentID] = true
						}
					}
				}
			}

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
				if submittedIDs[id] {
					continue
				}
				var a struct {
					Name     string `json:"name"`
					DueAt    string `json:"due_at"`
					CourseID string `json:"course_id"`
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
				ecid := a.CourseID
				if ecid == "" {
					ecid = cid
				}
				assignMap[id] = assignInfo{Name: a.Name, DueAt: due, CourseID: ecid}
			}

			// Load announcements
			type announcement struct {
				Title    string
				PostedAt time.Time
				CourseID string
			}
			var announcements []announcement

			dRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_discussion_topics`)
			if err == nil {
				defer dRows.Close()
				for dRows.Next() {
					var cid string
					var raw []byte
					if err := dRows.Scan(&cid, &raw); err != nil {
						continue
					}
					var d struct {
						IsAnnouncement bool   `json:"is_announcement"`
						Title          string `json:"title"`
						PostedAt       string `json:"posted_at"`
						CourseID       string `json:"course_id"`
					}
					if err := json.Unmarshal(raw, &d); err != nil {
						continue
					}
					if !d.IsAnnouncement {
						continue
					}
					if d.PostedAt == "" || d.PostedAt == "null" {
						continue
					}
					postedAt, err := time.Parse(time.RFC3339, d.PostedAt)
					if err != nil {
						continue
					}
					ecid := d.CourseID
					if ecid == "" {
						ecid = cid
					}
					announcements = append(announcements, announcement{
						Title:    d.Title,
						PostedAt: postedAt,
						CourseID: ecid,
					})
				}
			}

			windowDur := time.Duration(hours) * time.Hour
			var results []headsUpMatch

			for _, ann := range announcements {
				for _, asgn := range assignMap {
					if ann.CourseID != asgn.CourseID {
						continue
					}
					diff := asgn.DueAt.Sub(ann.PostedAt)
					if diff >= 0 && diff <= windowDur {
						results = append(results, headsUpMatch{
							CourseID:             ann.CourseID,
							AnnouncementTitle:    ann.Title,
							AnnouncementPostedAt: ann.PostedAt.Format(time.RFC3339),
							AssignmentName:       asgn.Name,
							AssignmentDueAt:      asgn.DueAt.Format(time.RFC3339),
							HoursBeforeDue:       math.Round(diff.Hours()*100) / 100,
						})
					}
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No announcements found within %d hours of unsubmitted due dates.\n", hours)
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tANNOUNCEMENT\tASSIGNMENT\tHRS BEFORE DUE")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\n",
					r.CourseID, truncate(r.AnnouncementTitle, 35), truncate(r.AssignmentName, 35), r.HoursBeforeDue)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&hours, "hours", 72, "Window in hours before due date")
	return cmd
}
