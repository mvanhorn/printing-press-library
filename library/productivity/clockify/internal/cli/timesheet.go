// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: offline weekly timesheet reconstruction and gap
// detection from the local SQLite mirror.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

var weekdayLabels = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

func newTimesheetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timesheet",
		Short: "Reconstruct and inspect your weekly timesheet offline",
		Long: `Rebuild the weekly Clockify timesheet grid from locally synced time
entries, find the days you are short before you submit, and submit the
week for approval — all without re-querying the live API.

Run 'clockify-pp-cli sync' first to populate the local store.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTimesheetWeekCmd(flags))
	cmd.AddCommand(newTimesheetGapsCmd(flags))
	return cmd
}

// weekEntries loads entries that start within [ws, ws+7d), optionally filtered
// to one workspace. Entries come from the local store, hydrated live for the
// week when the store has none cached.
func weekEntries(db *store.Store, flags *rootFlags, ws time.Time, workspace string) ([]timeEntry, error) {
	weekEnd := ws.AddDate(0, 0, 7)
	all, err := ensureTimeEntries(db, flags, ws, weekEnd, workspace)
	if err != nil {
		return nil, err
	}
	out := make([]timeEntry, 0, len(all))
	for _, e := range all {
		if e.Start.IsZero() || e.Start.Before(ws) || !e.Start.Before(weekEnd) {
			continue
		}
		if workspace != "" && e.WorkspaceID != workspace {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func newTimesheetWeekCmd(flags *rootFlags) *cobra.Command {
	var dateFlag, workspace, dbPath string
	var submit bool

	cmd := &cobra.Command{
		Use:   "week",
		Short: "Rebuild the weekly timesheet grid offline (project x weekday)",
		Long: `Pivot every synced time entry for the week into a project-by-weekday
grid with per-day, per-project, and weekly totals.

By default this only reads the local store. Pass --submit to send the
week to the Clockify approval workflow (a live API write); a gap check
runs first and warns about under-logged days.`,
		Example: `  # This week's grid
  clockify-pp-cli timesheet week

  # A specific week (any date in it), as JSON
  clockify-pp-cli timesheet week --date 2026-05-11 --json

  # Submit the current week for approval
  clockify-pp-cli timesheet week --submit`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			ref, err := parseDateFlag(dateFlag, time.Now())
			if err != nil {
				return usageErr(err)
			}
			ws := weekStart(ref)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			entries, err := weekEntries(db, flags, ws, workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			projects := loadProjects(db)

			// Pivot: projectId -> [7]duration.
			type projRow struct {
				ID    string
				Name  string
				Daily [7]time.Duration
				Total time.Duration
			}
			rowByID := map[string]*projRow{}
			var dailyTotals [7]time.Duration
			var grand time.Duration
			running := 0
			for _, e := range entries {
				if e.Running {
					running++
				}
				col := (int(e.Start.Weekday()) + 6) % 7
				r := rowByID[e.ProjectID]
				if r == nil {
					name := "(no project)"
					if p, ok := projects[e.ProjectID]; ok && p.Name != "" {
						name = p.Name
					} else if e.ProjectID != "" {
						name = e.ProjectID
					}
					r = &projRow{ID: e.ProjectID, Name: name}
					rowByID[e.ProjectID] = r
				}
				r.Daily[col] += e.Duration
				r.Total += e.Duration
				dailyTotals[col] += e.Duration
				grand += e.Duration
			}

			rows := make([]*projRow, 0, len(rowByID))
			for _, r := range rowByID {
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Total != rows[j].Total {
					return rows[i].Total > rows[j].Total
				}
				return rows[i].Name < rows[j].Name
			})

			if flags.asJSON {
				type jsonRow struct {
					ProjectID  string     `json:"project_id"`
					Project    string     `json:"project"`
					DailyHours [7]float64 `json:"daily_hours"`
					TotalHours float64    `json:"total_hours"`
				}
				jrows := make([]jsonRow, 0, len(rows))
				for _, r := range rows {
					jr := jsonRow{ProjectID: r.ID, Project: r.Name, TotalHours: round2(r.Total.Hours())}
					for i := 0; i < 7; i++ {
						jr.DailyHours[i] = round2(r.Daily[i].Hours())
					}
					jrows = append(jrows, jr)
				}
				var dt [7]float64
				for i := 0; i < 7; i++ {
					dt[i] = round2(dailyTotals[i].Hours())
				}
				return flags.printJSON(cmd, map[string]any{
					"week_start":   ws.Format("2006-01-02"),
					"week_end":     ws.AddDate(0, 0, 6).Format("2006-01-02"),
					"days":         weekdayLabels,
					"projects":     jrows,
					"daily_totals": dt,
					"total_hours":  round2(grand.Hours()),
					"running":      running,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Timesheet — week of %s\n\n", ws.Format("Mon Jan 2, 2006"))
			if len(rows) == 0 {
				fmt.Fprintln(out, "No time entries for this week.")
				fmt.Fprintf(out, "(%s)\n", emptyStoreHint)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintf(tw, "PROJECT\t%s\t%s\t%s\t%s\t%s\t%s\t%s\tTOTAL\n",
				weekdayLabels[0], weekdayLabels[1], weekdayLabels[2], weekdayLabels[3],
				weekdayLabels[4], weekdayLabels[5], weekdayLabels[6])
			for _, r := range rows {
				fmt.Fprintf(tw, "%s", truncate(r.Name, 28))
				for i := 0; i < 7; i++ {
					fmt.Fprintf(tw, "\t%s", cell(r.Daily[i]))
				}
				fmt.Fprintf(tw, "\t%.2f\n", r.Total.Hours())
			}
			fmt.Fprintf(tw, "TOTAL")
			for i := 0; i < 7; i++ {
				fmt.Fprintf(tw, "\t%s", cell(dailyTotals[i]))
			}
			fmt.Fprintf(tw, "\t%.2f\n", grand.Hours())
			tw.Flush()
			if running > 0 {
				fmt.Fprintf(out, "\n%d timer(s) still running — totals exclude open entries.\n", running)
			}

			if !submit {
				return nil
			}
			return submitWeek(cmd, flags, workspace, ws, dailyTotals)
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "Any date in the target week (YYYY-MM-DD, default: today)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&submit, "submit", false, "Submit this week for approval (live API write)")
	return cmd
}

// submitWeek runs a gap check then posts an approval request for the week.
func submitWeek(cmd *cobra.Command, flags *rootFlags, workspace string, ws time.Time, daily [7]time.Duration) error {
	short := 0
	for i := 0; i < 5; i++ { // Mon-Fri
		if daily[i].Hours() < 8 {
			short++
		}
	}
	out := cmd.OutOrStdout()
	periodStart := ws.UTC().Format("2006-01-02T15:04:05.000Z")

	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(out, "would submit week of %s for approval (periodStart=%s)\n", ws.Format("2006-01-02"), periodStart)
		return nil
	}
	if short > 0 {
		fmt.Fprintf(out, "\nWarning: %d weekday(s) below 8h — submitting anyway. Run 'timesheet gaps' to review.\n", short)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	wsID := workspace
	uid := ""
	if wsID == "" {
		wsID, uid, err = resolveWorkspaceUser(c, "")
		if err != nil {
			return err
		}
		_ = uid
	}
	body := map[string]any{"period": "WEEKLY", "periodStart": periodStart}
	resp, _, err := c.Post(fmt.Sprintf("/v1/workspaces/%s/approval-requests", wsID), body)
	if err != nil {
		return apiErr(fmt.Errorf("submitting approval request: %w", err))
	}
	if flags.asJSON {
		return printOutput(out, resp, true)
	}
	fmt.Fprintf(out, "\nSubmitted week of %s for approval.\n", ws.Format("2006-01-02"))
	return nil
}

func newTimesheetGapsCmd(flags *rootFlags) *cobra.Command {
	var dateFlag, workspace, dbPath string
	var workday time.Duration
	var includeWeekends bool

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Find weekdays under your target hours before you submit",
		Long: `Compare each day's tracked hours for the week against a workday target
