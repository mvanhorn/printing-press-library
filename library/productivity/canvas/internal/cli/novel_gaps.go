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

type gapsRow struct {
	CourseID            string  `json:"course_id"`
	ModuleName          string  `json:"module_name"`
	CompletionPct       float64 `json:"completion_pct"`
	State               string  `json:"state"`
	ItemsCount          int     `json:"items_count"`
	HasUpcomingDeadline bool    `json:"has_upcoming_deadline"`
}

func newGapsCmd(flags *rootFlags) *cobra.Command {
	var course string

	cmd := &cobra.Command{
		Use:         "gaps",
		Short:       "Module Completion Gap Report — flag incomplete modules with upcoming deadlines",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Shows module completion status per course and flags modules that are not completed
but have upcoming assignment deadlines. Helps you avoid being blocked by prerequisite
modules at the last minute.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli gaps --json
  canvas-lms-pp-cli gaps --course 12345
  canvas-lms-pp-cli gaps --agent --select course_id,module_name,completion_pct,state`, "\n"),
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
			upcoming := now.Add(14 * 24 * time.Hour)

			// Collect courses with upcoming deadlines
			coursesWithDeadlines := map[string]bool{}
			aRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_assignments`)
			if err == nil {
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
					if due.After(now) && due.Before(upcoming) {
						ecid := a.CourseID
						if ecid == "" {
							ecid = cid
						}
						coursesWithDeadlines[ecid] = true
					}
				}
			}

			// Load modules
			mRows, err := sqlDB.QueryContext(cmd.Context(), `SELECT courses_id, data FROM modules`)
			if err != nil {
				return fmt.Errorf("querying modules: %w", err)
			}
			defer mRows.Close()

			var results []gapsRow
			for mRows.Next() {
				var cid string
				var raw []byte
				if err := mRows.Scan(&cid, &raw); err != nil {
					continue
				}
				var m struct {
					Name       string `json:"name"`
					State      string `json:"state"`
					ItemsCount int    `json:"items_count"`
					CourseID   string `json:"course_id"`
				}
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				ecid := m.CourseID
				if ecid == "" {
					ecid = cid
				}
				if course != "" && !strings.Contains(ecid, course) {
					continue
				}

				completionPct := 0.0
				switch m.State {
				case "completed":
					completionPct = 100.0
				case "started":
					completionPct = 50.0
				case "unlocked":
					completionPct = 10.0
				case "locked":
					completionPct = 0.0
				}

				results = append(results, gapsRow{
					CourseID:            ecid,
					ModuleName:          m.Name,
					CompletionPct:       completionPct,
					State:               m.State,
					ItemsCount:          m.ItemsCount,
					HasUpcomingDeadline: coursesWithDeadlines[ecid],
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No module data found. Run 'canvas-lms-pp-cli sync' first.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tMODULE\tSTATE\tITEMS\tCOMPLETION\tUPCOMING DEADLINE")
			for _, r := range results {
				deadline := ""
				if r.HasUpcomingDeadline {
					deadline = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%.0f%%\t%s\n",
					truncate(r.CourseID, 20), truncate(r.ModuleName, 35),
					r.State, r.ItemsCount, r.CompletionPct, deadline)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&course, "course", "", "Filter by course ID")
	return cmd
}
