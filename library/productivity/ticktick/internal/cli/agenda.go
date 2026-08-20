// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Agenda: one-command daily briefing joined from the local SQLite mirror.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/store"
)

// pp:data-source local
func newNovelAgendaCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "Today's tasks, habits, and focus sessions in one bounded response",
		Long: "Use this command for a single day's snapshot. Do NOT use it to gather multi-day review data; use 'review' instead.\n" +
			"Reads the local SQLite mirror (run 'sync' first) and joins tasks due or starting on the date, all active habits, and the day's focus records.",
		Example:     "  ticktick-pp-cli agenda --json --select tasks,focus_minutes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for today's tasks, habits, and focus records")
				return nil
			}
			date, err := resolveNoteDate(flagDate)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("ticktick-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ticktick-pp-cli sync --resources tasks,habits,focus --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					// Keep the documented object shape on an empty/unsynced
					// cache so JSON consumers keyed on object fields don't
					// see the type flip to a bare array.
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"date":          date,
						"tasks":         []any{},
						"habits":        []any{},
						"focus":         []any{},
						"focus_minutes": 0,
					}, flags)
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "tasks") {
				hintIfStale(cmd, db, "tasks", 24*time.Hour)
			}

			tasks, err := queryResourceMaps(cmd, db, "tasks")
			if err != nil {
				return err
			}
			dayTasks := make([]map[string]any, 0)
			for _, t := range tasks {
				due, _ := t["dueDate"].(string)
				start, _ := t["startDate"].(string)
				if strings.HasPrefix(due, date) || strings.HasPrefix(start, date) {
					dayTasks = append(dayTasks, slimTask(t))
				}
			}

			habitRows, err := queryResourceMaps(cmd, db, "habits")
			if err != nil {
				return err
			}
			habits := make([]map[string]any, 0)
			for _, h := range habitRows {
				status, _ := h["status"].(float64)
				if int(status) != 0 { // 0 = active; skip archived
					continue
				}
				habits = append(habits, map[string]any{
					"id":            h["id"],
					"name":          h["name"],
					"goal":          h["goal"],
					"unit":          h["unit"],
					"totalCheckIns": h["totalCheckIns"],
				})
			}

			focusRows, err := queryResourceMaps(cmd, db, "focus")
			if err != nil {
				return err
			}
			focus := make([]map[string]any, 0)
			var focusMinutes float64
			for _, f := range focusRows {
				start, _ := f["startTime"].(string)
				if !strings.HasPrefix(start, date) {
					continue
				}
				focus = append(focus, map[string]any{
					"startTime": f["startTime"],
					"endTime":   f["endTime"],
					"status":    f["status"],
				})
				focusMinutes += focusDurationMinutes(f)
			}

			view := map[string]any{
				"date":          date,
				"tasks":         dayTasks,
				"habits":        habits,
				"focus":         focus,
				"focus_minutes": int(focusMinutes),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agenda for %s\n\nTasks (%d):\n", date, len(dayTasks))
			for _, t := range dayTasks {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %v\n", t["title"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nHabits (%d active):\n", len(habits))
			for _, h := range habits {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %v\n", h["name"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nFocus: %d sessions, %d minutes\n", len(focus), int(focusMinutes))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "today", "Agenda date: 'today', 'yesterday', or YYYY-MM-DD")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// queryResourceMaps drains all rows of one resource type into decoded maps
// (drain-first: no nested queries while rows are open).
func queryResourceMaps(cmd *cobra.Command, db *store.Store, resourceType string) ([]map[string]any, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", resourceType, err)
	}
	raw := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan %s: %w", resourceType, err)
		}
		raw = append(raw, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate %s: %w", resourceType, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close %s: %w", resourceType, err)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, d := range raw {
		var m map[string]any
		if err := json.Unmarshal([]byte(d), &m); err == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// slimTask keeps the agenda-relevant task fields.
func slimTask(t map[string]any) map[string]any {
	return map[string]any{
		"id":        t["id"],
		"projectId": t["projectId"],
		"title":     t["title"],
		"kind":      t["kind"],
		"priority":  t["priority"],
		"dueDate":   t["dueDate"],
		"startDate": t["startDate"],
		"status":    t["status"],
	}
}

// focusDurationMinutes computes a focus record's duration in minutes.
func focusDurationMinutes(f map[string]any) float64 {
	start, _ := f["startTime"].(string)
	end, _ := f["endTime"].(string)
	st, err1 := parseFocusTime(start)
	et, err2 := parseFocusTime(end)
	if err1 != nil || err2 != nil || et.Before(st) {
		return 0
	}
	d := et.Sub(st).Minutes()
	if pause, ok := f["pauseDuration"].(float64); ok {
		d -= pause / 60
	}
	if d < 0 {
		return 0
	}
	return d
}

func parseFocusTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable time %q", s)
}
