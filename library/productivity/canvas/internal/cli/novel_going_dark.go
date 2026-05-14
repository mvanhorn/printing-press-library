// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type goingDarkRow struct {
	CourseID                string `json:"course_id"`
	AssignmentsDue          int    `json:"assignments_due"`
	UnreadDiscussions       int    `json:"unread_discussions"`
	DaysSinceModuleActivity int    `json:"days_since_module_activity"`
}

func newGoingDarkCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:         "going-dark",
		Short:       "Silent Drop Detector — flag courses slipping off your radar",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Flags courses that have upcoming assignment deadlines, unread discussions,
and no recent module completions — courses that may be going off your radar.

A course is flagged when it has:
  - At least one assignment due within --days
  - At least one unread discussion topic
  - OR no module completions recorded`,
		Example: strings.Trim(`
  canvas-lms-pp-cli going-dark --json
  canvas-lms-pp-cli going-dark --days 14
  canvas-lms-pp-cli going-dark --agent --select course_id,assignments_due,unread_discussions`, "\n"),
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

			// Count upcoming assignments per course
			upcomingMap := map[string]int{}
			aRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_assignments`)
			if err != nil {
				return fmt.Errorf("querying assignments: %w", err)
			}
			defer aRows.Close()
			for aRows.Next() {
				var cid string
				var raw []byte
				if err := aRows.Scan(&cid, &raw); err != nil {
					continue
				}
				var a struct {
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
				if due.After(now) && due.Before(cutoff) {
					ecid := a.CourseID
					if ecid == "" {
						ecid = cid
					}
					upcomingMap[ecid]++
				}
			}

			// Count unread discussion topics per course
			unreadMap := map[string]int{}
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
						UnreadCount int    `json:"unread_count"`
						CourseID    string `json:"course_id"`
					}
					if err := json.Unmarshal(raw, &d); err != nil {
						continue
					}
					if d.UnreadCount > 0 {
						ecid := d.CourseID
						if ecid == "" {
							ecid = cid
						}
						unreadMap[ecid]++
					}
				}
			}

			// Get days since last module completion per course
			modActivityMap := map[string]int{}
			mRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM modules`)
			if err == nil {
				defer mRows.Close()
				for mRows.Next() {
					var cid string
					var raw []byte
					if err := mRows.Scan(&cid, &raw); err != nil {
						continue
					}
					var m struct {
						State       string `json:"state"`
						CompletedAt string `json:"completed_at"`
						CourseID    string `json:"course_id"`
					}
					if err := json.Unmarshal(raw, &m); err != nil {
						continue
					}
					ecid := m.CourseID
					if ecid == "" {
						ecid = cid
					}
					if m.CompletedAt != "" && m.CompletedAt != "null" {
						t, err := time.Parse(time.RFC3339, m.CompletedAt)
						if err == nil {
							daysSince := int(now.Sub(t).Hours() / 24)
							// Track minimum (most recent)
							if existing, ok := modActivityMap[ecid]; !ok || daysSince < existing {
								modActivityMap[ecid] = daysSince
							}
						}
					} else if _, ok := modActivityMap[ecid]; !ok {
						modActivityMap[ecid] = 9999
					}
				}
			}

			var results []goingDarkRow
			for cid, count := range upcomingMap {
				if count == 0 {
					continue
				}
				unread := unreadMap[cid]
				daysSince := modActivityMap[cid]
				if daysSince == 0 {
					daysSince = 9999
				}
				// Flag if: unread discussions exist OR module activity is stale (>7 days or never)
				if unread > 0 || daysSince > 7 {
					results = append(results, goingDarkRow{
						CourseID:                cid,
						AssignmentsDue:          count,
						UnreadDiscussions:       unread,
						DaysSinceModuleActivity: daysSince,
					})
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No courses flagged as going dark.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tDUE SOON\tUNREAD DISC\tMODULE ACTIVITY (DAYS)")
			for _, r := range results {
				daysStr := fmt.Sprintf("%d", r.DaysSinceModuleActivity)
				if r.DaysSinceModuleActivity >= 9999 {
					daysStr = "never"
				}
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n",
					r.CourseID, r.AssignmentsDue, r.UnreadDiscussions, daysStr)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Look-ahead window in days")
	return cmd
}
