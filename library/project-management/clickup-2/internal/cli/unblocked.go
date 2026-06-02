// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelUnblockedCmd(flags *rootFlags) *cobra.Command {
	var flagList string
	var flagBlocked bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "unblocked",
		Short: "Open tasks whose blockers are all closed (the ready-to-work set); --blocked inverts it",
		Long: `Joins synced tasks with their dependencies in the local store to compute
which open tasks are actually ready to work on: every task they wait on is
closed. With --blocked, it instead lists open tasks that still have at least
one open blocker, naming the blockers.

A task's "blockers" are the tasks it depends on (ClickUp dependency entries
where this task waits on another). Reads the local store only; run
'clickup-2-pp-cli sync' first.`,
		Example: `  # Everything ready to pick up now
  clickup-2-pp-cli unblocked

  # What's blocked in one list, and by what, as JSON
  clickup-2-pp-cli unblocked --list 901100 --blocked --json`,
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

			// status index: task id -> open?
			isOpen := make(map[string]bool, len(tasks))
			for _, t := range tasks {
				isOpen[t.ID] = t.open()
			}

			type row struct {
				ID           string   `json:"id"`
				Name         string   `json:"name"`
				Status       string   `json:"status"`
				OpenBlockers []string `json:"open_blockers,omitempty"`
				URL          string   `json:"url,omitempty"`
			}
			out := []row{}
			for _, t := range tasks {
				if !t.open() {
					continue
				}
				if flagList != "" && t.ListID != flagList {
					continue
				}
				// Collect the tasks this one waits on (its blockers).
				var openBlockers []string
				for _, d := range t.Deps {
					blocker := d.DependsOn
					// Only count entries where this task is the waiting side.
					if d.TaskID != "" && d.TaskID != t.ID {
						continue
					}
					if blocker == "" || blocker == t.ID {
						continue
					}
					// A blocker counts as blocking only if it's still open
					// (or unknown-but-present, which we treat as open to be safe).
					if open, known := isOpen[blocker]; !known || open {
						openBlockers = append(openBlockers, blocker)
					}
				}
				blocked := len(openBlockers) > 0
				if flagBlocked && !blocked {
					continue
				}
				if !flagBlocked && blocked {
					continue
				}
				out = append(out, row{ID: t.ID, Name: t.Name, Status: t.Status, OpenBlockers: openBlockers, URL: t.URL})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				if flagBlocked {
					fmt.Fprintln(w, "No blocked open tasks.")
				} else {
					fmt.Fprintln(w, "No unblocked open tasks (everything is either blocked or closed).")
				}
				return nil
			}
			if flagBlocked {
				fmt.Fprintf(w, "%-12s %-10s %s\n", "ID", "STATUS", "NAME / OPEN BLOCKERS")
				for _, r := range out {
					fmt.Fprintf(w, "%-12s %-10s %s  [blocked by: %v]\n", r.ID, r.Status, r.Name, r.OpenBlockers)
				}
			} else {
				fmt.Fprintf(w, "%-12s %-10s %s\n", "ID", "STATUS", "NAME")
				for _, r := range out {
					fmt.Fprintf(w, "%-12s %-10s %s\n", r.ID, r.Status, r.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagList, "list", "", "Restrict to a single list id")
	cmd.Flags().BoolVar(&flagBlocked, "blocked", false, "Show blocked tasks (with their open blockers) instead of unblocked")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
