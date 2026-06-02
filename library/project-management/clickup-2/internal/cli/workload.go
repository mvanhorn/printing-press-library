// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWorkloadCmd(flags *rootFlags) *cobra.Command {
	var flagSpace string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Open-task count, summed time estimates, and active-timer status per team member",
		Long: `Joins synced tasks, members, and running time entries in the local store
to show, per assignee: how many open tasks they hold, the summed time
estimate of those tasks, and whether they currently have a running timer.

This is the "who's overloaded?" view ClickUp gates behind paid Dashboards.
Restrict to one space with --space <space_id>. Reads the local store only;
run 'clickup-2-pp-cli sync' first.`,
		Example: `  # Workload across the whole synced workspace
  clickup-2-pp-cli workload

  # Just one space, as JSON
  clickup-2-pp-cli workload --space 90010 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("clickup-2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clickup-2-pp-cli sync' first.", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "task", flags.maxAge)

			tasks, err := loadPMTasks(db)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+noTasksHint)
			}

			activeTimers := loadActiveTimerUserIDs(db)

			type bucket struct {
				UserID      int64   `json:"user_id"`
				Username    string  `json:"username,omitempty"`
				OpenTasks   int     `json:"open_tasks"`
				EstimateMS  int64   `json:"estimate_ms"`
				EstimateHrs float64 `json:"estimate_hours"`
				ActiveTimer bool    `json:"active_timer"`
			}
			buckets := map[int64]*bucket{}
			unassigned := &bucket{UserID: 0, Username: "(unassigned)"}
			for _, t := range tasks {
				if !t.open() {
					continue
				}
				if flagSpace != "" && t.SpaceID != flagSpace {
					continue
				}
				if len(t.Assignees) == 0 {
					unassigned.OpenTasks++
					unassigned.EstimateMS += t.TimeEstimate
					continue
				}
				for _, a := range t.Assignees {
					b := buckets[a.ID]
					if b == nil {
						b = &bucket{UserID: a.ID, Username: a.Username}
						buckets[a.ID] = b
					}
					if b.Username == "" {
						b.Username = a.Username
					}
					b.OpenTasks++
					b.EstimateMS += t.TimeEstimate
					if activeTimers[a.ID] {
						b.ActiveTimer = true
					}
				}
			}

			out := make([]bucket, 0, len(buckets)+1)
			for _, b := range buckets {
				b.EstimateHrs = float64(b.EstimateMS) / 3600000.0
				out = append(out, *b)
			}
			if unassigned.OpenTasks > 0 {
				unassigned.EstimateHrs = float64(unassigned.EstimateMS) / 3600000.0
				out = append(out, *unassigned)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].OpenTasks != out[j].OpenTasks {
					return out[i].OpenTasks > out[j].OpenTasks
				}
				return out[i].EstimateMS > out[j].EstimateMS
			})

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No open tasks found for any assignee.")
				return nil
			}
			fmt.Fprintf(w, "%-22s %-6s %-9s %s\n", "MEMBER", "OPEN", "EST(hrs)", "TIMER")
			for _, b := range out {
				name := b.Username
				if name == "" {
					name = strconv.FormatInt(b.UserID, 10)
				}
				timer := ""
				if b.ActiveTimer {
					timer = "running"
				}
				fmt.Fprintf(w, "%-22s %-6d %-9.1f %s\n", name, b.OpenTasks, b.EstimateHrs, timer)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSpace, "space", "", "Restrict to a single space id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// loadActiveTimerUserIDs returns the set of user ids with a currently running
// timer, read from synced time entries (a running entry has no/zero `end`).
func loadActiveTimerUserIDs(db *store.Store) map[int64]bool {
	out := map[int64]bool{}
	rows, err := db.List("time", pmListLimit)
	if err != nil {
		return out
	}
	for _, r := range rows {
		var obj map[string]any
		if json.Unmarshal(r, &obj) != nil {
			continue
		}
		end := parseMSField(obj["end"])
		running := false
		if b, ok := obj["running"].(bool); ok && b {
			running = true
		}
		if end != 0 {
			running = false
		}
		if !running {
			continue
		}
		// user id can be a number, string, or nested {"user":{"id":...}}
		switch u := obj["user"].(type) {
		case map[string]any:
			switch id := u["id"].(type) {
			case float64:
				out[int64(id)] = true
			case string:
				if n, err := strconv.ParseInt(id, 10, 64); err == nil {
					out[n] = true
				}
			}
		case float64:
			out[int64(u)] = true
		case string:
			if n, err := strconv.ParseInt(u, 10, 64); err == nil {
				out[n] = true
			}
		}
	}
	return out
}
