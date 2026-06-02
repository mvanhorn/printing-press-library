// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelTimeInStatusCmd(flags *rootFlags) *cobra.Command {
	var flagRank bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "time-in-status <list_id|task_id>",
		Short: "How long tasks dwell in each status, per task or rolled up per list",
		Long: `Reads the synced time-in-status history from the local store and reports
how long work dwells in each status.

Pass a task id to see that task's per-status breakdown. Pass a list id to
roll up every synced task in that list by status. --rank sorts statuses by
total dwell time, worst-first, to surface bottlenecks (e.g. tasks piling up
in "Code Review").

Time-in-status history is captured during sync. If results are empty, sync
the time_in_status resource first:
  clickup-2-pp-cli sync --resources team,team_task,time_in_status`,
		Example: `  # A single task's per-status dwell
  clickup-2-pp-cli time-in-status 86abc123

  # A list, ranked by worst bottleneck, as JSON
  clickup-2-pp-cli time-in-status 901100 --rank --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			target := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("clickup-2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clickup-2-pp-cli sync' first.", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "time_in_status", flags.maxAge)

			// Map task_id -> minutes-per-status from the synced history.
			perStatus := map[string]int64{}
			var matchedTasks int

			// Decide whether target is a task we have history for, or a list.
			taskIDsInList := tasksInList(db, target)
			isList := len(taskIDsInList) > 0

			rows, err := db.List("time_in_status", pmListLimit)
			if err != nil {
				return fmt.Errorf("reading time_in_status from local store: %w", err)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: no time-in-status history in the local store. Sync it first:\n  clickup-2-pp-cli sync --resources team,team_task,time_in_status")
				return emptyTimeInStatus(cmd, flags, target, isList)
			}
			wantTask := map[string]bool{}
			if isList {
				for _, id := range taskIDsInList {
					wantTask[id] = true
				}
			} else {
				wantTask[target] = true
			}

			for _, r := range rows {
				var obj map[string]any
				if json.Unmarshal(r, &obj) != nil {
					continue
				}
				tid, _ := obj["task_id"].(string)
				if tid == "" {
					// Some stores key history under id; fall back to that.
					tid, _ = obj["id"].(string)
				}
				if !wantTask[tid] {
					continue
				}
				matchedTasks++
				accumulateStatusMinutes(obj, perStatus)
			}

			if matchedTasks == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: no synced time-in-status history for %q (neither a known task nor a list with synced tasks)\n", target)
				return emptyTimeInStatus(cmd, flags, target, isList)
			}

			type statusRow struct {
				Status  string  `json:"status"`
				Minutes int64   `json:"minutes"`
				Hours   float64 `json:"hours"`
			}
			out := make([]statusRow, 0, len(perStatus))
			for s, m := range perStatus {
				out = append(out, statusRow{Status: s, Minutes: m, Hours: float64(m) / 60.0})
			}
			if flagRank {
				sort.SliceStable(out, func(i, j int) bool { return out[i].Minutes > out[j].Minutes })
			} else {
				sort.SliceStable(out, func(i, j int) bool { return out[i].Status < out[j].Status })
			}

			payload := map[string]any{
				"target":     target,
				"is_list":    isList,
				"task_count": matchedTasks,
				"statuses":   out,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			w := cmd.OutOrStdout()
			scope := "task"
			if isList {
				scope = fmt.Sprintf("list (%d tasks)", matchedTasks)
			}
			fmt.Fprintf(w, "Time in status for %s %s:\n\n", scope, target)
			fmt.Fprintf(w, "%-24s %-10s %s\n", "STATUS", "HOURS", "MINUTES")
			for _, s := range out {
				fmt.Fprintf(w, "%-24s %-10.1f %d\n", s.Status, s.Hours, s.Minutes)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagRank, "rank", false, "Sort statuses by total dwell time, worst-first")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// emptyTimeInStatus emits an empty (but well-shaped) result and exit 0 so the
// command behaves consistently with the other read-only store commands when
// no history is available yet.
func emptyTimeInStatus(cmd *cobra.Command, flags *rootFlags, target string, isList bool) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		payload := map[string]any{"target": target, "is_list": isList, "task_count": 0, "statuses": []any{}}
		return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "No time-in-status history for %s.\n", target)
	return nil
}

// tasksInList returns the synced task ids whose list.id equals listID.
func tasksInList(db *store.Store, listID string) []string {
	tasks, err := loadPMTasks(db)
	if err != nil {
		return nil
	}
	var out []string
	for _, t := range tasks {
		if t.ListID == listID {
			out = append(out, t.ID)
		}
	}
	return out
}

// accumulateStatusMinutes folds one task's time_in_status payload into the
// per-status minute totals. ClickUp returns current_status + status_history,
// each with total_time.by_minute.
func accumulateStatusMinutes(obj map[string]any, perStatus map[string]int64) {
	add := func(node any) {
		m, ok := node.(map[string]any)
		if !ok {
			return
		}
		status, _ := m["status"].(string)
		if status == "" {
			return
		}
		var minutes int64
		if tt, ok := m["total_time"].(map[string]any); ok {
			minutes = parseMSField(tt["by_minute"])
		}
		perStatus[status] += minutes
	}
	add(obj["current_status"])
	if hist, ok := obj["status_history"].([]any); ok {
		for _, h := range hist {
			add(h)
		}
	}
	// Some stores wrap the payload under a "data" key.
	if data, ok := obj["data"].(map[string]any); ok {
		add(data["current_status"])
		if hist, ok := data["status_history"].([]any); ok {
			for _, h := range hist {
				add(h)
			}
		}
	}
}