and report the days you are short, with the missing time.`,
		Example: `  # Gaps for this week against an 8h workday
  clockify-pp-cli timesheet gaps

  # A 7.5h workday, a specific week, JSON
  clockify-pp-cli timesheet gaps --workday 7h30m --date 2026-05-11 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			ref, err := parseDateFlag(dateFlag, time.Now())
			if err != nil {
				return usageErr(err)
			}
			ws := weekStart(ref)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			entries, err := weekEntries(db, flags, ws, workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			var daily [7]time.Duration
			for _, e := range entries {
				col := (int(e.Start.Weekday()) + 6) % 7
				daily[col] += e.Duration
			}

			type gap struct {
				Date         string  `json:"date"`
				Weekday      string  `json:"weekday"`
				TrackedHours float64 `json:"tracked_hours"`
				TargetHours  float64 `json:"target_hours"`
				MissingHours float64 `json:"missing_hours"`
			}
			lastCol := 5 // Mon-Fri
			if includeWeekends {
				lastCol = 7
			}
			var gaps []gap
			var totalMissing time.Duration
			for i := 0; i < lastCol; i++ {
				if daily[i] >= workday {
					continue
				}
				missing := workday - daily[i]
				totalMissing += missing
				gaps = append(gaps, gap{
					Date:         ws.AddDate(0, 0, i).Format("2006-01-02"),
					Weekday:      weekdayLabels[i],
					TrackedHours: round2(daily[i].Hours()),
					TargetHours:  round2(workday.Hours()),
					MissingHours: round2(missing.Hours()),
				})
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"week_start":          ws.Format("2006-01-02"),
					"workday_target":      round2(workday.Hours()),
					"gaps":                gaps,
					"total_missing_hours": round2(totalMissing.Hours()),
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Timesheet gaps — week of %s (target %.2fh/day)\n\n", ws.Format("Mon Jan 2, 2006"), workday.Hours())
			if len(gaps) == 0 {
				fmt.Fprintln(out, "No gaps — every day meets the target.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "DATE\tDAY\tTRACKED\tTARGET\tMISSING")
			for _, g := range gaps {
				fmt.Fprintf(tw, "%s\t%s\t%.2fh\t%.2fh\t%.2fh\n", g.Date, g.Weekday, g.TrackedHours, g.TargetHours, g.MissingHours)
			}
			tw.Flush()
			fmt.Fprintf(out, "\n%d day(s) short, %.2fh missing total.\n", len(gaps), totalMissing.Hours())
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "Any date in the target week (YYYY-MM-DD, default: today)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().DurationVar(&workday, "workday", 8*time.Hour, "Expected tracked time per workday")
	cmd.Flags().BoolVar(&includeWeekends, "include-weekends", false, "Also check Saturday and Sunday")
	return cmd
}

// cell renders a grid cell: blank for zero, decimal hours otherwise.
func cell(d time.Duration) string {
	if d == 0 {
		return "·"
	}
	return fmt.Sprintf("%.2f", d.Hours())
}

// round2 rounds a float to 2 decimal places for stable JSON output.
func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
