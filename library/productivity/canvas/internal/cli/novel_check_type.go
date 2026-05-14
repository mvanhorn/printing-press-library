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

type checkTypeRow struct {
	CourseID       string   `json:"course_id"`
	AssignmentName string   `json:"assignment_name"`
	RequiredTypes  []string `json:"required_types"`
	FamiliarTypes  []string `json:"familiar_types"`
	IsNewType      bool     `json:"is_new_type"`
}

func newCheckTypeCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:         "check-type",
		Short:       "Submission Type Mismatch Alert — warn when upcoming assignments need an unfamiliar format",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compares the submission types required for upcoming assignments against your
historical submission patterns per course. Flags assignments that require a
format you haven't used in that course before (e.g. media_recording for a
course where you've only done online_upload).`,
		Example: strings.Trim(`
  canvas-lms-pp-cli check-type --json
  canvas-lms-pp-cli check-type --days 7
  canvas-lms-pp-cli check-type --agent --select course_id,assignment_name,required_types,is_new_type`, "\n"),
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

			// Build familiar submission types per course from past submissions
			// We join with assignments to get submission_types
			famMap := map[string]map[string]bool{} // course_id -> set of submission types

			// Get assignment submission types map: assignment_id -> []submission_types
			assignTypes := map[string][]string{} // assignment_id -> submission_types
			assignCourse := map[string]string{}  // assignment_id -> course_id

			aAllRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT id, courses_id, data FROM courses_assignments`)
			if err == nil {
				defer aAllRows.Close()
				for aAllRows.Next() {
					var id, cid string
					var raw []byte
					if err := aAllRows.Scan(&id, &cid, &raw); err != nil {
						continue
					}
					var a struct {
						CourseID        string   `json:"course_id"`
						SubmissionTypes []string `json:"submission_types"`
					}
					if err := json.Unmarshal(raw, &a); err != nil {
						continue
					}
					ecid := a.CourseID
					if ecid == "" {
						ecid = cid
					}
					assignTypes[id] = a.SubmissionTypes
					assignCourse[id] = ecid
				}
			}

			// Now find past submissions (graded/submitted) and record their types as familiar
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
							if s.WorkflowState != "graded" && s.WorkflowState != "submitted" {
								continue
							}
							cid := assignCourse[s.AssignmentID]
							if cid == "" {
								continue
							}
							if famMap[cid] == nil {
								famMap[cid] = map[string]bool{}
							}
							for _, t := range assignTypes[s.AssignmentID] {
								famMap[cid][t] = true
							}
						}
					}
				}
			}

			// Now check upcoming assignments
			upRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT id, courses_id, data FROM courses_assignments`)
			if err != nil {
				return fmt.Errorf("querying assignments: %w", err)
			}
			defer upRows.Close()

			var results []checkTypeRow
			for upRows.Next() {
				var id, cid string
				var raw []byte
				if err := upRows.Scan(&id, &cid, &raw); err != nil {
					continue
				}
				var a struct {
					Name            string   `json:"name"`
					DueAt           string   `json:"due_at"`
					CourseID        string   `json:"course_id"`
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
				ecid := a.CourseID
				if ecid == "" {
					ecid = cid
				}

				familiar := famMap[ecid]
				var familiarList []string
				for t := range familiar {
					familiarList = append(familiarList, t)
				}

				isNew := false
				for _, rt := range a.SubmissionTypes {
					if rt == "none" || rt == "not_graded" || rt == "on_paper" {
						continue
					}
					if !familiar[rt] {
						isNew = true
						break
					}
				}

				if isNew || len(familiar) == 0 {
					results = append(results, checkTypeRow{
						CourseID:       ecid,
						AssignmentName: a.Name,
						RequiredTypes:  a.SubmissionTypes,
						FamiliarTypes:  familiarList,
						IsNewType:      isNew,
					})
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No submission type mismatches found for upcoming assignments.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tASSIGNMENT\tREQUIRED TYPES\tFAMILIAR TYPES\tNEW?")
			for _, r := range results {
				newStr := ""
				if r.IsNewType {
					newStr = "YES"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					truncate(r.CourseID, 20),
					truncate(r.AssignmentName, 35),
					strings.Join(r.RequiredTypes, ","),
					strings.Join(r.FamiliarTypes, ","),
					newStr)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Look-ahead window in days")
	return cmd
}
