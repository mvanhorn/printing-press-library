// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Week-in-review data pack: completed tasks, daily notes, focus totals from
// the local mirror in one structured response.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/store"
)

// pp:data-source local
func newNovelReviewCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Gather a window of completed tasks, daily notes, and focus totals as one data pack",
		Long: "Use this command to gather a week's raw review data. Do NOT use it for a single day's snapshot; use 'agenda' instead.\n" +
			"Reads the local SQLite mirror (run 'sync' first): completed tasks in the window, TEXT/NOTE-kind notes, and focus minutes per day. Emits raw data for synthesis — no summarization.",
		Example:     "  ticktick-pp-cli review --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for the review window")
				return nil
			}
			if flagSince == "" {
				flagSince = "7d"
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			cutoff := time.Now().Add(-window)
			cutoffStr := cutoff.Format("2006-01-02")
			if dbPath == "" {
				dbPath = defaultDBPath("ticktick-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ticktick-pp-cli sync --resources completed,tasks,focus --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					// Keep the documented object shape on an empty/unsynced
					// cache so JSON consumers keyed on object fields don't
					// see the type flip to a bare array.
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"window_start":    cutoffStr,
						"window":          flagSince,
						"completed":       []any{},
						"completed_count": 0,
						"notes":           []any{},
						"focus_totals":    map[string]any{"minutes": 0, "by_day": []any{}},
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
			if !hintIfUnsynced(cmd, db, "completed") {
				hintIfStale(cmd, db, "completed", 24*time.Hour)
			}

			completedRows, err := queryResourceMaps(cmd, db, "completed")
			if err != nil {
				return err
			}
			completed := make([]map[string]any, 0)
			for _, t := range completedRows {
				ct, _ := t["completedTime"].(string)
				if ct >= cutoffStr {
					completed = append(completed, slimTask(t))
				}
			}

			taskRows, err := queryResourceMaps(cmd, db, "tasks")
			if err != nil {
				return err
			}
			notes := make([]map[string]any, 0)
			for _, t := range taskRows {
				kind, _ := t["kind"].(string)
				if kind != "TEXT" && kind != "NOTE" {
					continue
				}
				start, _ := t["startDate"].(string)
				if start >= cutoffStr {
					notes = append(notes, map[string]any{
						"id":        t["id"],
						"title":     t["title"],
						"startDate": t["startDate"],
						"content":   t["content"],
					})
				}
			}

			focusRows, err := queryResourceMaps(cmd, db, "focus")
			if err != nil {
				return err
			}
			byDay := map[string]float64{}
			var totalMinutes float64
			for _, f := range focusRows {
				start, _ := f["startTime"].(string)
				if len(start) < 10 || start[:10] < cutoffStr {
					continue
				}
				m := focusDurationMinutes(f)
				byDay[start[:10]] += m
				totalMinutes += m
			}
			days := make([]string, 0, len(byDay))
			for d := range byDay {
				days = append(days, d)
			}
			sort.Strings(days)
			focusByDay := make([]map[string]any, 0, len(days))
			for _, d := range days {
				focusByDay = append(focusByDay, map[string]any{"date": d, "minutes": int(byDay[d])})
			}

			view := map[string]any{
				"window_start":    cutoffStr,
				"window":          flagSince,
				"completed":       completed,
				"completed_count": len(completed),
				"notes":           notes,
				"focus_totals":    map[string]any{"minutes": int(totalMinutes), "by_day": focusByDay},
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Review since %s (%s)\n\nCompleted (%d):\n", cutoffStr, flagSince, len(completed))
			for _, t := range completed {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %v\n", t["title"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nNotes (%d):\n", len(notes))
			for _, n := range notes {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %v (%v)\n", n["title"], truncateDate(n["startDate"]))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nFocus: %d minutes total across %d days\n", int(totalMinutes), len(days))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Review window (e.g. 7d, 2w, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func truncateDate(v any) string {
	s, _ := v.(string)
	if len(s) > 10 {
		return s[:10]
	}
	return strings.TrimSpace(s)
}
